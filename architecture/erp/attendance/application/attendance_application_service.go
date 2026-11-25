// Package application 是 attendance 域的应用层（对应 Java 版 AttendanceApplicationService）。
//
// 用例编排：跨域调用（system/grade 的 api 契约）+ 事务 + 领域规则 + 事件发布。
// 注意依赖方向：本包 import 的都是其他域的【api 契约包】与本域的 domain 层。
package application

import (
	"context"
	"strconv"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/api"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/model"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/repository"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/service"
	gradeapi "github.com/jamieleestudio/go-tutorials/architecture/erp/grade/api"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/messaging"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/persistence"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/shared"
	sysapi "github.com/jamieleestudio/go-tutorials/architecture/erp/system/api"
)

// AttendanceRecordView 应用层查询结果（与 api.AttendanceRecordDto 分离，
// 契约与本地用例互不依赖 —— 见 system 域同款注释）。
type AttendanceRecordView struct {
	ID          string
	StudentID   string
	Status      string
	ClockInTime time.Time
}

// ClockInCommand 应用层签到入参（与 api.ClockInCommand 分离；web/mobile/device
// 各端控制器构造它，provider 负责契约命令 → 应用命令的转换）。
type ClockInCommand struct {
	StudentID   string
	ClockInTime time.Time
	FaceToken   string
}

// AttendanceApplicationService 考勤域应用服务。
type AttendanceApplicationService struct {
	repo      repository.AttendanceRecordRepository
	facePort  service.FaceRecognitionPort
	systemApi sysapi.SystemApi           // 跨域：system 契约（纯提供方）
	gradeApi  gradeapi.GradeApi          // 跨域：grade 契约
	publisher messaging.MessagePublisher // 平台层消息端口
	txRunner  persistence.TxRunner       // 平台层事务传播（@Transactional 的 Go 对应物）
}

// NewAttendanceApplicationService 构造应用服务。
func NewAttendanceApplicationService(
	repo repository.AttendanceRecordRepository,
	facePort service.FaceRecognitionPort,
	systemApi sysapi.SystemApi,
	gradeApi gradeapi.GradeApi,
	publisher messaging.MessagePublisher,
	txRunner persistence.TxRunner,
) *AttendanceApplicationService {
	return &AttendanceApplicationService{
		repo: repo, facePort: facePort, systemApi: systemApi,
		gradeApi: gradeApi, publisher: publisher, txRunner: txRunner,
	}
}

// ClockIn 签到用例（对应 Java 版 @Transactional clockIn）：
// 打通租户 → 人脸校验（可选）→ 跨域查成绩 → 领域工厂签到 → 持久化 → 发布事件。
func (s *AttendanceApplicationService) ClockIn(ctx context.Context, command ClockInCommand) (AttendanceRecordView, error) {
	var view AttendanceRecordView
	err := s.txRunner.WithinTx(ctx, func(ctx context.Context) error {
		// 1. 跨域调用 system 契约（演示租户上下文打通；Java 版同样只调用不使用返回值）
		_ = s.systemApi.CurrentTenantID(ctx)

		// 2. 人脸校验：传了 faceToken 才校验（对应 Java 的 faceToken != null 判断）
		if command.FaceToken != "" && !s.facePort.Verify(command.StudentID, command.FaceToken) {
			return shared.NewBusinessRuleViolation("face verification failed for student " + command.StudentID)
		}

		// 3. 跨域调用 grade 契约（演示域间协作：签到时带上该学生的成绩数）
		grades, err := s.gradeApi.FindByStudentID(ctx, command.StudentID)
		if err != nil {
			return err
		}

		// 4. 领域工厂：签到规则（PRESENT/LATE）+ 领域事件在聚合内产生
		record, err := model.ClockIn(shared.NewID(), command.StudentID, command.ClockInTime, nil)
		if err != nil {
			return err
		}

		// 5. 持久化
		saved, err := s.repo.Save(ctx, record)
		if err != nil {
			return err
		}

		// 6. 发布集成事件（平台层消息端口；payload 与 Java 版一致 "recordId:gradeCount"）
		s.publisher.Publish(messaging.Of(api.TopicClocked, saved.ID()+":"+strconv.Itoa(len(grades))))

		view = toView(saved)
		return nil
	})
	if err != nil {
		return AttendanceRecordView{}, err
	}
	return view, nil
}

// FindByID 按主键查询（对应 Java 版 @Transactional(readOnly=true) findById）。
func (s *AttendanceApplicationService) FindByID(ctx context.Context, id string) (AttendanceRecordView, error) {
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AttendanceRecordView{}, err
	}
	if record == nil {
		return AttendanceRecordView{}, shared.NewEntityNotFound("AttendanceRecord not found: " + id)
	}
	return toView(record), nil
}

// FindByStudentID 查询某学生的全部考勤记录。
func (s *AttendanceApplicationService) FindByStudentID(ctx context.Context, studentID string) ([]AttendanceRecordView, error) {
	records, err := s.repo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	views := make([]AttendanceRecordView, 0, len(records))
	for _, r := range records {
		views = append(views, toView(r))
	}
	return views, nil
}

func toView(r *model.AttendanceRecord) AttendanceRecordView {
	return AttendanceRecordView{
		ID:          r.ID(),
		StudentID:   r.StudentID(),
		Status:      string(r.Status()),
		ClockInTime: r.ClockInTime(),
	}
}
