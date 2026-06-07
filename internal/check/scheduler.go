package check

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"dsdns/internal/logger"
	"dsdns/internal/notify"
)

type Scheduler struct {
	db               *sql.DB
	timeoutSec       int
	successThreshold float64
	tgEnabled        bool
	tgBotToken       string
	tgChatID         string
}

func NewScheduler(db *sql.DB, timeoutSec int, threshold float64, tgEnabled bool, tgToken, tgChatID string) *Scheduler {
	return &Scheduler{
		db:               db,
		timeoutSec:       timeoutSec,
		successThreshold: threshold,
		tgEnabled:        tgEnabled,
		tgBotToken:       tgToken,
		tgChatID:         tgChatID,
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) {
	taskID := uuid.New().String()
	logger.Info("starting health check", "task_id", taskID)

	rows, err := s.db.QueryContext(ctx, `SELECT id, endpoint, token FROM nodes WHERE status = 'online'`)
	if err != nil {
		logger.Error("query nodes failed", "error", err)
		return
	}
	defer rows.Close()

	type nodeInfo struct {
		ID       int64
		Endpoint string
		Token    string
	}
	var nodes []nodeInfo
	nodes = append(nodes, nodeInfo{ID: 0, Endpoint: "", Token: ""})
	for rows.Next() {
		var n nodeInfo
		if err := rows.Scan(&n.ID, &n.Endpoint, &n.Token); err != nil {
			continue
		}
		if n.Endpoint != "" {
			nodes = append(nodes, n)
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for _, node := range nodes {
		wg.Add(1)
		go func(n nodeInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if n.ID == 0 {
				s.runLocalCheck(ctx, taskID)
			} else {
				s.triggerNodeCheck(ctx, n.ID, n.Endpoint, n.Token, taskID)
			}
		}(node)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.Info("all nodes finished check", "task_id", taskID)
	case <-time.After(time.Duration(s.timeoutSec) * time.Second):
		logger.Warn("check timeout, some nodes not finished", "task_id", taskID)
	}

	s.analyzeAndNotify(ctx, taskID)
	s.db.ExecContext(ctx, `DELETE FROM check_results WHERE checked_at < datetime('now', '-7 days')`)
}

func (s *Scheduler) triggerNodeCheck(ctx context.Context, nodeID int64, endpoint, token, taskID string) {
	url := fmt.Sprintf("%s/api/check/run", endpoint)
	reqBody := fmt.Sprintf(`{"task_id":"%s"}`, taskID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(reqBody))
	if err != nil {
		logger.Warn("create check request failed", "node_id", nodeID, "error", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Duration(s.timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("trigger node check failed", "node_id", nodeID, "error", err)
		return
	}
	defer resp.Body.Close()
}

func (s *Scheduler) runLocalCheck(ctx context.Context, taskID string) {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("runLocalCheck panicked", "task_id", taskID, "panic", r)
        }
    }()
    logger.Info("Local check started", "task_id", taskID)
    startAll := time.Now()
	rows, err := s.db.QueryContext(ctx, `SELECT id, type, value FROM records`)
	if err != nil {
		logger.Error("query records for local check failed", "error", err)
		return
	}
	defer rows.Close()
	type record struct {
		ID    int64
		Type  string
		Value string
	}
	var records []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.ID, &r.Type, &r.Value); err != nil {
			continue
		}
		records = append(records, r)
	}

	var results []map[string]interface{}
    for _, rec := range records {
        start := time.Now()
        success, invalid, msg := CheckRecord(rec.Type, rec.Value)
        elapsed := time.Since(start)
        logger.Debug("Check record", 
            "record_id", rec.ID, 
            "type", rec.Type, 
            "value", rec.Value,
            "success", success,
            "invalid", invalid,
            "message", msg,
            "elapsed_ms", elapsed.Milliseconds())
		results = append(results, map[string]interface{}{
			"record_id": rec.ID,
			"success":   success,
			"message":   msg,
			"invalid":   invalid,
		})
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("begin tx for local check failed", "error", err)
		return
	}
	defer tx.Rollback()
	stmt, _ := tx.PrepareContext(ctx, `
		INSERT INTO check_results (record_id, node_id, success, message, invalid, task_id)
		VALUES (?, 0, ?, ?, ?, ?)
	`)
	for _, res := range results {
		successInt := 0
		if res["success"].(bool) {
			successInt = 1
		}
		invalidInt := 0
		if res["invalid"].(bool) {
			invalidInt = 1
		}
		stmt.Exec(res["record_id"], successInt, res["message"], invalidInt, taskID)
	}
	tx.Commit()
    logger.Info("Local check finished", "task_id", taskID, "total_ms", time.Since(startAll).Milliseconds())
}

// analyzeAndNotify 分析本轮测活结果，对成功率低于阈值的记录发送告警
func (s *Scheduler) analyzeAndNotify(ctx context.Context, taskID string) {
    rows, err := s.db.QueryContext(ctx, `
        SELECT record_id, AVG(CASE WHEN invalid=0 THEN success ELSE NULL END) as rate
        FROM check_results
        WHERE task_id = ? AND invalid = 0
        GROUP BY record_id
    `, taskID)
    if err != nil {
        logger.Error("query check results failed", "error", err)
        return
    }
    defer rows.Close()

    for rows.Next() {
        var rid int64
        var rate sql.NullFloat64
        if err := rows.Scan(&rid, &rate); err != nil {
            continue
        }
        if !rate.Valid || rate.Float64 >= s.successThreshold {
            continue
        }

        // 获取记录详情及所属用户ID
        var domain, recType, recValue string
        var userID sql.NullInt64
        err := s.db.QueryRowContext(ctx, `
            SELECT d.domain, r.type, r.value, u.id
            FROM records r
            JOIN domains d ON d.id = r.domain_id
            JOIN users u ON u.id = d.user_id
            WHERE r.id = ?
        `, rid).Scan(&domain, &recType, &recValue, &userID)
        if err != nil {
            logger.Error("query record owner failed", "record_id", rid, "error", err)
            continue
        }

        content := fmt.Sprintf("⚠️ 健康度告警\n域名: %s\n记录: %s %s\n健康度: %.0f%% (低于阈值 %.0f%%)",
            domain, recType, recValue, rate.Float64*100, s.successThreshold*100)

        // 检查最近1小时内是否已为相同域名+用户发送过告警（避免重复）
        var count int
        s.db.QueryRowContext(ctx, `
            SELECT COUNT(*) FROM notifications
            WHERE type = 'record_invalid' AND content LIKE ? AND user_id = ? AND created_at > datetime('now', '-1 hour')
        `, "%"+domain+"%", userID).Scan(&count)

        if count == 0 {
            _, err = s.db.ExecContext(ctx, `
                INSERT INTO notifications (type, content, user_id)
                VALUES (?, ?, ?)
            `, "record_invalid", content, userID)
            if err != nil {
                logger.Error("insert notification failed", "error", err)
            }

            // 发送 Telegram 告警（全局配置，与用户无关）
            if s.tgEnabled {
                notify.SendTelegram(s.tgBotToken, s.tgChatID, content)
            }
        }
    }
}
