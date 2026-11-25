// Package web 是 grade 域的 web 端接口（对应 Java 版 interfaces.web.GradeController）。
//
// 依赖规则：本包只允许注入【应用层】—— 不碰 domain/infrastructure/api（架构护栏）。
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/grade/application"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/kernel"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/web"
)

// CreateGradeWebRequest 创建成绩请求（对应 Java 版 interfaces.web.dto.CreateGradeWebRequest）。
type CreateGradeWebRequest struct {
	StudentID  string  `json:"studentId"`
	CourseName string  `json:"courseName"`
	Score      float64 `json:"score"`
}

// GradeResponse 成绩响应（对应 Java 版 GradeResponse）。
type GradeResponse struct {
	ID         string  `json:"id"`
	StudentID  string  `json:"studentId"`
	CourseName string  `json:"courseName"`
	Score      float64 `json:"score"`
}

// GradeController 成绩 web 控制器。
type GradeController struct {
	applicationService *application.GradeApplicationService
}

// NewGradeController 构造控制器（注入应用服务）。
func NewGradeController(applicationService *application.GradeApplicationService) *GradeController {
	return &GradeController{applicationService: applicationService}
}

// Register 注册路由（对应 @RequestMapping("/api/v1/grades")）。
func (ctl *GradeController) Register(r gin.IRouter) {
	g := r.Group("/api/v1/grades")
	g.POST("", ctl.create)
	g.GET("", ctl.byStudent)
	g.GET("/:gradeId", ctl.get)
}

// create POST /api/v1/grades
func (ctl *GradeController) create(c *gin.Context) {
	var req CreateGradeWebRequest
	if !web.BindJSON(c, &req) {
		return
	}
	view, err := ctl.applicationService.Create(c.Request.Context(), req.StudentID, req.CourseName, req.Score)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// get GET /api/v1/grades/:gradeId
func (ctl *GradeController) get(c *gin.Context) {
	view, err := ctl.applicationService.FindByID(c.Request.Context(), c.Param("gradeId"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, toResponse(view))
}

// byStudent GET /api/v1/grades?studentId=
func (ctl *GradeController) byStudent(c *gin.Context) {
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
	responses := make([]GradeResponse, 0, len(views))
	for _, v := range views {
		responses = append(responses, toResponse(v))
	}
	c.JSON(http.StatusOK, responses)
}

func toResponse(v application.GradeView) GradeResponse {
	return GradeResponse{ID: v.ID, StudentID: v.StudentID, CourseName: v.CourseName, Score: v.Score}
}
