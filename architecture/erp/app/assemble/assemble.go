// Package assemble 是 erp 的组合根（Composition Root）。
//
// 对应 Java 版 4 个 app 模块的装配方式：boot/admin/mobile/openapi 都是
// @SpringBootApplication(scanBasePackages = "com.example.erp")，但用
// @ComponentScan(excludeFilters = REGEX 正则) 排除不需要的端。
// Go 没有 classpath 扫描 —— 端装配在这里是【显式的】：EndSet 里不写 true，
// 就不注册那个端的控制器。"不写那行注册代码" 就是 Go 版的 excludeFilters。
//
// 装配顺序即依赖顺序：db → system → grade → attendance → HTTP 端。
// 域间注入的都是对方的【api 契约实现】（interfaces/provider）。
package assemble

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	attapi "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/api"
	attapp "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
	attclient "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/infrastructure/client"
	attpersistence "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/infrastructure/persistence"
	attadmin "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/admin"
	attdevice "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/device"
	attinternal "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/internalapi"
	attjob "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/job"
	attmobile "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/mobile"
	attmq "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/mq"
	attopenapi "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/openapi"
	attp "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/provider"
	attweb "github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/interfaces/web"
	gradeapi "github.com/jamieleestudio/go-tutorials/architecture/erp/grade/api"
	gradeapp "github.com/jamieleestudio/go-tutorials/architecture/erp/grade/application"
	gradepersistence "github.com/jamieleestudio/go-tutorials/architecture/erp/grade/infrastructure/persistence"
	gradep "github.com/jamieleestudio/go-tutorials/architecture/erp/grade/interfaces/provider"
	gradeweb "github.com/jamieleestudio/go-tutorials/architecture/erp/grade/interfaces/web"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/integration"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/messaging"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/security"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/web"
	sysapi "github.com/jamieleestudio/go-tutorials/architecture/erp/system/api"
	sysapp "github.com/jamieleestudio/go-tutorials/architecture/erp/system/application"
	syspersistence "github.com/jamieleestudio/go-tutorials/architecture/erp/system/infrastructure/persistence"
	sysp "github.com/jamieleestudio/go-tutorials/architecture/erp/system/interfaces/provider"
)

// EndSet 端集合：本次部署暴露哪些端（对应 Java 各 app 的 excludeFilters 正则）。
type EndSet struct {
	Web      bool
	Admin    bool
	Mobile   bool
	OpenAPI  bool
	Internal bool
	Device   bool
}

// 四种部署形态（Java 端集合的反向表达：exclude 排掉的，这里就是 false）。
var (
	// BootEnds 全部端 + mq + job（对应 erp-app-boot）
	BootEnds = EndSet{Web: true, Admin: true, Mobile: true, OpenAPI: true, Internal: true, Device: true}
	// AdminEnds web + admin（对应 erp-app-admin 排除 mobile|openapi|internal|device|mq|job）
	AdminEnds = EndSet{Web: true, Admin: true}
	// MobileEnds 仅 mobile（对应 erp-app-mobile）
	MobileEnds = EndSet{Mobile: true}
	// OpenAPIEnds openapi + device（对应 erp-app-openapi）
	OpenAPIEnds = EndSet{OpenAPI: true, Device: true}
)

// Config 装配配置（对应各 app 的 application.yml）。
type Config struct {
	Port                  string // HTTP 监听端口
	DSN                   string // 数据库连接串（sqlite:// 前缀走 SQLite，否则按 MySQL）
	GormLogLevel          string
	SecurityEnabled       bool   // 对应 erp.security.enabled
	ExternalSystemBaseURL string // 外部系统地址（空 = 未配置，人脸校验走降级）
	Ends                  EndSet // 端集合
}

// LoadConfig 从环境变量读取（默认值对齐 Java 版 boot 的 application.yml）。
func LoadConfig(ends EndSet, defaultPort string) Config {
	return Config{
		Port:                  getenv("APP_PORT", defaultPort),
		DSN:                   getenv("DB_DSN", "sqlite://file:erp?mode=memory&cache=shared"),
		GormLogLevel:          getenv("GORM_LOG_LEVEL", "warn"),
		SecurityEnabled:       getenv("SECURITY_ENABLED", "false") == "true",
		ExternalSystemBaseURL: getenv("EXTERNAL_SYSTEM_URL", ""),
		Ends:                  ends,
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// App 装配完成的应用。
type App struct {
	DB     *gorm.DB
	Engine *gin.Engine

	// 各域契约的"出口"：后续新域跨域调用、或 P4 把 provider 换成 gRPC 客户端时，
	// 都从组合根拿这里的实现（对应 Java 版容器里的 @Component provider bean）。
	SystemAPI     sysapi.SystemApi
	GradeAPI      gradeapi.GradeApi
	AttendanceAPI attapi.AttendanceApi

	TokenService       *security.TokenService
	AttendanceConsumer *attmq.AttendanceClockConsumer // mq 端消费者（测试可观察 lastPayload）
	attendanceJob      *attjob.AttendanceDailyJob
}

// New 装配整个应用（等价于 Java 应用启动时 Spring 容器做的所有事）。
func New(ctx context.Context, cfg Config) (*App, error) {
	// 1. 数据库 + 建表（三个域的实体 —— 组合根特权：只有它 import 各域的 infrastructure）
	db, err := persistence.Open(cfg.DSN, cfg.GormLogLevel)
	if err != nil {
		return nil, err
	}
	if err := persistence.Migrate(db,
		&syspersistence.SysUserEntity{},
		&gradepersistence.GradeEntity{},
		&attpersistence.AttendanceRecordEntity{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	txRunner := persistence.NewGormTxRunner(db)

	// 2. system 域（纯提供方）：仓储 → 应用服务 → 契约实现
	sysRepo := syspersistence.NewSysUserRepository(db)
	sysService := sysapp.NewSystemApplicationService(sysRepo)
	var systemAPI sysapi.SystemApi = sysp.NewSystemApiProvider(sysService)

	// 3. grade 域：仓储 → 应用服务（跨域注入 system 契约）→ 契约实现
	gradeRepo := gradepersistence.NewGradeRepository(db)
	gradeService := gradeapp.NewGradeApplicationService(gradeRepo, systemAPI)
	var gradeAPI gradeapi.GradeApi = gradep.NewGradeApiProvider(gradeService)

	// 4. mq 端消费者先行创建 —— publisher 装配时要收集消费者
	//   （Java 版靠构造器注入 List<MessageConsumer> 自动收集）
	attendanceConsumer := attmq.NewAttendanceClockConsumer()
	publisher := messaging.NewInMemoryMessagePublisher([]messaging.MessageConsumer{attendanceConsumer})

	// 5. attendance 域：基础设施适配器 + 仓储 → 应用服务（跨域注入 system/grade 契约）→ 契约实现
	externalClient := integration.NewHttpExternalSystemClient(cfg.ExternalSystemBaseURL)
	facePort := attclient.NewFaceRecognitionClient(externalClient)
	attRepo := attpersistence.NewAttendanceRecordRepository(db)
	attService := attapp.NewAttendanceApplicationService(attRepo, facePort, systemAPI, gradeAPI, publisher, txRunner)
	var attendanceAPI attapi.AttendanceApi = attp.NewAttendanceApiProvider(attService)

	// 6. HTTP 引擎 + 平台中间件
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), web.ErrorHandler(), security.ApiAuthFilter())
	if cfg.SecurityEnabled {
		engine.Use(security.AuthMiddleware()) // 对应 WebMvcConfig 按 security.enabled 注册拦截器
	}

	// 7. 按端集合注册控制器 —— Go 版的 excludeFilters：false 就不注册
	if cfg.Ends.Web {
		gradeweb.NewGradeController(gradeService).Register(engine)
		attweb.NewAttendanceWebController(attService).Register(engine)
	}
	if cfg.Ends.Admin {
		attadmin.NewAttendanceAdminController(attService).Register(engine)
	}
	if cfg.Ends.Mobile {
		attmobile.NewAttendanceMobileController(attService).Register(engine)
	}
	if cfg.Ends.OpenAPI {
		attopenapi.NewAttendanceOpenApiController(attService).Register(engine)
	}
	if cfg.Ends.Internal {
		attinternal.NewAttendanceInternalController(attService).Register(engine)
	}
	if cfg.Ends.Device {
		attdevice.NewAttendanceDeviceController(attService).Register(engine)
	}

	return &App{
		DB:                 db,
		Engine:             engine,
		SystemAPI:          systemAPI,
		GradeAPI:           gradeAPI,
		AttendanceAPI:      attendanceAPI,
		TokenService:       security.NewTokenService(),
		AttendanceConsumer: attendanceConsumer,
		attendanceJob:      attjob.NewAttendanceDailyJob(attService),
	}, nil
}

// StartAttendanceDailyJob 启动每日考勤任务（仅 boot 部署形态调用；
// ctx 取消时任务停止 —— 集成测试不调用它）。
func (a *App) StartAttendanceDailyJob(ctx context.Context) {
	a.attendanceJob.Start(ctx)
}
