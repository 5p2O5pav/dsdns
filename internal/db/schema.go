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

-- 新版 notifications 表，去掉 is_resolved，增加 user_id
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    content TEXT,
    user_id INTEGER,           -- 关联的用户 ID，NULL 表示节点告警或系统通知
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

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

// MigrateNotifications 升级旧表结构（如果存在 is_resolved 列）
func MigrateNotifications(db *sql.DB) error {
    // 检查 notifications 表是否有 is_resolved 列
    var count int
    err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notifications') WHERE name='is_resolved'`).Scan(&count)
    if err != nil {
        return err
    }
    if count > 0 {
        // 需要重建表
        _, err = db.Exec(`
            CREATE TABLE notifications_new (
                id INTEGER PRIMARY KEY,
                type TEXT NOT NULL,
                content TEXT,
                user_id INTEGER,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            );
            INSERT INTO notifications_new (id, type, content, created_at)
            SELECT id, type, content, created_at FROM notifications;
            DROP TABLE notifications;
            ALTER TABLE notifications_new RENAME TO notifications;
        `)
        if err != nil {
            return err
        }
    }
    // 增加 user_id 列（如果不存在）
    err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notifications') WHERE name='user_id'`).Scan(&count)
    if err != nil {
        return err
    }
    if count == 0 {
        _, err = db.Exec(`ALTER TABLE notifications ADD COLUMN user_id INTEGER REFERENCES users(id)`)
        if err != nil {
            return err
        }
    }
    return nil
}
