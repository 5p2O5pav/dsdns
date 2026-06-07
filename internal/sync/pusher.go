package sync

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "dsdns/internal/logger"
)

type FullConfig struct {
    Users []UserConfig `json:"users"`
}

type UserConfig struct {
    ID           int64          `json:"id"`
    Username     string         `json:"username"`
    PasswordHash string         `json:"password_hash"`
    IsAdmin      bool           `json:"is_admin"`
    Domains      []DomainConfig `json:"domains"`
}

type DomainConfig struct {
    ID      int64          `json:"id"`
    Domain  string         `json:"domain"`
    Records []RecordConfig `json:"records"`
}

type RecordConfig struct {
    ID        int64  `json:"id"`
    RuleType  string `json:"rule_type"`
    Continent string `json:"continent"`
    ISP       string `json:"isp"`
    Province  string `json:"province"`
    Type      string `json:"type"`
    Value     string `json:"value"`
    TTL       int    `json:"ttl"`
}

// ExportFullConfig 导出所有用户及其域名的完整配置（用于全量同步）
func ExportFullConfig(db *sql.DB) ([]byte, error) {
    usersRows, err := db.Query(`
        SELECT id, username, password_hash, is_admin
        FROM users
        ORDER BY id
    `)
    if err != nil {
        return nil, err
    }
    defer usersRows.Close()

    var users []UserConfig
    for usersRows.Next() {
        var u UserConfig
        if err := usersRows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin); err != nil {
            continue
        }
        domainRows, err := db.Query(`
            SELECT id, domain
            FROM domains
            WHERE user_id = ?
            ORDER BY id
        `, u.ID)
        if err != nil {
            return nil, err
        }
        for domainRows.Next() {
            var d DomainConfig
            if err := domainRows.Scan(&d.ID, &d.Domain); err != nil {
                domainRows.Close()
                return nil, err
            }
            recordRows, err := db.Query(`
                SELECT id, rule_type, continent, isp, province, type, value, ttl
                FROM records
                WHERE domain_id = ?
                ORDER BY id
            `, d.ID)
            if err != nil {
                domainRows.Close()
                return nil, err
            }
            for recordRows.Next() {
                var rec RecordConfig
                if err := recordRows.Scan(&rec.ID, &rec.RuleType, &rec.Continent, &rec.ISP, &rec.Province, &rec.Type, &rec.Value, &rec.TTL); err != nil {
                    recordRows.Close()
                    domainRows.Close()
                    return nil, err
                }
                d.Records = append(d.Records, rec)
            }
            recordRows.Close()
            u.Domains = append(u.Domains, d)
        }
        domainRows.Close()
        users = append(users, u)
    }
    full := FullConfig{Users: users}
    return json.Marshal(full)
}

// SyncToNode 全量同步到单个节点
func SyncToNode(db *sql.DB, nodeID int64, endpoint, token string, timeoutSec int) {
    configJSON, err := ExportFullConfig(db)
    if err != nil {
        logger.Error("export full config failed", "node_id", nodeID, "error", err)
        return
    }
    url := fmt.Sprintf("%s/api/sync", endpoint)
    req, err := http.NewRequest("POST", url, bytes.NewReader(configJSON))
    if err != nil {
        logger.Error("create full sync request failed", "error", err)
        return
    }
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")
    client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        logger.Warn("full sync to node failed", "node_id", nodeID, "error", err)
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusOK {
        logger.Info("full sync success", "node_id", nodeID)
        // 更新节点的 last_sync_at
        db.Exec(`UPDATE nodes SET last_sync_at = CURRENT_TIMESTAMP WHERE id = ?`, nodeID)
    } else {
        logger.Warn("full sync returned non-200", "node_id", nodeID, "status", resp.Status)
    }
}

// SyncToAllNodes 全量同步到所有在线节点（异步，供前端按钮调用）
func SyncToAllNodes(db *sql.DB, timeoutSec int) {
    rows, err := db.Query(`SELECT id, endpoint, token FROM nodes WHERE status = 'online' AND endpoint != ''`)
    if err != nil {
        logger.Error("query nodes for full sync failed", "error", err)
        return
    }
    defer rows.Close()
    sem := make(chan struct{}, 10) // 限制并发
    for rows.Next() {
        var id int64
        var endpoint, token string
        if err := rows.Scan(&id, &endpoint, &token); err != nil {
            continue
        }
        go func(nid int64, ep, tk string) {
            sem <- struct{}{}
            defer func() { <-sem }()
            SyncToNode(db, nid, ep, tk, timeoutSec)
        }(id, endpoint, token)
    }
}
