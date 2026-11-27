// Outbox 定时任务（relay）：轮询认领 → 逐条消费 → 状态流转。
// 逐行对照 Java OutboxRelay + OutboxRelayProperties + OutboxMessageRepository。
//
// 认领与消费在同一个事务中完成（Java 注释原话的教学点）：
//   - 认领使用 FOR UPDATE SKIP LOCKED 保证多实例并发下不重复消费：
//     各自锁住不重叠的行，跳过已被其他事务锁定的行，既不互相阻塞也不重复认领；
//   - 行锁持有到事务提交，因此消费动作（本地业务执行）与状态标记是原子的；
//   - 已知局限：消费在认领事务内部，行锁持有到 commit —— 慢消费会拉长事务（教学版接受）。
//
// 语义为至少一次（at-least-once）：事务提交前宕机，已消费但未提交的消息仍是
// PENDING，会被再次捞取，因此 Publisher 实现必须幂等。
package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// Config relay 配置项（对应 Java OutboxRelayProperties，默认值即 application.yml 的兜底值）。
type Config struct {
	// PollInterval 轮询间隔（对应 poll-interval: 1000，@Scheduled fixedDelay）。
	PollInterval time.Duration
	// BatchSize 每批最多认领的消息数（对应 batch-size: 50）。
	BatchSize int
	// MaxRetries 最大重试次数，达到后消息转 FAILED（死信）（对应 max-retries: 5）。
	MaxRetries int
	// InitialBackoff 首次失败的退避时间，之后按 BackoffMultiplier 指数增长（对应 initial-backoff: 5s）。
	InitialBackoff time.Duration
	// BackoffMultiplier 退避倍数（对应 backoff-multiplier: 2.0）。
	BackoffMultiplier float64
}

// WithDefaults 零值字段回落到 Java @ConfigurationProperties 的默认值。
func (c Config) WithDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 5
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = 5 * time.Second
	}
	if c.BackoffMultiplier <= 0 {
		c.BackoffMultiplier = 2.0
	}
	return c
}

// Relay Outbox 定时任务（对应 Java OutboxRelay）。
type Relay struct {
	db        *sql.DB
	publisher Publisher
	cfg       Config
}

// New 建 relay（cfg 传零值即全默认 —— 对应 Java 的 properties 占位符兜底）。
func New(db *sql.DB, publisher Publisher, cfg Config) *Relay {
	return &Relay{db: db, publisher: publisher, cfg: cfg.WithDefaults()}
}

// RelayOnce 执行一轮消费：认领一批到期消息并逐条处理（对应 Java relayOnce）。
//
// 返回本轮实际处理（含转入 FAILED）的消息条数。整个方法体是一个事务：
// claimBatch → handle 逐条 → commit；任何 DB 错误都回滚整批（本轮白做，下轮重来）。
func (r *Relay) RelayOnce(ctx context.Context) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin claim tx: %w", err)
	}

	// Java 在事务开头取一次 now：认领口径统一，避免批内时间漂移
	now := time.Now().UTC()
	batch, err := claimBatch(ctx, tx, now, r.cfg.BatchSize)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("claim batch: %w", err)
	}
	if len(batch) == 0 {
		_ = tx.Rollback() // 空批：无状态变更，收掉事务即可
		return 0, nil
	}
	slog.Info("Outbox relay 认领消息", "count", len(batch))

	for _, msg := range batch {
		if err := r.handle(ctx, tx, msg); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("handle message %d: %w", msg.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit claim tx: %w", err)
	}
	return len(batch), nil
}

// claimBatch 认领一批到期（PENDING 且 next_retry_at 已到）的消息 —— 本模块的灵魂 SQL，
// 对应 Java OutboxMessageRepository.claimBatch：
//
//	SELECT * FROM outbox_messages
//	WHERE status = 'PENDING' AND next_retry_at <= :now
//	ORDER BY id
//	LIMIT :limit
//	FOR UPDATE SKIP LOCKED
//
// FOR UPDATE SKIP LOCKED（MySQL 8.0+）：被其他 relay 事务锁住的行直接跳过而不是等待，
// 多实例并发扫描各自拿到不重叠的子集。必须在事务内调用，行锁持续到事务提交。
func claimBatch(ctx context.Context, tx *sql.Tx, now time.Time, limit int) ([]*Message, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+messageColumns+` FROM outbox_messages
WHERE status = 'PENDING' AND next_retry_at <= ?
ORDER BY id
LIMIT ?
FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim query: %w", err)
	}
	defer rows.Close()

	var batch []*Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		batch = append(batch, m)
	}
	return batch, rows.Err()
}

// handle 单条消费：成功 → PUBLISHED；失败 → 重试计数、退避或死信（对应 Java handle）。
// 消费错误在这里被吞掉转为状态迁移（业务失败不是事务失败 —— Java try/catch 的语义）；
// 只有 DB 错误才向外抛、回滚整批。
func (r *Relay) handle(ctx context.Context, tx *sql.Tx, msg *Message) error {
	err := r.publisher.Publish(ctx, msg)
	now := time.Now().UTC()

	var out outcome
	if err == nil {
		out = resolveSuccess(now)
		slog.Info("Outbox relay 消费成功",
			"id", msg.ID, "type", msg.EventType,
			"aggregate", msg.AggregateType+"/"+msg.AggregateID,
			"retryCount", msg.RetryCount)
	} else {
		out = resolveFailure(r.cfg, msg, now)
		if out.status == StatusFailed {
			slog.Error("Outbox relay 重试超限，转 FAILED 死信",
				"id", msg.ID, "type", msg.EventType,
				"aggregate", msg.AggregateType+"/"+msg.AggregateID,
				"reason", err)
		} else {
			slog.Warn("Outbox relay 消费失败，第 N 次重试将于 X 后进行",
				"retryCount", out.retryCount,
				"backoff", out.backoff,
				"id", msg.ID,
				"reason", err)
		}
	}
	return applyOutcome(ctx, tx, msg, out)
}

// outcome 消费后的状态决策（纯数据 + 纯函数拆分自 Java handle 的 try/catch，不连库可单测）。
type outcome struct {
	status      Status
	retryCount  int
	nextRetryAt time.Time     // status==PENDING 时有效
	publishedAt *time.Time    // status==PUBLISHED 时有效
	backoff     time.Duration // 本次失败安排的退避时长（日志用，0 = 无）
}

// resolveSuccess 消费成功：PUBLISHED + publishedAt。
func resolveSuccess(now time.Time) outcome {
	return outcome{status: StatusPublished, publishedAt: &now}
}

// resolveFailure 消费失败的状态机（对应 Java handle 的 catch 分支）：
// retryCount+1；达到 MaxRetries → FAILED 死信；否则 PENDING + 指数退避的 nextRetryAt。
func resolveFailure(cfg Config, msg *Message, now time.Time) outcome {
	retryCount := msg.RetryCount + 1
	if retryCount >= cfg.MaxRetries {
		return outcome{status: StatusFailed, retryCount: retryCount}
	}
	backoff := NextBackoff(cfg, retryCount)
	return outcome{
		status:      StatusPending,
		retryCount:  retryCount,
		nextRetryAt: now.Add(backoff),
		backoff:     backoff,
	}
}

// NextBackoff 指数退避：initialBackoff * multiplier^(retryCount-1)（Java 公式照抄）。
// retryCount 从 1 计：首次失败 retryCount=1 → backoff = initialBackoff。
func NextBackoff(cfg Config, retryCount int) time.Duration {
	factor := math.Pow(cfg.BackoffMultiplier, float64(retryCount-1))
	return time.Duration(int64(float64(cfg.InitialBackoff) * factor))
}

// applyOutcome 把状态决策落库（同一事务内 UPDATE，行锁持续到 commit）并回写内存实体。
func applyOutcome(ctx context.Context, tx *sql.Tx, msg *Message, out outcome) error {
	var err error
	switch out.status {
	case StatusPublished:
		_, err = tx.ExecContext(ctx,
			`UPDATE outbox_messages SET status = ?, published_at = ? WHERE id = ?`,
			out.status, out.publishedAt, msg.ID)
	case StatusFailed:
		_, err = tx.ExecContext(ctx,
			`UPDATE outbox_messages SET status = ?, retry_count = ? WHERE id = ?`,
			out.status, out.retryCount, msg.ID)
	case StatusPending:
		_, err = tx.ExecContext(ctx,
			`UPDATE outbox_messages SET status = ?, retry_count = ?, next_retry_at = ? WHERE id = ?`,
			out.status, out.retryCount, out.nextRetryAt, msg.ID)
	}
	if err != nil {
		return fmt.Errorf("update message %d to %s: %w", msg.ID, out.status, err)
	}
	msg.Status = out.status
	msg.RetryCount = out.retryCount
	if out.status == StatusPublished {
		msg.PublishedAt = out.publishedAt
	} else {
		msg.NextRetryAt = out.nextRetryAt
	}
	return nil
}

// Run 定时轮询主循环（对应 Java @Scheduled(fixedDelayString = "${...poll-interval:1s}")）：
// 每 PollInterval 执行一轮 relayOnce，ctx 取消退出。生产部署时就是这一个 goroutine
// —— 多副本部署天然形成多实例并发认领（SKIP LOCKED 保证不重复）。
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.RelayOnce(ctx); err != nil {
				slog.Warn("Outbox relay 一轮失败，下个周期重试", "error", err)
			}
		}
	}
}
