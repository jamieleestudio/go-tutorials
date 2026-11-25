package persistence

import (
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 按协议打开数据库连接（对应 Java 版各 app 的 application.yml datasource 配置）。
//
// DSN 协议路由：
//   - "sqlite://..." 开头 → SQLite（内存或文件，纯 Go 驱动，无 CGO）
//   - 其他               → MySQL DSN
func Open(dsn, logLevel string) (*gorm.DB, error) {
	lvl := logger.Warn
	switch logLevel {
	case "info":
		lvl = logger.Info
	case "error":
		lvl = logger.Error
	case "silent":
		lvl = logger.Silent
	}

	var (
		dialector gorm.Dialector
		isSQLite  bool
	)
	switch {
	case strings.HasPrefix(dsn, "sqlite://"):
		dialector = sqlite.Open(strings.TrimPrefix(dsn, "sqlite://"))
		isSQLite = true
	default:
		dialector = mysql.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(lvl)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if isSQLite {
		// SQLite 内存库每个连接是独立的数据库，必须限制单连接，
		// 否则连接池分到新连接时会"找不到表"（对应 H2 DB_CLOSE_DELAY=-1）。
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
	return db, nil
}

// Migrate 自动建表（对应 Java 版 jpa.hibernate.ddl-auto=create）。
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
