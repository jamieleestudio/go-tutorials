// Package persistence 是 system 域的持久化实现（对应 Java 版 infrastructure.persistence）。
package persistence

import (
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
)

// SysUserEntity 系统用户实体（对应 Java 版 SysUserEntity extends BaseEntity）。
// 嵌入平台层 BaseModel 获得统一主键/软删/审计字段。
type SysUserEntity struct {
	persistence.BaseModel
	Name string `gorm:"column:name;type:varchar(128)"`
}

// TableName 对应 Java 版 @Table(name = "sys_user")。
func (SysUserEntity) TableName() string { return "sys_user" }
