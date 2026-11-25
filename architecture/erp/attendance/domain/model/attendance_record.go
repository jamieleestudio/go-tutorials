// Package model 是 attendance 域的领域模型。
package model

import (
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/event"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
)

// AttendanceStatus 考勤状态（对应 Java 版枚举）。
type AttendanceStatus string

const (
	StatusPresent AttendanceStatus = "PRESENT"
	StatusLate    AttendanceStatus = "LATE"
	StatusAbsent  AttendanceStatus = "ABSENT"
)

// lateThresholdMinutes 迟到阈值：晚于课程开始 10 分钟记迟到。
const lateThresholdMinutes = 10

// AttendanceRecord 考勤记录聚合根（对应 Java 版 AttendanceRecord extends AggregateRoot）。
type AttendanceRecord struct {
	shared.AggregateRoot
	id          string
	studentID   string
	status      AttendanceStatus
	clockInTime time.Time
}

// NewAttendanceRecord 重建聚合（仓储重水合用，不走签到规则 —— 对应 Java 的公有构造器）。
func NewAttendanceRecord(id, studentID string, status AttendanceStatus, clockInTime time.Time) *AttendanceRecord {
	return &AttendanceRecord{id: id, studentID: studentID, status: status, clockInTime: clockInTime}
}

// ClockIn 签到工厂（对应 Java 的静态工厂 AttendanceRecord.clockIn）。
// 规则：studentId/clockInTime 必填；scheduleStart 非空且晚于开始 10 分钟 → LATE，否则 PRESENT；
// 同时注册领域事件。
func ClockIn(id, studentID string, clockInTime time.Time, scheduleStart *time.Time) (*AttendanceRecord, error) {
	if studentID == "" {
		return nil, shared.NewBusinessRuleViolation("studentId must not be blank")
	}
	if clockInTime.IsZero() {
		return nil, shared.NewBusinessRuleViolation("clockInTime must not be null")
	}
	status := StatusPresent
	if scheduleStart != nil && clockInTime.After(scheduleStart.Add(lateThresholdMinutes*time.Minute)) {
		status = StatusLate
	}
	record := NewAttendanceRecord(id, studentID, status, clockInTime)
	record.RegisterEvent(event.NewAttendanceClockedEvent(id, studentID, string(status), clockInTime))
	return record, nil
}

// MarkAbsent 标记缺勤（对应 Java 的 markAbsent）。
func (r *AttendanceRecord) MarkAbsent() { r.status = StatusAbsent }

func (r *AttendanceRecord) ID() string               { return r.id }
func (r *AttendanceRecord) StudentID() string        { return r.studentID }
func (r *AttendanceRecord) Status() AttendanceStatus { return r.status }
func (r *AttendanceRecord) ClockInTime() time.Time   { return r.clockInTime }
