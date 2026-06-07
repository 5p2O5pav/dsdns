package db

const nodeSchema = `
PRAGMA foreign_keys=OFF;

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    domain TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS records (
    id INTEGER PRIMARY KEY,
    domain_id INTEGER NOT NULL,
    rule_type TEXT NOT NULL,
    continent TEXT,
    isp TEXT,
    province TEXT,
    type TEXT NOT NULL,
    value TEXT NOT NULL,
    ttl INTEGER NOT NULL DEFAULT 600,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
);

-- 增量变更日志表
CREATE TABLE IF NOT EXISTS sync_log (
    id INTEGER PRIMARY KEY,
    op TEXT NOT NULL,
    data TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 子节点 nodes 表（可选，用于兼容性）
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY,
    name TEXT,
    endpoint TEXT,
    token TEXT,
    last_heartbeat DATETIME,
    status TEXT,
    created_at DATETIME,
    last_sync_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_records_domain_id ON records(domain_id);
CREATE INDEX IF NOT EXISTS idx_domains_domain ON domains(domain);
CREATE INDEX IF NOT EXISTS idx_domains_user_id ON domains(user_id);

PRAGMA foreign_keys=ON;
`
