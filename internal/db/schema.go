package db

const schema = `
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
    FOREIGN KEY (user_id) REFERENCES users(id)
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

CREATE INDEX IF NOT EXISTS idx_records_domain_id ON records(domain_id);
CREATE INDEX IF NOT EXISTS idx_domains_domain ON domains(domain);

-- 节点表（包含 last_sync_at 列）
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL UNIQUE,
    token TEXT NOT NULL,
    last_heartbeat DATETIME,
    status TEXT DEFAULT 'unknown',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_sync_at DATETIME
);

-- 测活结果表
CREATE TABLE IF NOT EXISTS check_results (
    id INTEGER PRIMARY KEY,
    record_id INTEGER NOT NULL,
    node_id INTEGER NOT NULL,
    success INTEGER NOT NULL,
    message TEXT,
    invalid INTEGER DEFAULT 0,
    checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    task_id TEXT,
    FOREIGN KEY (record_id) REFERENCES records(id) ON DELETE CASCADE,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- 通知表
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    content TEXT,
    is_resolved INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 增量变更日志表
CREATE TABLE IF NOT EXISTS sync_log (
    id INTEGER PRIMARY KEY,
    op TEXT NOT NULL,
    data TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

const pragmas = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
`
