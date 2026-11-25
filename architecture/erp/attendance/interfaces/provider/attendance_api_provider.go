// Package provider 是 attendance 域契约的进程内实现。
package provider

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/api"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
)

// 编译期断言：AttendanceApiProvider 实现了 attendance 域契约。
var _ api.AttendanceApi = (*AttendanceApiProvider)(nil)

// AttendanceApiProvider 契约实现：委托应用服务，契约类型 ↔ 应用层类型转换。
type AttendanceApiProvider struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceApiProvider 构造提供方。
func NewAttendanceApiProvider(applicationService *application.AttendanceApplicationService) *AttendanceApiProvider {
	return &AttendanceApiProvider{applicationService: applicationService}
}

// FindByID 实现 AttendanceApi。
func (p *AttendanceApiProvider) FindByID(ctx context.Context, id string) (api.AttendanceRecordDto, error) {
	view, err := p.applicationService.FindByID(ctx, id)
	if err != nil {
		return api.AttendanceRecordDto{}, err
	}
	return toDto(view), nil
}

// FindByStudentID 实现 AttendanceApi。
func (p *AttendanceApiProvider) FindByStudentID(ctx context.Context, studentID string) ([]api.AttendanceRecordDto, error) {
	views, err := p.applicationService.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	dtos := make([]api.AttendanceRecordDto, 0, len(views))
	for _, v := range views {
		dtos = append(dtos, toDto(v))
	}
	return dtos, nil
}

// ClockIn 实现 AttendanceApi：契约命令 → 应用命令。
func (p *AttendanceApiProvider) ClockIn(ctx context.Context, command api.ClockInCommand) (api.AttendanceRecordDto, error) {
	view, err := p.applicationService.ClockIn(ctx, application.ClockInCommand{
		StudentID:   command.StudentID,
		ClockInTime: command.ClockInTime,
		FaceToken:   command.FaceToken,
	})
	if err != nil {
		return api.AttendanceRecordDto{}, err
	}
	return toDto(view), nil
}

func toDto(v application.AttendanceRecordView) api.AttendanceRecordDto {
	return api.AttendanceRecordDto{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime,
	}
}
