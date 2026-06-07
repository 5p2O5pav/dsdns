package worker

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"dsdns/internal/check"
	"dsdns/internal/config"
	"dsdns/internal/logger"
	"dsdns/internal/notify"
	"dsdns/internal/sync"
)

func StartBackgroundTasks(db *sql.DB, cfg *config.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	go cleanOldNotifications(ctx, db)
	go heartbeatMonitor(ctx, db, cfg)
	if cfg.Mode == "master" {
		go checkScheduler(ctx, db, cfg)
	}
	_ = cancel
}

func heartbeatMonitor(ctx context.Context, db *sql.DB, cfg *config.Config) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			timeoutSec := cfg.Controller.HeartbeatTimeoutSec
			rows, err := db.QueryContext(ctx, `
				SELECT id, name, status, last_sync_at FROM nodes
				WHERE id != 0 AND (last_heartbeat IS NULL OR (strftime('%s','now') - strftime('%s', last_heartbeat)) > ?)
			`, timeoutSec)
			if err != nil {
				logger.Error("heartbeat monitor query failed", "error", err)
				continue
			}
			for rows.Next() {
				var id int64
				var name, status string
				var lastSyncAt sql.NullTime
				rows.Scan(&id, &name, &status, &lastSyncAt)
				if status != "offline" {
					db.ExecContext(ctx, `UPDATE nodes SET status = 'offline' WHERE id = ?`, id)
					content := "节点 " + name + " 离线超过 " + strconv.Itoa(timeoutSec) + " 秒"
					// 离线告警
					db.ExecContext(ctx, `INSERT INTO notifications (type, content, user_id) VALUES (?, ?, NULL)`, "node_offline", content)
					if cfg.Telegram.Enabled {
						notify.SendTelegram(cfg.Telegram.BotToken, cfg.Telegram.ChatID, content)
					}
				}
			}
			rows.Close()

			rows2, err := db.QueryContext(ctx, `
				SELECT id, name, last_sync_at FROM nodes
				WHERE id != 0 AND status = 'offline' AND (strftime('%s','now') - strftime('%s', last_heartbeat)) <= ?
			`, timeoutSec)
			if err != nil {
				continue
			}
			for rows2.Next() {
				var id int64
				var name string
				var lastSyncAt sql.NullTime
				rows2.Scan(&id, &name, &lastSyncAt)
				db.ExecContext(ctx, `UPDATE nodes SET status = 'online' WHERE id = ?`, id)
				content := "节点 " + name + " 已恢复"
				db.ExecContext(ctx, `INSERT INTO notifications (type, content, user_id) VALUES (?, ?, NULL)`, "node_online", content)
				if cfg.Telegram.Enabled {
					notify.SendTelegram(cfg.Telegram.BotToken, cfg.Telegram.ChatID, content)
				}
				var since time.Time
				if lastSyncAt.Valid {
					since = lastSyncAt.Time
				} else {
					since = time.Now().Add(-24 * time.Hour)
				}
				go sync.PushMissedChanges(db, id, since, cfg.Controller.SyncTimeoutSec)
			}
			rows2.Close()
		}
	}
}

func checkScheduler(ctx context.Context, db *sql.DB, cfg *config.Config) {
	time.Sleep(10 * time.Second)
	ticker := time.NewTicker(time.Duration(cfg.Controller.CheckIntervalSec) * time.Second)
	defer ticker.Stop()
	sched := check.NewScheduler(
		db,
		cfg.Controller.SyncTimeoutSec,
		cfg.Controller.CheckSuccessThreshold,
		cfg.Telegram.Enabled,
		cfg.Telegram.BotToken,
		cfg.Telegram.ChatID,
	)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sched.RunOnce(ctx)
		}
	}
}

// 新增函数
func cleanOldNotifications(ctx context.Context, db *sql.DB) {
    ticker := time.NewTicker(6 * time.Hour) // 每6小时清理一次
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            _, err := db.ExecContext(ctx, `DELETE FROM notifications WHERE created_at < datetime('now', '-2 days')`)
            if err != nil {
                logger.Error("clean old notifications failed", "error", err)
            } else {
                logger.Debug("cleaned old notifications")
            }
        }
    }
}
