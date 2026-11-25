// Package config 应用配置，对应 Java 版 bootstrap 模块的 application.yml。
//
// Java 版 P2 把数据库从 MySQL 换成了 H2 内存库（这个阶段重点是模块化而非数据库）；
// Go 版对应地默认使用 SQLite 内存库 —— 零外部依赖，go run 即可启动。
// 需要真实 MySQL 时设置 DB_DSN 覆盖（可与 P1 共用同一个 docker-compose）。
package config

import "os"

// Config 应用运行配置。
type Config struct {
	Port         string // HTTP 监听端口，对应 server.port
	DSN          string // 数据库连接串，对应 spring.datasource.url（支持 sqlite:// 与 MySQL DSN）
	GormLogLevel string // gorm 日志级别：silent | error | warn | info
}

// Load 从环境变量读取配置。
// 默认 SQLite 共享内存库，对应 Java 版 jdbc:h2:mem:mmm;DB_CLOSE_DELAY=-1。
func Load() Config {
	return Config{
		Port: getenv("APP_PORT", "8081"),
		DSN:  getenv("DB_DSN", "sqlite://file:mmm?mode=memory&cache=shared"),
		GormLogLevel: getenv("GORM_LOG_LEVEL", "warn"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
