// Package persistence 数据库连接管理 —— bootstrap（部署单元）的职责。
//
// 对应 Java 版 bootstrap 的 application.yml + Spring Boot 自动配置：
// Java 版用 H2 内存库（这个阶段重点是模块化而非数据库），Go 版对应地
// 默认使用 SQLite 内存库 —— 零外部依赖，go run 即可启动。
//
// DSN 协议路由：
//   - "sqlite://..." 开头 → SQLite（内存或文件，纯 Go 驱动，无 CGO）
//   - 其他               → MySQL DSN（如 monolith:monolith@tcp(localhost:13306)/mmm）
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

// Open 按协议打开数据库连接。
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
		// SQLite 内存库每个连接是独立的数据库，必须限制为单连接，
		// 否则连接池分配到新连接时会"找不到表"（对应 H2 DB_CLOSE_DELAY=-1 的效果）。
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
	return db, nil
}

// Migrate 自动建表。
// 实体类型来自三个业务模块的 infrastructure 包 —— bootstrap 依赖全部模块（部署单元特权）。
// 对应 Java 版 jpa.hibernate.ddl-auto=create。
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
