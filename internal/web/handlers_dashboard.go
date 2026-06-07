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
    claims := getClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    limit := 20
    if l := r.URL.Query().Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
            limit = parsed
        }
    }

    var rows *sql.Rows
    var err error
    if claims.IsAdmin {
        // 管理员：显示所有 node 告警 + 自己域名的记录告警
        rows, err = h.DB.Query(`
            SELECT id, type, content, created_at FROM notifications
            WHERE (type = 'node_offline' OR type = 'node_online' OR user_id = ?)
            ORDER BY created_at DESC LIMIT ?
        `, claims.UserID, limit)
    } else {
        // 普通用户只显示自己域名的记录告警
        rows, err = h.DB.Query(`
            SELECT id, type, content, created_at FROM notifications
            WHERE user_id = ? AND type = 'record_invalid'
            ORDER BY created_at DESC LIMIT ?
        `, claims.UserID, limit)
    }
    if err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var list []map[string]interface{}
    for rows.Next() {
        var id int64
        var typ, content, createdAt string
        rows.Scan(&id, &typ, &content, &createdAt)
        list = append(list, map[string]interface{}{
            "id":         id,
            "type":       typ,
            "content":    content,
            "created_at": createdAt,
        })
    }
    json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": list})
}

func (h *Handler) healthTree(w http.ResponseWriter, r *http.Request) {
    claims := getClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    type RecordHealth struct {
        RecordID    int64   `json:"record_id"`
        Type        string  `json:"type"`
        Value       string  `json:"value"`
        SuccessRate float64 `json:"success_rate"`
        LastCheck   string  `json:"last_check"`
    }
    type DomainHealth struct {
        DomainID   int64          `json:"domain_id"`
        DomainName string         `json:"domain_name"`
        Records    []RecordHealth `json:"records"`
    }
    type UserHealth struct {
        UserID   int64          `json:"user_id"`
        Username string         `json:"username"`
        Domains  []DomainHealth `json:"domains"`
    }

    if claims.IsAdmin {
        // 管理员：获取所有用户，再按用户获取域名和记录健康度
        userRows, err := h.DB.Query(`SELECT id, username FROM users ORDER BY id`)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
        defer userRows.Close()
        var result []UserHealth
        for userRows.Next() {
            var uid int64
            var uname string
            userRows.Scan(&uid, &uname)
            uh := UserHealth{UserID: uid, Username: uname}

            domainRows, err := h.DB.Query(`
                SELECT id, domain FROM domains WHERE user_id = ? ORDER BY id
            `, uid)
            if err != nil {
                continue
            }
            for domainRows.Next() {
                var did int64
                var dname string
                domainRows.Scan(&did, &dname)
                dh := DomainHealth{DomainID: did, DomainName: dname}

                recRows, err := h.DB.Query(`
                    SELECT r.id, r.type, r.value,
                           AVG(CASE WHEN cr.invalid=0 THEN cr.success ELSE NULL END) as rate,
                           MAX(cr.checked_at) as last_check
                    FROM records r
                    LEFT JOIN check_results cr ON cr.record_id = r.id
                    WHERE r.domain_id = ?
                    GROUP BY r.id
                `, did)
                if err != nil {
                    continue
                }
                for recRows.Next() {
                    var rid int64
                    var rtype, rvalue string
                    var rate sql.NullFloat64
                    var lastCheck sql.NullString
                    recRows.Scan(&rid, &rtype, &rvalue, &rate, &lastCheck)
                    successRate := 0.0
                    if rate.Valid {
                        successRate = rate.Float64
                    }
                    dh.Records = append(dh.Records, RecordHealth{
                        RecordID:    rid,
                        Type:        rtype,
                        Value:       rvalue,
                        SuccessRate: successRate,
                        LastCheck:   lastCheck.String,
                    })
                }
                recRows.Close()
                uh.Domains = append(uh.Domains, dh)
            }
            domainRows.Close()
            result = append(result, uh)
        }
        json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": result})
    } else {
        // 普通用户：直接返回自己的域名和记录健康度
        domainRows, err := h.DB.Query(`
            SELECT id, domain FROM domains WHERE user_id = ? ORDER BY id
        `, claims.UserID)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
        defer domainRows.Close()
        var result []DomainHealth
        for domainRows.Next() {
            var did int64
            var dname string
            domainRows.Scan(&did, &dname)
            dh := DomainHealth{DomainID: did, DomainName: dname}

            recRows, err := h.DB.Query(`
                SELECT r.id, r.type, r.value,
                       AVG(CASE WHEN cr.invalid=0 THEN cr.success ELSE NULL END) as rate,
                       MAX(cr.checked_at) as last_check
                FROM records r
                LEFT JOIN check_results cr ON cr.record_id = r.id
                WHERE r.domain_id = ?
                GROUP BY r.id
            `, did)
            if err != nil {
                continue
            }
            for recRows.Next() {
                var rid int64
                var rtype, rvalue string
                var rate sql.NullFloat64
                var lastCheck sql.NullString
                recRows.Scan(&rid, &rtype, &rvalue, &rate, &lastCheck)
                successRate := 0.0
                if rate.Valid {
                    successRate = rate.Float64
                }
                dh.Records = append(dh.Records, RecordHealth{
                    RecordID:    rid,
                    Type:        rtype,
                    Value:       rvalue,
                    SuccessRate: successRate,
                    LastCheck:   lastCheck.String,
                })
            }
            recRows.Close()
            result = append(result, dh)
        }
        json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": result})
    }
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
