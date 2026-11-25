// Package openapi 是 attendance 域的 openapi 端接口（对外开放 API，只读）。
// 注意：路径在 /openapi/ 前缀下 —— 平台层 AuthMiddleware 会要求 X-App-Id。
package openapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
)

const timeLayout = "2006-01-02T15:04:05"

// AttendanceOpenApiResponse openapi 端响应（对应 Java 版 AttendanceOpenApiResponse）。
type AttendanceOpenApiResponse struct {
	ID          string `json:"id"`
	StudentID   string `json:"studentId"`
	Status      string `json:"status"`
	ClockInTime string `json:"clockInTime"`
}

// AttendanceOpenApiController openapi 端控制器（只读）。
type AttendanceOpenApiController struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceOpenApiController 构造控制器。
func NewAttendanceOpenApiController(applicationService *application.AttendanceApplicationService) *AttendanceOpenApiController {
	return &AttendanceOpenApiController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/openapi/v1/attendance")）。
func (ctl *AttendanceOpenApiController) Register(r gin.IRouter) {
	g := r.Group("/openapi/v1/attendance")
	g.GET("", ctl.byStudent)
	g.GET("/:attendanceId", ctl.get)
}

// get GET /openapi/v1/attendance/:attendanceId
func (ctl *AttendanceOpenApiController) get(c *gin.Context) {
	view, err := ctl.applicationService.FindByID(c.Request.Context(), c.Param("attendanceId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// byStudent GET /openapi/v1/attendance?studentId=
func (ctl *AttendanceOpenApiController) byStudent(c *gin.Context) {
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
	responses := make([]AttendanceOpenApiResponse, 0, len(views))
	for _, v := range views {
		responses = append(responses, toResponse(v))
	}
	c.JSON(http.StatusOK, responses)
}

func toResponse(v application.AttendanceRecordView) AttendanceOpenApiResponse {
	return AttendanceOpenApiResponse{
		ID:          v.ID,
		StudentID:   v.StudentID,
		Status:      v.Status,
		ClockInTime: v.ClockInTime.Format(timeLayout),
	}
}
