package web

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"dsdns/internal/logger"
	"dsdns/internal/sync"
)

// handleDomains 处理域名列表和创建
func (h *Handler) handleDomains(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		var rows *sql.Rows
		var err error
		if claims.IsAdmin {
			rows, err = h.DB.Query("SELECT id, user_id, domain, created_at FROM domains ORDER BY id")
		} else {
			rows, err = h.DB.Query("SELECT id, user_id, domain, created_at FROM domains WHERE user_id = ? ORDER BY id", claims.UserID)
		}
		if err != nil {
			logger.Error("query domains failed", "error", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type Domain struct {
			ID        int64  `json:"id"`
			UserID    int64  `json:"user_id"`
			Domain    string `json:"domain"`
			CreatedAt string `json:"created_at"`
		}
		var domains []Domain
		for rows.Next() {
			var d Domain
			if err := rows.Scan(&d.ID, &d.UserID, &d.Domain, &d.CreatedAt); err != nil {
				continue
			}
			domains = append(domains, d)
		}
		json.NewEncoder(w).Encode(domains)

	case http.MethodPost:
		var input struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		domain := strings.TrimSpace(input.Domain)
		if domain == "" {
			http.Error(w, "domain required", http.StatusBadRequest)
			return
		}
		if !strings.HasSuffix(domain, ".") {
			domain = domain + "."
		}
		var count int
		err := h.DB.QueryRow("SELECT COUNT(*) FROM domains WHERE domain = ?", domain).Scan(&count)
		if err != nil {
			logger.Error("check domain exists failed", "error", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "domain already exists", http.StatusConflict)
			return
		}
		res, err := h.DB.Exec("INSERT INTO domains (user_id, domain) VALUES (?, ?)", claims.UserID, domain)
		if err != nil {
			logger.Error("insert domain failed", "error", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      id,
			"domain":  domain,
			"user_id": claims.UserID,
		})

		go sync.NotifyNodesIncremental(h.DB, h.Config.Controller.SyncTimeoutSec, sync.IncrementalOp{
			Op: "add_domain",
			Data: map[string]interface{}{
				"user_id":   claims.UserID,
				"domain":    domain,
				"domain_id": id,
			},
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDomainByID 处理单个域名的获取、更新、删除
func (h *Handler) handleDomainByID(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/domains/")
	idStr := strings.Split(path, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid domain id", http.StatusBadRequest)
		return
	}

	var userID int64
	var domainName string
	err = h.DB.QueryRow("SELECT user_id, domain FROM domains WHERE id = ?", id).Scan(&userID, &domainName)
	if err == sql.ErrNoRows {
		http.Error(w, "domain not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("query domain failed", "error", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !claims.IsAdmin && userID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := h.DB.Query(`
			SELECT id, rule_type, continent, isp, province, type, value, ttl
			FROM records WHERE domain_id = ?
		`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		type Record struct {
			ID        int64  `json:"id"`
			RuleType  string `json:"rule_type"`
			Continent string `json:"continent"`
			ISP       string `json:"isp"`
			Province  string `json:"province"`
			Type      string `json:"type"`
			Value     string `json:"value"`
			TTL       int    `json:"ttl"`
		}
		records := make([]Record, 0)
		for rows.Next() {
			var rec Record
			if err := rows.Scan(&rec.ID, &rec.RuleType, &rec.Continent, &rec.ISP, &rec.Province, &rec.Type, &rec.Value, &rec.TTL); err != nil {
				continue
			}
			records = append(records, rec)
		}
		json.NewEncoder(w).Encode(records)

	case http.MethodPut:
		var newRecords []struct {
			RuleType  string `json:"rule_type"`
			Continent string `json:"continent"`
			ISP       string `json:"isp"`
			Province  string `json:"province"`
			Type      string `json:"type"`
			Value     string `json:"value"`
			TTL       int    `json:"ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&newRecords); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		tx, err := h.DB.Begin()
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		_, err = tx.Exec("DELETE FROM records WHERE domain_id = ?", id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		stmt, err := tx.Prepare(`
			INSERT INTO records (domain_id, rule_type, continent, isp, province, type, value, ttl)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()
		for _, rec := range newRecords {
			_, err = stmt.Exec(id, rec.RuleType, rec.Continent, rec.ISP, rec.Province, rec.Type, rec.Value, rec.TTL)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		go sync.NotifyNodesIncremental(h.DB, h.Config.Controller.SyncTimeoutSec, sync.IncrementalOp{
			Op: "replace_records",
			Data: map[string]interface{}{
				"domain_id": id,
				"records":   newRecords,
			},
		})
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "updated"})

	case http.MethodPatch:
		var input struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		newDomain := strings.TrimSpace(input.Domain)
		if newDomain == "" {
			http.Error(w, "domain required", http.StatusBadRequest)
			return
		}
		if !strings.HasSuffix(newDomain, ".") {
			newDomain = newDomain + "."
		}
		var count int
		err = h.DB.QueryRow("SELECT COUNT(*) FROM domains WHERE domain = ? AND id != ?", newDomain, id).Scan(&count)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "domain already exists", http.StatusConflict)
			return
		}
		_, err = h.DB.Exec("UPDATE domains SET domain = ? WHERE id = ?", newDomain, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		go sync.NotifyNodesIncremental(h.DB, h.Config.Controller.SyncTimeoutSec, sync.IncrementalOp{
			Op: "rename_domain",
			Data: map[string]interface{}{
				"domain_id":  id,
				"new_domain": newDomain,
			},
		})
		json.NewEncoder(w).Encode(map[string]string{"message": "renamed"})

	case http.MethodDelete:
		_, err = h.DB.Exec("DELETE FROM domains WHERE id = ?", id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		go sync.NotifyNodesIncremental(h.DB, h.Config.Controller.SyncTimeoutSec, sync.IncrementalOp{
			Op: "del_domain",
			Data: map[string]interface{}{
				"domain_id": id,
			},
		})
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
