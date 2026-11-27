// 数据库连接管理 —— shared/persistence 的第二个职责（对应 Java 版每个服务
// application.yml 里的 spring.datasource 配置）。
//
// P4 微服务里没有"统一 bootstrap"：每个服务进程独立连自己的数据库，
// 但连接协议路由（SQLite 内存库默认 / DB_DSN 切 MySQL）是三服务共用逻辑，
// 与事务端口（tx.go）一样放在共享内核的 persistence 子包。
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
//
// DSN 协议路由：
//   - "sqlite://..." 开头 → SQLite（内存或文件，纯 Go 驱动，无 CGO）
//   - 其他                → MySQL DSN（如 monolith:monolith@tcp(localhost:13306)/ms_product）
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

// Migrate 自动建表（对应 Java 版 jpa.hibernate.ddl-auto=create-drop）。
// 实体类型由各服务自己的组合根传入 —— 每个服务只建自己的表。
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
