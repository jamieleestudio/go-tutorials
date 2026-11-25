// Package provider 是 grade 域契约的进程内实现。
package provider

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/api"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/application"
)

// 编译期断言：GradeApiProvider 实现了 grade 域契约。
var _ api.GradeApi = (*GradeApiProvider)(nil)

// GradeApiProvider 契约实现：委托应用服务，应用层视图 → 契约 DTO。
type GradeApiProvider struct {
	applicationService *application.GradeApplicationService
}

// NewGradeApiProvider 构造提供方。
func NewGradeApiProvider(applicationService *application.GradeApplicationService) *GradeApiProvider {
	return &GradeApiProvider{applicationService: applicationService}
}

// FindByID 实现 GradeApi。
func (p *GradeApiProvider) FindByID(ctx context.Context, id string) (api.GradeDto, error) {
	view, err := p.applicationService.FindByID(ctx, id)
	if err != nil {
		return api.GradeDto{}, err
	}
	return toDto(view), nil
}

// FindByStudentID 实现 GradeApi。
func (p *GradeApiProvider) FindByStudentID(ctx context.Context, studentID string) ([]api.GradeDto, error) {
	views, err := p.applicationService.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	dtos := make([]api.GradeDto, 0, len(views))
	for _, v := range views {
		dtos = append(dtos, toDto(v))
	}
	return dtos, nil
}

// Create 实现 GradeApi。
func (p *GradeApiProvider) Create(ctx context.Context, studentID, courseName string, score float64) (api.GradeDto, error) {
	view, err := p.applicationService.Create(ctx, studentID, courseName, score)
	if err != nil {
		return api.GradeDto{}, err
	}
	return toDto(view), nil
}

func toDto(v application.GradeView) api.GradeDto {
	return api.GradeDto{
		ID:         v.ID,
		StudentID:  v.StudentID,
		CourseName: v.CourseName,
		Score:      v.Score,
	}
}
