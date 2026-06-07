package node
import (
    "bytes"
    "database/sql"
    "encoding/json"
    "net/http"
    "strings"
    "time"

    "dsdns/internal/check"
    "dsdns/internal/config"
    "dsdns/internal/logger"
    "dsdns/internal/sync"
)

type API struct {
    db     *sql.DB
    cfg    *config.Config
    server *http.Server
}

func NewAPI(db *sql.DB, cfg *config.Config) *API {
    return &API{db: db, cfg: cfg}
}

func (api *API) Start() error {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/sync", api.fullSyncHandler)          // 全量同步
    mux.HandleFunc("/api/sync/incremental", api.incrementalHandler) // 增量同步
    mux.HandleFunc("/api/check/run", api.checkRunHandler)
    api.server = &http.Server{Addr: api.cfg.Node.APIListen, Handler: mux}
    logger.Info("node internal API listening", "addr", api.cfg.Node.APIListen)
    return api.server.ListenAndServe()
}

// fullSyncHandler 接收全量配置，原子替换所有数据
func (api *API) fullSyncHandler(w http.ResponseWriter, r *http.Request) {
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, "Bearer ") {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    token := strings.TrimPrefix(auth, "Bearer ")
    if token != api.cfg.Node.Token {
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }

    var full sync.FullConfig
    if err := json.NewDecoder(r.Body).Decode(&full); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    tx, err := api.db.Begin()
    if err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback()

    // 临时关闭外键
    if _, err := tx.Exec("PRAGMA foreign_keys=OFF"); err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    defer tx.Exec("PRAGMA foreign_keys=ON")

    // 清空现有数据（顺序：records, domains, users）
    if _, err := tx.Exec("DELETE FROM records"); err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    if _, err := tx.Exec("DELETE FROM domains"); err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    if _, err := tx.Exec("DELETE FROM users"); err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }

    // 插入新数据
    for _, user := range full.Users {
        _, err = tx.Exec(`
            INSERT INTO users (id, username, password_hash, is_admin, created_at)
            VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
        `, user.ID, user.Username, user.PasswordHash, boolToInt(user.IsAdmin))
        if err != nil {
            http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
            return
        }
        for _, domain := range user.Domains {
            _, err = tx.Exec(`
                INSERT INTO domains (id, user_id, domain, created_at)
                VALUES (?, ?, ?, CURRENT_TIMESTAMP)
            `, domain.ID, user.ID, domain.Domain)
            if err != nil {
                http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
                return
            }
            for _, rec := range domain.Records {
                _, err = tx.Exec(`
                    INSERT INTO records (id, domain_id, rule_type, continent, isp, province, type, value, ttl)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                `, rec.ID, domain.ID, rec.RuleType, rec.Continent, rec.ISP, rec.Province, rec.Type, rec.Value, rec.TTL)
                if err != nil {
                    http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
                    return
                }
            }
        }
    }
    if err := tx.Commit(); err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}

// incrementalHandler 接收增量操作
func (api *API) incrementalHandler(w http.ResponseWriter, r *http.Request) {
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, "Bearer ") {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    token := strings.TrimPrefix(auth, "Bearer ")
    if token != api.cfg.Node.Token {
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }

    var op struct {
        Op   string          `json:"op"`
        Data json.RawMessage `json:"data"`
    }
    if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    switch op.Op {
    case "add_domain":
        var data struct {
            UserID   int64  `json:"user_id"`
            Domain   string `json:"domain"`
            DomainID int64  `json:"domain_id"`
        }
        if err := json.Unmarshal(op.Data, &data); err != nil {
            http.Error(w, "bad data", http.StatusBadRequest)
            return
        }
        _, err := api.db.Exec(`
            INSERT INTO domains (id, user_id, domain, created_at)
            VALUES (?, ?, ?, CURRENT_TIMESTAMP)
        `, data.DomainID, data.UserID, data.Domain)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
    case "del_domain":
        var data struct {
            DomainID int64 `json:"domain_id"`
        }
        if err := json.Unmarshal(op.Data, &data); err != nil {
            http.Error(w, "bad data", http.StatusBadRequest)
            return
        }
        _, err := api.db.Exec("DELETE FROM domains WHERE id = ?", data.DomainID)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
    case "rename_domain":
        var data struct {
            DomainID  int64  `json:"domain_id"`
            NewDomain string `json:"new_domain"`
        }
        if err := json.Unmarshal(op.Data, &data); err != nil {
            http.Error(w, "bad data", http.StatusBadRequest)
            return
        }
        _, err := api.db.Exec("UPDATE domains SET domain = ? WHERE id = ?", data.NewDomain, data.DomainID)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
    case "replace_records":
        var data struct {
            DomainID int64                `json:"domain_id"`
            Records  []sync.RecordConfig `json:"records"`
        }
        if err := json.Unmarshal(op.Data, &data); err != nil {
            http.Error(w, "bad data", http.StatusBadRequest)
            return
        }
        tx, _ := api.db.Begin()
        defer tx.Rollback()
        _, err := tx.Exec("DELETE FROM records WHERE domain_id = ?", data.DomainID)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
        for _, rec := range data.Records {
            _, err = tx.Exec(`
                INSERT INTO records (id, domain_id, rule_type, continent, isp, province, type, value, ttl)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            `, rec.ID, data.DomainID, rec.RuleType, rec.Continent, rec.ISP, rec.Province, rec.Type, rec.Value, rec.TTL)
            if err != nil {
                http.Error(w, "db error", http.StatusInternalServerError)
                return
            }
        }
        tx.Commit()
    case "del_user":
        var data struct {
            UserID int64 `json:"user_id"`
        }
        if err := json.Unmarshal(op.Data, &data); err != nil {
            http.Error(w, "bad data", http.StatusBadRequest)
            return
        }
        _, err := api.db.Exec("DELETE FROM users WHERE id = ?", data.UserID)
        if err != nil {
            http.Error(w, "db error", http.StatusInternalServerError)
            return
        }
    default:
        http.Error(w, "unknown operation", http.StatusBadRequest)
        return
    }
    w.WriteHeader(http.StatusOK)
}

func (api *API) checkRunHandler(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token != api.cfg.Node.Token {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.TaskID = ""
	}
	// 执行本地测活
	results := api.runLocalCheck(req.TaskID)
	// 上报到主控
	reportURL := api.cfg.Node.MasterURL + "/api/check/report"
	reportBody := map[string]interface{}{
		"node_id": api.cfg.Node.ID,
		"task_id": req.TaskID,
		"results": results,
	}
	jsonBody, _ := json.Marshal(reportBody)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(reportURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		logger.Warn("failed to report check results", "error", err)
	} else {
		defer resp.Body.Close()
	}
	w.WriteHeader(http.StatusOK)
}

func (api *API) runLocalCheck(taskID string) []map[string]interface{} {
    rows, err := api.db.Query(`SELECT id, type, value FROM records`)
    if err != nil {
        logger.Error("query records for check failed", "error", err)
        return nil
    }
    defer rows.Close()
    var results []map[string]interface{}
    for rows.Next() {
        var id int64
        var recType, value string
        rows.Scan(&id, &recType, &value)
        success, invalid, msg := check.CheckRecord(recType, value)
        results = append(results, map[string]interface{}{
            "record_id": id,
            "success":   success,
            "message":   msg,
            "invalid":   invalid,
        })
    }
    return results
}

func boolToInt(b bool) int {
    if b {
        return 1
    }
    return 0
}
