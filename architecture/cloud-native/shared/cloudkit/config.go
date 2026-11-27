package cloudkit

import "os"

// EnvOr 12-Factor 配置读取：环境变量优先，未设置时回退默认值。
// 对应 Java application.yml 的 ${PORT:8080} 占位符语法 ——
// 配置全部由平台（K8s ConfigMap/Secret → env）注入，镜像本身不含环境差异。
func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// AddrFromPort 监听地址：K8s 惯例容器统一监听 8080（对应 Java 的 server.port: 8080）。
// PORT 环境变量可覆盖 —— 本地 make run-cn 就是用它让三服务在同一台机器共存。
func AddrFromPort(defaultPort string) string {
	return ":" + EnvOr("PORT", defaultPort)
}
