package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dsdns/internal/sync"
)

func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil || !claims.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := h.DB.Query(`
			SELECT id, name, endpoint, token, last_heartbeat, status, created_at
			FROM nodes WHERE id != 0 ORDER BY id
		`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		nodes := []map[string]interface{}{}
		for rows.Next() {
			var id int64
			var name, endpoint, token, status string
			var lastHeartbeatPtr, createdAtPtr interface{}
			if err := rows.Scan(&id, &name, &endpoint, &token, &lastHeartbeatPtr, &status, &createdAtPtr); err != nil {
				continue
			}
			// 脱敏 token（仅显示前8位）
			maskedToken := ""
			if len(token) > 8 {
				maskedToken = token[:8] + "..."
			}
			node := map[string]interface{}{
				"id":       id,
				"name":     name,
				"endpoint": endpoint,
				"token":    maskedToken,
				"status":   status,
			}
			if t, ok := lastHeartbeatPtr.(time.Time); ok {
				node["last_heartbeat"] = t
			}
			if t, ok := createdAtPtr.(time.Time); ok {
				node["created_at"] = t
			}
			nodes = append(nodes, node)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": nodes})

	case http.MethodPost:
		var input struct {
			Name     string `json:"name"`
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// 生成随机 token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		res, err := h.DB.Exec(`
			INSERT INTO nodes (name, endpoint, token, status, last_heartbeat)
			VALUES (?, ?, ?, 'unknown', NULL)
		`, input.Name, input.Endpoint, token)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				http.Error(w, "endpoint already exists", http.StatusConflict)
			} else {
				http.Error(w, "db error", http.StatusInternalServerError)
			}
			return
		}
		id, _ := res.LastInsertId()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 201,
			"data": map[string]interface{}{
				"id":       id,
				"name":     input.Name,
				"endpoint": input.Endpoint,
				"token":    token, // 仅创建时返回完整token
				"status":   "unknown",
			},
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil || !claims.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// 解析 id
	path := strings.TrimPrefix(r.URL.Path, "/api/nodes/")
	// 可能包含 /sync 后缀
	if strings.HasSuffix(path, "/sync") {
		idStr := strings.TrimSuffix(path, "/sync")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		h.handleNodeSync(id, w, r)
		return
	}
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var input struct {
			Name     string `json:"name"`
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, err = h.DB.Exec(`UPDATE nodes SET name=?, endpoint=? WHERE id=?`, input.Name, input.Endpoint, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "updated"})

	case http.MethodDelete:
		// 删除前清理相关测活结果
		_, err = h.DB.Exec(`DELETE FROM check_results WHERE node_id = ?`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		_, err = h.DB.Exec(`DELETE FROM nodes WHERE id = ?`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleNodeSync(id int64, w http.ResponseWriter, r *http.Request) {
	// 获取节点信息
	var endpoint, token string
	err := h.DB.QueryRow(`SELECT endpoint, token FROM nodes WHERE id = ?`, id).Scan(&endpoint, &token)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	// 异步同步
	go sync.SyncToNode(h.DB, id, endpoint, token, h.Config.Controller.SyncTimeoutSec)
	json.NewEncoder(w).Encode(map[string]string{"message": "sync triggered"})
}

func (h *Handler) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil || !claims.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rows, err := h.DB.Query(`SELECT id, endpoint, token FROM nodes WHERE status = 'online' AND endpoint != ''`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var endpoint, token string
		if err := rows.Scan(&id, &endpoint, &token); err != nil {
			continue
		}
		go sync.SyncToNode(h.DB, id, endpoint, token, h.Config.Controller.SyncTimeoutSec)
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "sync broadcast"})
}

// nodeHeartbeat 供子节点调用
func (h *Handler) nodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	// 验证 token
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	var nodeID int64
	var dbToken string
	err := h.DB.QueryRow(`SELECT id, token FROM nodes WHERE token = ?`, token).Scan(&nodeID, &dbToken)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	var body struct {
		NodeID int64 `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.NodeID != nodeID {
		http.Error(w, "node id mismatch", http.StatusBadRequest)
		return
	}
	// 更新心跳
	_, err = h.DB.Exec(`UPDATE nodes SET last_heartbeat = CURRENT_TIMESTAMP, status = 'online' WHERE id = ?`, nodeID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// checkReport 接收子节点测活结果上报
func (h *Handler) checkReport(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	var nodeID int64
	err := h.DB.QueryRow(`SELECT id FROM nodes WHERE token = ?`, token).Scan(&nodeID)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	var report struct {
		NodeID  int64  `json:"node_id"`
		TaskID  string `json:"task_id"`
		Results []struct {
			RecordID int64  `json:"record_id"`
			Success  bool   `json:"success"`
			Message  string `json:"message"`
			Invalid  bool   `json:"invalid"`
		} `json:"results"`
	}
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if report.NodeID != nodeID {
		http.Error(w, "node id mismatch", http.StatusBadRequest)
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	stmt, _ := tx.Prepare(`
		INSERT INTO check_results (record_id, node_id, success, message, invalid, task_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	for _, res := range report.Results {
		successInt := 0
		if res.Success {
			successInt = 1
		}
		invalidInt := 0
		if res.Invalid {
			invalidInt = 1
		}
		_, err = stmt.Exec(res.RecordID, nodeID, successInt, res.Message, invalidInt, report.TaskID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	}
	tx.Commit()
	w.WriteHeader(http.StatusOK)
}
