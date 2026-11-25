// Package api 是 grade 域发布的契约（对应 Java 版 com.example.erp.grade.api）。
package api

import "context"

// GradeDto 成绩契约 DTO（对应 Java 版 GradeDto record）。
type GradeDto struct {
	ID         string  `json:"id"`
	StudentID  string  `json:"studentId"`
	CourseName string  `json:"courseName"`
	Score      float64 `json:"score"`
}

// GradeApi 成绩域对外契约。attendance 域跨域调用它 —— 这是本教程里
// "域间只走 api" 的实际案例。
type GradeApi interface {
	// FindByID 按主键查询；不存在时返回 ENTITY_NOT_FOUND 错误。
	FindByID(ctx context.Context, id string) (GradeDto, error)
	// FindByStudentID 查询某学生的全部成绩。
	FindByStudentID(ctx context.Context, studentID string) ([]GradeDto, error)
	// Create 创建成绩记录。
	Create(ctx context.Context, studentID, courseName string, score float64) (GradeDto, error)
}
