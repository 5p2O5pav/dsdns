package node

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"dsdns/internal/config"
	"dsdns/internal/logger"
)

func StartHeartbeat(cfg *config.Config) {
	ticker := time.NewTicker(time.Duration(cfg.Node.HeartbeatIntervalSec) * time.Second)
	for range ticker.C {
		url := cfg.Node.MasterURL + "/api/node/heartbeat"
		body := map[string]int64{"node_id": int64(cfg.Node.ID)}
		jsonBody, _ := json.Marshal(body)
		req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
		if err != nil {
			logger.Warn("create heartbeat request failed", "error", err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+cfg.Node.Token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("heartbeat request failed", "error", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			logger.Debug("heartbeat sent", "node_id", cfg.Node.ID)
		} else {
			logger.Warn("heartbeat returned non-200", "status", resp.Status)
		}
	}
}
