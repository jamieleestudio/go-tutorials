// Package job 是 attendance 域的 job 端（定时任务）。
// Java 版用 @Scheduled(cron = "0 0 2 * * *")；Go 没有调度容器，
// 用 goroutine + timer 实现"每天 02:00 触发"（演示用例：跑一次每日检查查询）。
package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/application"
)

// AttendanceDailyJob 每日考勤检查任务（对应 Java 版 AttendanceDailyJob）。
type AttendanceDailyJob struct {
	applicationService *application.AttendanceApplicationService
}

// NewAttendanceDailyJob 构造任务。
func NewAttendanceDailyJob(applicationService *application.AttendanceApplicationService) *AttendanceDailyJob {
	return &AttendanceDailyJob{applicationService: applicationService}
}

// Start 启动后台调度（对应 @EnableScheduling）。
// 只在 boot 部署形态的组合根里调用 —— 集成测试不启动调度（与 Java @Scheduled 在
// 测试上下文中被排除的行为一致）。ctx 取消即停止。
func (j *AttendanceDailyJob) Start(ctx context.Context) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
			if !next.After(now) {
				next = next.Add(24 * time.Hour) // 已过今天 02:00 → 明天
			}
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if err := j.Run(ctx); err != nil {
					slog.Error("attendance daily job failed", "error", err)
				}
			}
		}
	}()
	slog.Info("attendance daily job scheduled", "runAt", "02:00 daily")
}

// Run 执行一次（对应 Java 的 run() 方法，公开以便手动触发/测试）。
func (j *AttendanceDailyJob) Run(ctx context.Context) error {
	_, err := j.applicationService.FindByStudentID(ctx, "daily-check")
	return err
}
