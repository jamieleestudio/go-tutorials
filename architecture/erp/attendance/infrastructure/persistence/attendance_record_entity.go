// Package persistence 是 attendance 域的持久化实现。
package persistence

import (
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
)

// AttendanceRecordEntity 考勤记录实体（对应 Java 版 AttendanceRecordEntity，表 attendance_record）。
type AttendanceRecordEntity struct {
	persistence.BaseModel
	StudentID   string    `gorm:"column:student_id;type:varchar(36)"`
	Status      string    `gorm:"column:status;type:varchar(16)"`
	ClockInTime time.Time `gorm:"column:clock_in_time"`
}

// TableName 对应 @Table(name = "attendance_record")。
func (AttendanceRecordEntity) TableName() string { return "attendance_record" }
