package database

import (
	"database/sql"
	"time"

	"dict_new/config"

	_ "github.com/go-sql-driver/mysql"
)

type Connection struct {
	DB     *sql.DB
	Config *config.Config
}

func NewConnection(cfg *config.Config) (*Connection, error) {
	dsn := cfg.GetDSN()
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// 设置连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	return &Connection{
		DB:     db,
		Config: cfg,
	}, nil
}

func (c *Connection) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}