// Package web 是 attendance 域的 web 端接口（PC 管理端用户，可读可写）。
// 各端 DTO 独立定义 —— 端与端之间互不依赖（架构护栏 end-isolation）。
package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/web"
)

// timeLayout web 端的时间格式（对应 Java LocalDateTime 的 JSON 序列化格式）。
const timeLayout = "2006-01-02T15:04:05"

// ClockInWebRequest 签到请求（对应 Java 版 interfaces.web.dto.ClockInWebRequest）。
// 时间用字符串承接 —— Java LocalDateTime 的 "2026-09-16T08:00:00" 没有时区，
// Go 的 time.Time JSON 解析要求 RFC3339，所以 web 端自己做格式约定。
type ClockInWebRequest struct {
	StudentID   string `json:"studentId"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceWebResponse 考勤响应（对应 Java 版 AttendanceWebResponse）。
type AttendanceWebResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	Status      string `json:"status"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceWebController web 端控制器（对应 Java 版 AttendanceWebController）。
type AttendanceWebController struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceWebController 构造控制器。
func NewAttendanceWebController(applicationService *application.AttendanceApplicationService) *AttendanceWebController {
	return &AttendanceWebController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/api/v1/attendance")）。
func (ctl *AttendanceWebController) Register(r gin.IRouter) {
	g := r.Group("/api/v1/attendance")
	g.POST("/clock-in", ctl.clockIn)
	g.GET("", ctl.byStudent)
	g.GET("/:attendanceId", ctl.get)
}

// clockIn POST /api/v1/attendance/clock-in
func (ctl *AttendanceWebController) clockIn(c *gin.Context) {
	var req ClockInWebRequest
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

// get GET /api/v1/attendance/:attendanceId
func (ctl *AttendanceWebController) get(c *gin.Context) {
	view, err := ctl.applicationService.FindByID(c.Request.Context(), c.Param("attendanceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// byStudent GET /api/v1/attendance?studentId=
func (ctl *AttendanceWebController) byStudent(c *gin.Context) {
	studentID := c.Query("studentId")
	if studentID == "" {
		_ = c.Error(kernel.NewBizException(kernel.CodeBadRequest, "studentId query param is required"))
		return
	}
	views, err := ctl.applicationService.FindByStudentID(c.Request.Context(), studentID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	responses := make([]AttendanceWebResponse, 0, len(views))
	for _, v := range views {
		responses = append(responses, toResponse(v))
	}
	c.JSON(http.StatusOK, responses)
}

// parseClockInTime web 端的时间解析约定（失败抛 400 校验错误）。
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

func toResponse(v application.AttendanceRecordView) AttendanceWebResponse {
	return AttendanceWebResponse{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime.Format(timeLayout),
	}
}
