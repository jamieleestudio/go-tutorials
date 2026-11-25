// Package persistence 是 grade 域的持久化实现。
package persistence

import (
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
)

// GradeEntity 成绩实体（对应 Java 版 GradeEntity，表 grade）。
type GradeEntity struct {
	persistence.BaseModel
	StudentID  string  `gorm:"column:student_id;type:varchar(36)"`
	CourseName string  `gorm:"column:course_name;type:varchar(128)"`
	Score      float64 `gorm:"column:score"`
}

// TableName 对应 @Table(name = "grade")。
func (GradeEntity) TableName() string { return "grade" }
