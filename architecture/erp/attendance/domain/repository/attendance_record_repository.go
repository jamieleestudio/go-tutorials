// Package repository 是 attendance 域的仓储端口。
package repository

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/model"
)

// AttendanceRecordRepository 考勤记录仓储端口。
type AttendanceRecordRepository interface {
	// Save 保存聚合，返回持久化后的聚合（ID 为空时由实现生成）。
	Save(ctx context.Context, record *model.AttendanceRecord) (*model.AttendanceRecord, error)
	// FindByID 不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, id string) (*model.AttendanceRecord, error)
	// FindByStudentID 查询某学生的全部考勤记录（无匹配时返回空切片）。
	FindByStudentID(ctx context.Context, studentID string) ([]*model.AttendanceRecord, error)
}
