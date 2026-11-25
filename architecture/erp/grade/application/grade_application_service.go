// Package application 是 grade 域的应用层。
package application

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/domain/model"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/domain/repository"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/system/api"
)

// GradeView 应用层查询结果（与 api.GradeDto 分离，理由见 system 域同款注释）。
type GradeView struct {
	ID         string
	StudentID  string
	CourseName string
	Score      float64
}

// GradeApplicationService 成绩域应用服务。
type GradeApplicationService struct {
	repo      repository.GradeRepository
	systemApi api.SystemApi // 跨域依赖：只 import system 域的 api 契约包
}

// NewGradeApplicationService 构造应用服务。
func NewGradeApplicationService(repo repository.GradeRepository, systemApi api.SystemApi) *GradeApplicationService {
	return &GradeApplicationService{repo: repo, systemApi: systemApi}
}

// Create 创建成绩（对应 Java 版 create；先打通租户上下文，与 Java 行为一致）。
func (s *GradeApplicationService) Create(ctx context.Context, studentID, courseName string, score float64) (GradeView, error) {
	_ = s.systemApi.CurrentTenantID(ctx) // 跨域调用 system 契约（演示域间协作）
	grade, err := model.NewGrade(shared.NewID(), studentID, courseName, score)
	if err != nil {
		return GradeView{}, err
	}
	saved, err := s.repo.Save(ctx, grade)
	if err != nil {
		return GradeView{}, err
	}
	return toView(saved), nil
}

// FindByID 按主键查询。
func (s *GradeApplicationService) FindByID(ctx context.Context, id string) (GradeView, error) {
	grade, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return GradeView{}, err
	}
	if grade == nil {
		return GradeView{}, shared.NewEntityNotFound("Grade not found: " + id)
	}
	return toView(grade), nil
}

// FindByStudentID 查询某学生的全部成绩。
func (s *GradeApplicationService) FindByStudentID(ctx context.Context, studentID string) ([]GradeView, error) {
	grades, err := s.repo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	views := make([]GradeView, 0, len(grades))
	for _, g := range grades {
		views = append(views, toView(g))
	}
	return views, nil
}

func toView(g *model.Grade) GradeView {
	return GradeView{
		ID:         g.ID(),
		StudentID:  g.StudentID(),
		CourseName: g.CourseName(),
		Score:      g.Score(),
	}
}
