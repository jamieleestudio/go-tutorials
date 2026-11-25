// Package internalapi 是 attendance 域的 internal 端接口（内部服务间调用，只读）。
//
// 命名说明：目录不能叫 internal —— Go 编译器把 internal 目录视为强制可见性边界
// （只有 attendance 域内代码可导入），组合根就无法注册它的路由了。
// 这是 Java 端命名与 Go internal 语义的碰撞点，Go 版改名为 internalapi。
// Java 版它只暴露按 ID 查询；Go 版保持一致。
package internalapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
)

const timeLayout = "2006-01-02T15:04:05"

// AttendanceInternalResponse internal 端响应（对应 Java 版 AttendanceInternalResponse）。
type AttendanceInternalResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	Status      string `json:"status"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceInternalController internal 端控制器（只读，单查询）。
type AttendanceInternalController struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceInternalController 构造控制器。
func NewAttendanceInternalController(applicationService *application.AttendanceApplicationService) *AttendanceInternalController {
	return &AttendanceInternalController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/internal/v1/attendance")）。
func (ctl *AttendanceInternalController) Register(r gin.IRouter) {
	r.GET("/internal/v1/attendance/:attendanceId", ctl.get)
}

// get GET /internal/v1/attendance/:attendanceId
func (ctl *AttendanceInternalController) get(c *gin.Context) {
	view, err := ctl.applicationService.FindByID(c.Request.Context(), c.Param("attendanceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

func toResponse(v application.AttendanceRecordView) AttendanceInternalResponse {
	return AttendanceInternalResponse{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime.Format(timeLayout),
	}
}
