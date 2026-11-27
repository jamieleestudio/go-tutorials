// Command demo 对应 Java OutboxDemoApplication + OutboxDemoRunner：
// 三个场景串演 outbox 的完整语义。
//
//	场景一：订单与事件同事务提交，定时任务捞取后本地处理器执行业务 → PUBLISHED
//	场景二：业务中途异常，同事务回滚，事件不会泄漏到表中（不多）
//	场景三：处理器前两次失败，观察重试与指数退避，最终成功（不丢、最终一致）
//
// 运行前先起 MySQL：docker compose -f distributed-infra/docker-compose.yml up -d
//
//	go run ./outbox/demo
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/jamieleestudio/go-tutorials/distributed-infra/outbox"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	ctx := context.Background()
	serverDSN := envOr("OUTBOX_MYSQL_DSN", "root:rootpw@tcp(127.0.0.1:33061)/?parseTime=true&loc=UTC&timeout=3s")

	db, err := outbox.Connect(ctx, serverDSN)
	if err != nil {
		slog.Error("MySQL 不可达，请先 docker compose -f distributed-infra/docker-compose.yml up -d", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 清场（对应 Java deleteAllInBatch ×2）
	if _, err := db.ExecContext(ctx, "DELETE FROM outbox_messages"); err != nil {
		slog.Error("clean outbox_messages", "error", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM demo_orders"); err != nil {
		slog.Error("clean demo_orders", "error", err)
		os.Exit(1)
	}
	slog.Info("========== Outbox Demo 开始 ==========")

	// 定时任务后台轮询（对应 Java @Scheduled；默认配置 poll 1s / batch 50 / 退避 5s×2.0）
	relay := outbox.New(db, outbox.NewDemoHandler(slog.Default()), outbox.Config{})
	go relay.Run(ctx)

	scenario1SameTransactionPublish(ctx, db)
	scenario2RollbackLeavesNothing(ctx, db)
	scenario3RetryWithBackoff(ctx, db)

	printSummary(ctx, db)
}

// scenario1SameTransactionPublish 场景一：业务行与事件行在【同一个本地事务】里落库
// —— outbox 模式的立身之本：业务成功则事件必然存在。
func scenario1SameTransactionPublish(ctx context.Context, db *sql.DB) {
	slog.Info("---- 场景一：业务与事件同事务落库，定时任务捞取执行 ----")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("begin tx", "error", err)
		return
	}
	if err := outbox.SaveOrderTx(ctx, tx, outbox.NewOrder("ORDER-OK-1")); err != nil {
		slog.Error("save order", "error", err)
		_ = tx.Rollback()
		return
	}
	if err := outbox.SaveMessageTx(ctx, tx, outbox.Pending("ORDER", "ORDER-OK-1", "ORDER_CREATED",
		`{"orderNo":"ORDER-OK-1","amount":99.9}`)); err != nil {
		slog.Error("save message", "error", err)
		_ = tx.Rollback()
		return
	}
	if err := tx.Commit(); err != nil {
		slog.Error("commit", "error", err)
		return
	}
	slog.Info("订单 + outbox 事件已在同一事务提交，等待定时任务捞取...")
	awaitStatus(ctx, db, "ORDER-OK-1", outbox.StatusPublished, 10*time.Second)
}

// scenario2RollbackLeavesNothing 场景二：业务中途异常 → 整个事务回滚 → 事件不多不少。
// Java 版靠抛 IllegalStateException 触发回滚；Go 用显式错误路径 + defer Rollback 兜底，
// 教学点相同：业务失败路径绝不提交。
func scenario2RollbackLeavesNothing(ctx context.Context, db *sql.DB) {
	slog.Info("---- 场景二：业务异常导致同事务回滚，事件不多不少 ----")
	before, _ := outbox.CountMessages(ctx, db)

	err := func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback() // 任何失败路径都兜底回滚
		if err := outbox.SaveOrderTx(ctx, tx, outbox.NewOrder("ORDER-FAIL-1")); err != nil {
			return err
		}
		if err := outbox.SaveMessageTx(ctx, tx, outbox.Pending("ORDER", "ORDER-FAIL-1", "ORDER_CREATED",
			`{"orderNo":"ORDER-FAIL-1","amount":1.0}`)); err != nil {
			return err
		}
		return errors.New("模拟业务异常，整个事务回滚") // ← Java 的 throw IllegalStateException
	}()
	if err != nil {
		slog.Info("捕获业务异常", "reason", err)
	}

	after, _ := outbox.CountMessages(ctx, db)
	verdict := "异常！事件泄漏了"
	if before == after {
		verdict = "无事件泄漏 ✔"
	}
	slog.Info("回滚前后 outbox 总行数", "before", before, "after", after, "verdict", verdict)
}

// scenario3RetryWithBackoff 场景三：处理器前两次失败，观察「第 N 次重试将于 Xs 后进行」
// 的指数退避日志，第三次成功 —— 不丢、最终一致。
func scenario3RetryWithBackoff(ctx context.Context, db *sql.DB) {
	slog.Info("---- 场景三：消费失败重试（指数退避），最终成功 ----")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("begin tx", "error", err)
		return
	}
	defer tx.Rollback()
	if err := outbox.SaveOrderTx(ctx, tx, outbox.NewOrder("ORDER-RETRY-DEMO")); err != nil {
		slog.Error("save order", "error", err)
		return
	}
	if err := outbox.SaveMessageTx(ctx, tx, outbox.Pending("ORDER", "ORDER-RETRY-DEMO", "ORDER_CREATED",
		`{"orderNo":"ORDER-RETRY-DEMO","amount":66.6}`)); err != nil {
		slog.Error("save message", "error", err)
		return
	}
	if err := tx.Commit(); err != nil {
		slog.Error("commit", "error", err)
		return
	}
	slog.Info("事件已提交，处理器前两次会失败，观察重试日志...")
	awaitStatus(ctx, db, "ORDER-RETRY-DEMO", outbox.StatusPublished, 30*time.Second)
}

// awaitStatus 轮询等待消息到达期望状态（对应 Java awaitStatus，300ms 步进）。
func awaitStatus(ctx context.Context, db *sql.DB, aggregateID string, want outbox.Status, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m, err := outbox.FindMessageByAggregate(ctx, db, aggregateID)
		if err == nil && m.Status == want {
			slog.Info("已到达期望状态",
				"aggregate", aggregateID,
				"status", m.Status,
				"retryCount", m.RetryCount)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
	}
	slog.Warn("等待超时，未在期限内到达状态", "aggregate", aggregateID, "want", want, "timeout", timeout)
}

// printSummary 结果汇总（对应 Java printSummary）。
func printSummary(ctx context.Context, db *sql.DB) {
	slog.Info("========== Demo 结果汇总 ==========")
	messages, err := outbox.ListMessages(ctx, db)
	if err != nil {
		slog.Warn("list messages", "error", err)
		return
	}
	for _, m := range messages {
		slog.Info("outbox",
			"id", m.ID,
			"aggregate", m.AggregateID,
			"type", m.EventType,
			"status", m.Status,
			"retryCount", m.RetryCount,
			"nextRetryAt", m.NextRetryAt.Format(time.RFC3339))
	}
	orders, err := outbox.ListOrders(ctx, db)
	if err != nil {
		slog.Warn("list orders", "error", err)
		return
	}
	for _, o := range orders {
		slog.Info("order", "id", o.ID, "orderNo", o.OrderNo, "status", o.Status)
	}
}
