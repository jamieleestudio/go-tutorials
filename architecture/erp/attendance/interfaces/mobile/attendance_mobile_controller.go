// Package mobile 是 attendance 域的 mobile 端接口（学生手机端，可读可签到）。
package mobile

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/web"
)

const timeLayout = "2006-01-02T15:04:05"

// MobileClockInRequest mobile 端签到请求（对应 Java 版 MobileClockInRequest）。
type MobileClockInRequest struct {
	StudentID   string `json:"studentId"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceMobileResponse mobile 端响应（对应 Java 版 AttendanceMobileResponse）。
type AttendanceMobileResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	Status      string `json:"status"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceMobileController mobile 端控制器。
type AttendanceMobileController struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceMobileController 构造控制器。
func NewAttendanceMobileController(applicationService *application.AttendanceApplicationService) *AttendanceMobileController {
	return &AttendanceMobileController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/api/v1/mobile/attendance")）。
func (ctl *AttendanceMobileController) Register(r gin.IRouter) {
	g := r.Group("/api/v1/mobile/attendance")
	g.POST("/clock-in", ctl.clockIn)
	g.GET("/:attendanceId", ctl.get)
}

// clockIn POST /api/v1/mobile/attendance/clock-in
func (ctl *AttendanceMobileController) clockIn(c *gin.Context) {
	var req MobileClockInRequest
	if !web.BindJSON(c, &req) {
		return
	}
	clockInTime, err := parseClockInTime(req.ClockInTime)
	if err != nil {
		_ = c.Error(err)
		return
	}
	view, err := ctl.applicationService.ClockIn(c.Request.Context(), application.ClockInCommand{
		StudentID:   req.StudentID,
		ClockInTime: clockInTime,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// get GET /api/v1/mobile/attendance/:attendanceId
func (ctl *AttendanceMobileController) get(c *gin.Context) {
	view, err := ctl.applicationService.FindByID(c.Request.Context(), c.Param("attendanceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// parseClockInTime mobile 端自己的时间解析约定（端 DTO 独立，格式可以各端自定）。
func parseClockInTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, kernel.NewBizException(kernel.CodeBadRequest, "clockInTime must not be null")
	}
	t, err := time.ParseInLocation(timeLayout, s, time.Local)
	if err != nil {
		return time.Time{}, kernel.NewBizException(kernel.CodeBadRequest, "invalid clockInTime, expect "+timeLayout)
	}
	return t, nil
}

func toResponse(v application.AttendanceRecordView) AttendanceMobileResponse {
	return AttendanceMobileResponse{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime.Format(timeLayout),
	}
}
