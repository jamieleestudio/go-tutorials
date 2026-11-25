// Package api 是 system 域发布的契约（对应 Java 版 com.example.erp.system.api）。
//
// P3 的核心新概念：契约与实现分离 —— 其他域只允许 import 本包，不允许碰
// system 域的 application/domain/infrastructure/interfaces（架构护栏强制）。
// 本包只依赖标准库：契约必须稳定、可独立演进（P4 换 gRPC 时，这份契约就是 proto 的蓝本）。
package api

import "context"

// SysUserDto 系统用户契约 DTO（对应 Java 版 api.dto.SysUserDto record）。
type SysUserDto struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SystemApi 系统域对外契约（Go 习惯：接口方法名不带 get 前缀）。
type SystemApi interface {
	// CurrentTenantID 当前租户 ID。
	// Java 版从 ThreadLocal 读；Go 版租户随 ctx 传递（platform/kernel.TenantContext）。
	CurrentTenantID(ctx context.Context) string
	// FindByID 按主键查询；不存在时返回 ENTITY_NOT_FOUND 错误。
	FindByID(ctx context.Context, id string) (SysUserDto, error)
}
