// Package application 是 system 域的应用层（对应 Java 版 application）。
//
// system 域是"纯提供方"：没有自己的 HTTP 接口，只通过 api 契约服务其他域
// （架构护栏 platform-rules 里有 system_is_a_pure_provider 规则）。
package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/domain/repository"
)

// SysUserView 应用层查询结果。
//
// Go 版有意让应用层拥有自己的 DTO（而不是复用 api.SysUserDto）：
// 契约（..api..）与本地用例（..application..）互不依赖 —— 这让 Java 版
// ApiContractRulesTest 注释里的 "independent" 在 Go 里成为真实的结构事实。
// 附带的好处：Go 是结构化类型，若应用层方法签名恰好与契约一致，
// 它会"无意中实现"契约接口 —— 类型分离让这一点不可能发生（见架构护栏测试）。
type SysUserView struct {
	ID   string
	Name string
}

// SystemApplicationService 系统域应用服务。
type SystemApplicationService struct {
	repo repository.SysUserRepository
}

// NewSystemApplicationService 构造应用服务。
func NewSystemApplicationService(repo repository.SysUserRepository) *SystemApplicationService {
	return &SystemApplicationService{repo: repo}
}

// CurrentTenantID 当前租户（对应 Java 版读取 TenantContext，缺省 "default"）。
// Go 版租户在 ctx value 里，由 middleware/调用方放进请求 ctx。
func (s *SystemApplicationService) CurrentTenantID(ctx context.Context) string {
	if tenant := kernel.CurrentTenant(ctx); tenant != "" {
		return tenant
	}
	return "default"
}

// FindByID 按主键查询。
func (s *SystemApplicationService) FindByID(ctx context.Context, id string) (SysUserView, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return SysUserView{}, err
	}
	if user == nil {
		return SysUserView{}, shared.NewEntityNotFound("SysUser not found: " + id)
	}
	return SysUserView{ID: user.ID(), Name: user.Name()}, nil
}
