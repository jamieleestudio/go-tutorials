// Package repository 是 system 域的仓储端口（对应 Java 版 domain.repository）。
// 领域层定义接口，infrastructure 层实现 —— 依赖倒置。
package repository

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/domain/model"
)

// SysUserRepository 系统用户仓储端口。
type SysUserRepository interface {
	// Save 保存聚合，返回持久化后的聚合（ID 为空时由实现生成）。
	Save(ctx context.Context, user *model.SysUser) (*model.SysUser, error)
	// FindByID 按主键查询；不存在时返回 (nil, nil) —— 对应 Java 的 Optional.empty()，
	// 404 错误消息由应用层拼装（保持与 Java 版一致的职责划分）。
	FindByID(ctx context.Context, id string) (*model.SysUser, error)
}
