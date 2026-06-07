package web

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

func (h *Handler) dashboardStats(w http.ResponseWriter, r *http.Request) {
    claims := getClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    var totalDomains, totalRecords, onlineNodes, offlineNodes, todayNotif int

    if claims.IsAdmin {
        // 管理员看全局统计
        h.DB.QueryRow(`SELECT COUNT(*) FROM domains`).Scan(&totalDomains)
        h.DB.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&totalRecords)
        h.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE status='online'`).Scan(&onlineNodes)
        h.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE status='offline'`).Scan(&offlineNodes)
        h.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE date(created_at) = date('now')`).Scan(&todayNotif)
    } else {
        // 普通用户只看自己的
        h.DB.QueryRow(`SELECT COUNT(*) FROM domains WHERE user_id = ?`, claims.UserID).Scan(&totalDomains)
        h.DB.QueryRow(`
            SELECT COUNT(*) FROM records
            JOIN domains ON domains.id = records.domain_id
            WHERE domains.user_id = ?
        `, claims.UserID).Scan(&totalRecords)
        // 普通用户不需要节点信息，设为 0，前端隐藏即可
        onlineNodes = 0
        offlineNodes = 0
        h.DB.QueryRow(`
            SELECT COUNT(*) FROM notifications
            WHERE user_id = ? AND date(created_at) = date('now')
        `, claims.UserID).Scan(&todayNotif)
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "code": 200,
        "data": map[string]interface{}{
            "total_domains":       totalDomains,
            "total_records":       totalRecords,
            "online_nodes":        onlineNodes,
            "offline_nodes":       offlineNodes,
            "today_notifications": todayNotif,
        },
    })
}

func (h *Handler) dashboardNotifications(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	rows, err := h.DB.Query(`SELECT id, type, content, is_resolved, created_at FROM notifications ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id int64
		var typ, content string
		var isResolved int
		var createdAt string
		rows.Scan(&id, &typ, &content, &isResolved, &createdAt)
		list = append(list, map[string]interface{}{
			"id":          id,
			"type":        typ,
			"content":     content,
			"is_resolved": isResolved == 1,
			"created_at":  createdAt,
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": list})
}

func (h *Handler) recordHealth(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil || !claims.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rows, err := h.DB.Query(`
		SELECT r.id, d.domain, r.type, r.value,
			AVG(CASE WHEN cr.invalid=0 THEN cr.success ELSE NULL END) as rate,
			MAX(cr.checked_at) as last_check
		FROM records r
		JOIN domains d ON d.id = r.domain_id
		LEFT JOIN check_results cr ON cr.record_id = r.id
		GROUP BY r.id
		ORDER BY rate ASC
	`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var health []map[string]interface{}
	for rows.Next() {
		var rid int64
		var domain, recType, recValue string
		var rate sql.NullFloat64
		var lastCheck sql.NullString
		rows.Scan(&rid, &domain, &recType, &recValue, &rate, &lastCheck)
		successRate := 0.0
		if rate.Valid {
			successRate = rate.Float64
		}
		health = append(health, map[string]interface{}{
			"record_id":    rid,
			"domain":       domain,
			"type":         recType,
			"value":        recValue,
			"success_rate": successRate,
			"last_check":   lastCheck.String,
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": health})
}

func (h *Handler) handleTelegramSettings(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil || !claims.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		// 从配置中读取，但为了演示，可以从数据库读取（这里简化，直接返回配置中的值）
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{
				"enabled":   h.Config.Telegram.Enabled,
				"bot_token": maskToken(h.Config.Telegram.BotToken),
				"chat_id":   h.Config.Telegram.ChatID,
			},
		})
	case http.MethodPut:
		var input struct {
			Enabled  bool   `json:"enabled"`
			BotToken string `json:"bot_token"`
			ChatID   string `json:"chat_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// 更新配置（实际需写入配置文件或数据库，这里只更新内存中的 config）
		h.Config.Telegram.Enabled = input.Enabled
		h.Config.Telegram.BotToken = input.BotToken
		h.Config.Telegram.ChatID = input.ChatID
		// 可选：持久化到数据库
		json.NewEncoder(w).Encode(map[string]string{"message": "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
