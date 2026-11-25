// 集成测试：完整 erp 用例（对应 Java 版 erp-app-boot 的 ErpIntegrationTest）。
//
// Java 版用 @SpringBootTest + MockMvc + H2 内存库；Go 版用组合根 assemble.New
// 装配真实应用 + SQLite 内存库 + httptest 走完整 HTTP 层 —— 零外部依赖。
package assemble_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/app/assemble"
)

// newTestApp 装配真实应用（每个测试独立命名内存库），并清空数据保证隔离。
func newTestApp(t *testing.T, ends assemble.EndSet) *assemble.App {
	t.Helper()
	cfg := assemble.Config{
		Port: "0", // 集成测试不起 HTTP 服务，用 httptest 直接打 Engine
		DSN: fmt.Sprintf("sqlite://file:erp_%s?mode=memory&cache=shared",
			strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")),
		GormLogLevel: "silent",
		Ends:         ends,
	}
	app, err := assemble.New(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := app.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	for _, table := range []string{"sys_user", "grade", "attendance_record"} {
		require.NoError(t, app.DB.Exec("DELETE FROM "+table).Error)
	}
	return app
}

// doJSON 走完整 HTTP 中间件栈的请求辅助。
func doJSON(t *testing.T, app *assemble.App, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.Engine.ServeHTTP(rec, req)
	return rec
}

// --- 用例 1：成绩创建与查询（Java: grade_can_be_created_and_queried） ---

func TestGradeCanBeCreatedAndQueried(t *testing.T) {
	app := newTestApp(t, assemble.BootEnds)

	rec := doJSON(t, app, "POST", "/api/v1/grades",
		map[string]any{"studentId": "s-1", "courseName": "math", "score": 88.0})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	var body struct {
		ID        string  `json:"id"`
		StudentID string  `json:"studentId"`
		Score     float64 `json:"score"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.NotEmpty(t, body.ID)
	assert.Equal(t, "s-1", body.StudentID)
	assert.InDelta(t, 88.0, body.Score, 0.0001)
}

// --- 用例 2：签到走跨域 api（Java: attendance_clock_in_uses_cross_domain_api） ---

func TestAttendanceClockInUsesCrossDomainApi(t *testing.T) {
	app := newTestApp(t, assemble.BootEnds)

	// 先给 s-2 建一条成绩（签到应用服务会跨域查 grade 契约）
	rec := doJSON(t, app, "POST", "/api/v1/grades",
		map[string]any{"studentId": "s-2", "courseName": "math", "score": 70.0})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	rec = doJSON(t, app, "POST", "/api/v1/attendance/clock-in",
		map[string]any{"studentId": "s-2", "clockInTime": "2026-09-16T08:00:00"})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	var clockIn struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &clockIn))
	assert.Equal(t, "PRESENT", clockIn.Status) // 没传 scheduleStart → PRESENT

	// 事件已发布：payload 格式 "recordId:gradeCount"（该学生 1 条成绩）
	assert.Equal(t, clockIn.ID+":1", app.AttendanceConsumer.LastPayload())

	// 再按 ID 查询
	rec = doJSON(t, app, "GET", "/api/v1/attendance/"+clockIn.ID, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var fetched struct {
		ID        string `json:"id"`
		StudentID string `json:"studentId"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetched))
	assert.Equal(t, "s-2", fetched.StudentID)
}

// --- 用例 3：404 返回 Result 格式（Java: attendance_not_found_returns_404） ---

func TestAttendanceNotFoundReturns404(t *testing.T) {
	app := newTestApp(t, assemble.BootEnds)

	rec := doJSON(t, app, "GET", "/api/v1/attendance/missing", nil)
	require.Equal(t, 404, rec.Code)

	var result struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.False(t, result.Success)
	assert.Equal(t, "404", result.Code)
	assert.Contains(t, result.Message, "AttendanceRecord not found")
}

// --- 用例 4：openapi 端在 boot 装配形态下暴露（Java: openapi_end_is_exposed_on_the_boot_assembly） ---

func TestOpenapiEndIsExposedOnTheBootAssembly(t *testing.T) {
	app := newTestApp(t, assemble.BootEnds)

	rec := doJSON(t, app, "GET", "/openapi/v1/attendance?studentId=s-3", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.JSONEq(t, "[]", rec.Body.String()) // 无记录 → 空数组
}

// --- 附加用例：端集合决定暴露的路由（Java 版 excludeFilters 的运行时效果） ---

func TestEndSetControlsExposedRoutes(t *testing.T) {
	app := newTestApp(t, assemble.MobileEnds)

	// mobile 端可用
	rec := doJSON(t, app, "POST", "/api/v1/mobile/attendance/clock-in",
		map[string]any{"studentId": "s-9", "clockInTime": "2026-09-16T08:00:00"})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// web / openapi / internal / device 端未装配 → 404
	assert.Equal(t, 404, doJSON(t, app, "GET", "/api/v1/attendance/whatever", nil).Code)
	assert.Equal(t, 404, doJSON(t, app, "GET", "/openapi/v1/attendance?studentId=s-9", nil).Code)
	assert.Equal(t, 404, doJSON(t, app, "GET", "/internal/v1/attendance/whatever", nil).Code)
	assert.Equal(t, 404, doJSON(t, app, "POST", "/openapi/v1/device/attendance/clock",
		map[string]any{"studentId": "s-9", "clockInTime": "2026-09-16T08:00:00"}).Code)
}

// --- 附加用例：签到迟到规则 + 人脸校验失败 → 422（业务规则错误码） ---

func TestLateClockInAndFaceVerification(t *testing.T) {
	app := newTestApp(t, assemble.BootEnds)

	// 未配置外部系统 → faceToken 校验必然失败 → 422 业务规则错误
	//（faceToken 是 device 端的字段 —— web 端请求模型里没有它，与 Java 版一致）
	rec := doJSON(t, app, "POST", "/openapi/v1/device/attendance/clock",
		map[string]any{"studentId": "s-4", "clockInTime": "2026-09-16T08:00:00", "faceToken": "face-abc"})
	require.Equal(t, 422, rec.Code, rec.Body.String())
	var fail struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fail))
	assert.False(t, fail.Success)
	assert.Equal(t, "422", fail.Code)
	assert.Contains(t, fail.Message, "face verification failed")

	// 不带 faceToken → 正常签到（grade 校验失败也会 422：score 0~100 之外）
	rec = doJSON(t, app, "POST", "/api/v1/grades",
		map[string]any{"studentId": "s-4", "courseName": "math", "score": 150.0})
	require.Equal(t, 422, rec.Code, rec.Body.String())
}
