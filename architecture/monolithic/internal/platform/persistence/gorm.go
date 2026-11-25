// Package persistence 持久化平台基础设施：GORM 连接、建表、事务传播。
//
// 它是横切的基础设施（不属于任何业务上下文），对应 Java 版里
// spring-boot-starter-data-jpa 帮你自动做掉的那部分工作。
package persistence

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开 MySQL 连接。
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
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(lvl)})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

// Migrate 自动建表。
// 对应 Java 版 jpa.hibernate.ddl-auto=create-drop 的开发期便利；
// 生产环境应当用迁移工具（golang-migrate / atlas）替代。
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}
