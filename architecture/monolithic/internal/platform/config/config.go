// Package config 应用配置，对应 Java 版的 application.yml。
//
// Spring 用 yml + @ConfigurationProperties；Go 用环境变量 + 默认值（12-factor 风格）。
package config

import "os"

// Config 应用运行配置。
type Config struct {
	Port         string // HTTP 监听端口，对应 server.port
	MySQLDSN     string // 数据库连接串，对应 spring.datasource.url
	GormLogLevel string // gorm 日志级别：silent | error | warn | info
}

// Load 从环境变量读取配置，缺省值与 docker-compose.yml 保持一致。
func Load() Config {
	return Config{
		Port:         getenv("APP_PORT", "8080"),
		MySQLDSN:     getenv("DB_DSN", "monolith:monolith@tcp(localhost:13306)/monolith?charset=utf8mb4&parseTime=True&loc=Local"),
		GormLogLevel: getenv("GORM_LOG_LEVEL", "warn"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
