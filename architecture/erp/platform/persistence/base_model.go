// Package persistence 是平台层的持久化底座（对应 Java 版 erp-platform-persistence）。
//
// 提供：BaseModel（实体基类）、TxRunner + DBFromContext（事务传播，Go 版的
// "@Transactional"）、Open（SQLite/MySQL 路由）。
package persistence

import "time"

// BaseModel 实体基类（对应 Java 版 BaseEntity 的 @MappedSuperclass）。
//
// 业务实体嵌入它获得统一的主键/软删/审计字段 —— 对应 Java 版的 id / is_del /
// create_time / update_time 四列。gorm 的嵌入 struct 字段会被提升到实体表。
//
// 用法：RepositoryImpl 创建实体时填 ID/CreatedAt/UpdatedAt，Deleted 留 false；
// gorm 的 DeletedAt 协议字段这里不用（erp 用显式 is_del 布尔列对齐 Java 版）。
type BaseModel struct {
	ID        string    `gorm:"column:id;type:varchar(36);primaryKey" json:"id"`
	Deleted   bool      `gorm:"column:is_del" json:"deleted"`
	CreatedAt time.Time `gorm:"column:create_time" json:"createTime"`
	UpdatedAt time.Time `gorm:"column:update_time" json:"updateTime"`
}
