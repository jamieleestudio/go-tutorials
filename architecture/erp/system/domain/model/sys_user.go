// Package model 是 system 域的领域模型（对应 Java 版 domain.model）。
package model

import "github.com/jamieleestudio/go-tutorials/architecture/erp/shared"

// SysUser 系统用户聚合根（对应 Java 版 SysUser extends AggregateRoot）。
type SysUser struct {
	shared.AggregateRoot
	id   string
	name string
}

// NewSysUser 构造聚合（id 允许为空 —— 由仓储在持久化时生成，对应 Java 版
// repository.save 里 IdGenerator.next() 的时机）。
func NewSysUser(id, name string) (*SysUser, error) {
	u := &SysUser{id: id}
	if err := u.Rename(name); err != nil {
		return nil, err
	}
	return u, nil
}

// Rename 改名 —— 业务规则：名字不能为空（对应 Java 的 rename）。
func (u *SysUser) Rename(name string) error {
	if name == "" {
		return shared.NewBusinessRuleViolation("SysUser name must not be blank")
	}
	u.name = name
	return nil
}

func (u *SysUser) ID() string   { return u.id }
func (u *SysUser) Name() string { return u.name }
