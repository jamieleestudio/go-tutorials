// Package repository 是 grade 域的仓储端口。
package repository

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/domain/model"
)

// GradeRepository 成绩仓储端口。
type GradeRepository interface {
	// Save 保存聚合，返回持久化后的聚合（ID 为空时由实现生成）。
	Save(ctx context.Context, grade *model.Grade) (*model.Grade, error)
	// FindByID 不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, id string) (*model.Grade, error)
	// FindByStudentID 查询某学生的全部成绩（无匹配时返回空切片）。
	FindByStudentID(ctx context.Context, studentID string) ([]*model.Grade, error)
}
