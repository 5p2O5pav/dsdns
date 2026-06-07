package sync

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dsdns/internal/logger"
)

type IncrementalOp struct {
	Op   string      `json:"op"`
	Data interface{} `json:"data"`
}

func RecordChange(db *sql.DB, op IncrementalOp) error {
	dataBytes, err := json.Marshal(op)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO sync_log (op, data) VALUES (?, ?)`, op.Op, string(dataBytes))
	return err
}

func NotifyNodesIncremental(db *sql.DB, timeoutSec int, op IncrementalOp) {
	if err := RecordChange(db, op); err != nil {
		logger.Error("record change failed", "error", err)
	}

	rows, err := db.Query(`SELECT id, endpoint, token FROM nodes WHERE status = 'online' AND endpoint != ''`)
	if err != nil {
		logger.Error("query nodes for incremental failed", "error", err)
		return
	}
	defer rows.Close()

	sem := make(chan struct{}, 10)
	for rows.Next() {
		var id int64
		var endpoint, token string
		if err := rows.Scan(&id, &endpoint, &token); err != nil {
			continue
		}
		go func(ep, tk string, nid int64) {
			sem <- struct{}{}
			defer func() { <-sem }()
			sendIncrementalToNode(db, ep, tk, op, timeoutSec, nid)
		}(endpoint, token, id)
	}
}

func sendIncrementalToNode(db *sql.DB, endpoint, token string, op IncrementalOp, timeoutSec int, nodeID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/sync/incremental", endpoint)
	jsonData, err := json.Marshal(op)
	if err != nil {
		logger.Error("marshal incremental op failed", "error", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		logger.Error("create incremental request failed", "error", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("incremental sync to node failed", "node_id", nodeID, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Warn("incremental sync returned non-200", "node_id", nodeID, "status", resp.Status)
	} else {
		updateNodeLastSync(db, nodeID)
	}
}

func updateNodeLastSync(db *sql.DB, nodeID int64) {
	_, err := db.Exec(`UPDATE nodes SET last_sync_at = CURRENT_TIMESTAMP WHERE id = ?`, nodeID)
	if err != nil {
		logger.Error("update node last_sync failed", "node_id", nodeID, "error", err)
	}
}

func PushMissedChanges(db *sql.DB, nodeID int64, since time.Time, timeoutSec int) {
	var endpoint, token string
	err := db.QueryRow(`SELECT endpoint, token FROM nodes WHERE id = ?`, nodeID).Scan(&endpoint, &token)
	if err != nil {
		logger.Error("get node info failed", "node_id", nodeID, "error", err)
		return
	}

	rows, err := db.Query(`
		SELECT data FROM sync_log WHERE created_at > ? ORDER BY id ASC
	`, since)
	if err != nil {
		logger.Error("query missed changes failed", "error", err)
		return
	}
	defer rows.Close()

	var ops []IncrementalOp
	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			continue
		}
		var op IncrementalOp
		if err := json.Unmarshal([]byte(dataStr), &op); err != nil {
			continue
		}
		ops = append(ops, op)
	}

	if len(ops) == 0 {
		return
	}

	for _, op := range ops {
		sendIncrementalToNode(db, endpoint, token, op, timeoutSec, nodeID)
	}
	updateNodeLastSync(db, nodeID)
}
