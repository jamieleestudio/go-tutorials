// Package event 是 attendance 域的领域事件（对应 Java 版 domain.event）。
package event

import "time"

// AttendanceClockedEvent 签到领域事件（对应 Java 版 record implements DomainEvent）。
type AttendanceClockedEvent struct {
	RecordID   string
	StudentID  string
	Status     string
	OccurredAt time.Time
}

// NewAttendanceClockedEvent 构造事件。
func NewAttendanceClockedEvent(recordID, studentID, status string, occurredAt time.Time) AttendanceClockedEvent {
	return AttendanceClockedEvent{RecordID: recordID, StudentID: studentID, Status: status, OccurredAt: occurredAt}
}

// EventName 实现共享内核的 DomainEvent 接口。
func (e AttendanceClockedEvent) EventName() string { return "AttendanceClockedEvent" }
