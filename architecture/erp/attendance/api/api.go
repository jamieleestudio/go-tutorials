// Package api 是 attendance 域发布的契约（对应 Java 版 com.example.erp.attendance.api）。
package api

import (
	"context"
	"time"
)

// TopicClocked 签到事件主题（对应 Java 版 AttendanceTopics.CLOCKED）。
const TopicClocked = "erp.attendance.clocked"

// AttendanceRecordDto 考勤记录契约 DTO。
type AttendanceRecordDto struct {
	ID          string    `json:"id"`
	StudentID   string    `json:"studentId"`
	Status      string    `json:"status"`
	ClockInTime time.Time `json:"clockInTime"`
}

// ClockInCommand 签到命令（对应 Java 版 api.command.ClockInCommand record）。
// 契约包自带的命令类型 —— 跨域调用方与 P4 的 gRPC 契约都以它为准。
type ClockInCommand struct {
	StudentID   string    `json:"studentId"`
	ClockInTime time.Time `json:"clockInTime"`
	FaceToken   string    `json:"faceToken"`
}

// AttendanceApi 考勤域对外契约。
type AttendanceApi interface {
	// FindByID 按主键查询；不存在时返回 ENTITY_NOT_FOUND 错误。
	FindByID(ctx context.Context, id string) (AttendanceRecordDto, error)
	// FindByStudentID 查询某学生的全部考勤记录。
	FindByStudentID(ctx context.Context, studentID string) ([]AttendanceRecordDto, error)
	// ClockIn 签到（完整用例：跨域验证 + 事务 + 发布事件）。
	ClockIn(ctx context.Context, command ClockInCommand) (AttendanceRecordDto, error)
}
