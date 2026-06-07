package db

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
    // 增加连接池大小，支持并发写入
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(time.Hour)

	if _, err := db.Exec(pragmas); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
    // 执行通知表结构迁移
    if err := MigrateNotifications(db); err != nil {
        return nil, err
    }
	return db, nil
}

func OpenNodeDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
    // 增加连接池大小，支持并发写入
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(time.Hour)

	if _, err := db.Exec(pragmas); err != nil {
		return nil, err
	}
	if _, err := db.Exec(nodeSchema); err != nil {
		return nil, err
	}
    // 执行通知表结构迁移
    if err := MigrateNotifications(db); err != nil {
        return nil, err
    }
	return db, nil
}

// EnsureMasterNode 确保主控自身节点记录存在（id=0）
func EnsureMasterNode(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE id = 0").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.Exec("INSERT INTO nodes (id, name, endpoint, token, status) VALUES (0, 'master', '', '', 'online')")
		if err != nil {
			return err
		}
	}
	return nil
}
