// Package provider 是 system 域契约的进程内实现（对应 Java 版 interfaces.provider）。
//
// 契约只允许由本包实现（编译期断言 + 架构护栏双重保证）——
// P4 拆微服务时，把这里换成 gRPC client 实现即可，调用方代码不变。
package provider

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/api"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/application"
)

// 编译期断言：SystemApiProvider 实现了 system 域契约。
var _ api.SystemApi = (*SystemApiProvider)(nil)

// SystemApiProvider 契约实现：委托应用服务，并把应用层视图转换成契约 DTO。
type SystemApiProvider struct {
	applicationService *application.SystemApplicationService
}

// NewSystemApiProvider 构造提供方。
func NewSystemApiProvider(applicationService *application.SystemApplicationService) *SystemApiProvider {
	return &SystemApiProvider{applicationService: applicationService}
}

// CurrentTenantID 实现 SystemApi。
func (p *SystemApiProvider) CurrentTenantID(ctx context.Context) string {
	return p.applicationService.CurrentTenantID(ctx)
}

// FindByID 实现 SystemApi：应用层视图 → 契约 DTO。
func (p *SystemApiProvider) FindByID(ctx context.Context, id string) (api.SysUserDto, error) {
	view, err := p.applicationService.FindByID(ctx, id)
	if err != nil {
		return api.SysUserDto{}, err
	}
	return api.SysUserDto{ID: view.ID, Name: view.Name}, nil
}
