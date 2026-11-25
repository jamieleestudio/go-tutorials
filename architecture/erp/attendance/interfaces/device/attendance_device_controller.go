// Package device 是 attendance 域的 device 端接口（考勤机硬件，只签到）。
// 路径同样在 /openapi/ 前缀下（对外设备网关），受 X-App-Id 校验。
package device

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/web"
)

const timeLayout = "2006-01-02T15:04:05"

// DeviceClockRequest 考勤机上报请求（对应 Java 版 DeviceClockRequest）。
// 设备端多带 faceToken —— 触发领域端口的人脸校验。
type DeviceClockRequest struct {
	StudentID   string `json:"studentId"`
	ClockInTime string `json:"clockInTime"`
	FaceToken   string `json:"faceToken"`
}

// AttendanceDeviceResponse device 端响应（对应 Java 版 AttendanceDeviceResponse）。
type AttendanceDeviceResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	Status      string `json:"status"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceDeviceController device 端控制器（只上报，无查询）。
type AttendanceDeviceController struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceDeviceController 构造控制器。
func NewAttendanceDeviceController(applicationService *application.AttendanceApplicationService) *AttendanceDeviceController {
	return &AttendanceDeviceController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/openapi/v1/device/attendance")）。
func (ctl *AttendanceDeviceController) Register(r gin.IRouter) {
	r.POST("/openapi/v1/device/attendance/clock", ctl.clock)
}

// clock POST /openapi/v1/device/attendance/clock
func (ctl *AttendanceDeviceController) clock(c *gin.Context) {
	var req DeviceClockRequest
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
		FaceToken:   req.FaceToken,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// parseClockInTime device 端自己的时间解析约定。
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

func toResponse(v application.AttendanceRecordView) AttendanceDeviceResponse {
	return AttendanceDeviceResponse{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime.Format(timeLayout),
	}
}
