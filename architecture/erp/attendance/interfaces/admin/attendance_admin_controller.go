// Package admin 是 attendance 域的 admin 端接口（后台管理，只读）。
package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
)

const timeLayout = "2006-01-02T15:04:05"

// AttendanceAdminResponse admin 端响应（对应 Java 版 AttendanceAdminResponse）。
type AttendanceAdminResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	Status      string `json:"status"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceAdminController admin 端控制器（只读：无 clock-in 路由）。
type AttendanceAdminController struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceAdminController 构造控制器。
func NewAttendanceAdminController(applicationService *application.AttendanceApplicationService) *AttendanceAdminController {
	return &AttendanceAdminController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/api/v1/admin/attendance")）。
func (ctl *AttendanceAdminController) Register(r gin.IRouter) {
	g := r.Group("/api/v1/admin/attendance")
	g.GET("", ctl.byStudent)
	g.GET("/:attendanceId", ctl.get)
}

// get GET /api/v1/admin/attendance/:attendanceId
func (ctl *AttendanceAdminController) get(c *gin.Context) {
	view, err := ctl.applicationService.FindByID(c.Request.Context(), c.Param("attendanceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// byStudent GET /api/v1/admin/attendance?studentId=
func (ctl *AttendanceAdminController) byStudent(c *gin.Context) {
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
	responses := make([]AttendanceAdminResponse, 0, len(views))
	for _, v := range views {
		responses = append(responses, toResponse(v))
	}
	c.JSON(http.StatusOK, responses)
}

func toResponse(v application.AttendanceRecordView) AttendanceAdminResponse {
	return AttendanceAdminResponse{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime.Format(timeLayout),
	}
}
