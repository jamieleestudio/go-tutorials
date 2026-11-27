package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestDB 探测 MySQL 并返回绑定 outbox_demo 的连接。
// 不可达自动跳过 —— 对应 Java 测试 requireMysql 的 assumeTrue
// （FOR UPDATE SKIP LOCKED 是 MySQL 8.0+ 语义，SQLite 没有等价物，必须真库验证）。
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	serverDSN := envOr("OUTBOX_MYSQL_DSN", "root:rootpw@tcp(127.0.0.1:33061)/?parseTime=true&loc=UTC&timeout=3s")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Connect(ctx, serverDSN)
	if err != nil {
		t.Skipf("MySQL 不可达，请先 docker compose -f distributed-infra/docker-compose.yml up -d：%v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// testRelayConfig 对应 Java @SpringBootTest properties：
// batch-size=10 / max-retries=3 / initial-backoff=100ms / backoff-multiplier=2.0。
func testRelayConfig() Config {
	return Config{
		BatchSize:         10,
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
}

// cleanTables 对应 Java @BeforeEach 的 deleteAllInBatch ×2（用例间互不污染）。
func cleanTables(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "DELETE FROM outbox_messages"); err != nil {
		t.Fatalf("clean outbox_messages: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM demo_orders"); err != nil {
		t.Fatalf("clean demo_orders: %v", err)
	}
}

// savePendingMessage 对应 Java savePendingMessage helper：标准三件套
// （ORDER / ORDER_CREATED / {"orderNo":...}）。
func savePendingMessage(t *testing.T, ctx context.Context, db *sql.DB, aggregateID string) *Message {
	t.Helper()
	m := Pending("ORDER", aggregateID, "ORDER_CREATED", `{"orderNo":"`+aggregateID+`"}`)
	if err := SaveMessage(ctx, db, m); err != nil {
		t.Fatalf("save pending message: %v", err)
	}
	return m
}

// recordingPublisher 对应 Java 测试的 RecordingPublisher：统计调用次数与处理过的
// 消息 ID，可按策略模拟下游故障（failPolicy 命中即返回错误）。
type recordingPublisher struct {
	mu           sync.Mutex
	invocations  int
	processedIDs []string
	failPolicy   func(*Message) bool
}

func (p *recordingPublisher) Publish(ctx context.Context, m *Message) error {
	p.mu.Lock()
	p.invocations++
	policy := p.failPolicy
	p.mu.Unlock()
	if policy != nil && policy(m) {
		return fmt.Errorf("模拟下游处理失败: %s", m.AggregateID)
	}
	p.mu.Lock()
	p.processedIDs = append(p.processedIDs, strconv.FormatInt(m.ID, 10))
	p.mu.Unlock()
	return nil
}

func (p *recordingPublisher) setFailPolicy(f func(*Message) bool) {
	p.mu.Lock()
	p.failPolicy = f
	p.mu.Unlock()
}

func (p *recordingPublisher) reset() {
	p.mu.Lock()
	p.invocations = 0
	p.processedIDs = nil
	p.failPolicy = nil
	p.mu.Unlock()
}

// 用例一（对应 relayOnce_publishes_due_message_and_marks_published）：
// 到期消息被消费 → PUBLISHED + publishedAt + retryCount=0 + 恰好调用一次。
func TestRelayOncePublishesDueMessageAndMarksPublished(t *testing.T) {
	db := newTestDB(t)
	cleanTables(t, db)
	ctx := context.Background()

	msg := savePendingMessage(t, ctx, db, "ORDER-1")
	rec := &recordingPublisher{}
	relay := New(db, rec, testRelayConfig())

	processed, err := relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed=%d, want 1", processed)
	}

	reloaded, err := FindMessage(ctx, db, msg.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != StatusPublished {
		t.Fatalf("status=%s, want PUBLISHED", reloaded.Status)
	}
	if reloaded.PublishedAt == nil {
		t.Fatal("published_at must be set on PUBLISHED")
	}
	if reloaded.RetryCount != 0 {
		t.Fatalf("retryCount=%d, want 0", reloaded.RetryCount)
	}
	if rec.invocations != 1 {
		t.Fatalf("invocations=%d, want 1（at-least-once 的「一次」基线）", rec.invocations)
	}
	if len(rec.processedIDs) != 1 || rec.processedIDs[0] != strconv.FormatInt(msg.ID, 10) {
		t.Fatalf("processedIDs=%v, want [%d]", rec.processedIDs, msg.ID)
	}
}

// 用例二（对应 relayOnce_skips_messages_not_due_yet）：
// nextRetryAt 在未来的消息不被捞取 —— 退避机制的下半句（到期才可消费）。
func TestRelayOnceSkipsMessagesNotDueYet(t *testing.T) {
	db := newTestDB(t)
	cleanTables(t, db)
	ctx := context.Background()

	msg := Pending("ORDER", "ORDER-FUTURE-1", "ORDER_CREATED", "{}")
	msg.NextRetryAt = time.Now().UTC().Add(time.Hour) // 对应 Java setNextRetryAt(now+1h)
	if err := SaveMessage(ctx, db, msg); err != nil {
		t.Fatalf("save: %v", err)
	}

	rec := &recordingPublisher{}
	relay := New(db, rec, testRelayConfig())

	processed, err := relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed=%d, want 0（未到期不能捞）", processed)
	}
	reloaded, err := FindMessage(ctx, db, msg.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != StatusPending {
		t.Fatalf("status=%s, want PENDING", reloaded.Status)
	}
	if rec.invocations != 0 {
		t.Fatalf("invocations=%d, want 0", rec.invocations)
	}
}

// 用例三（对应 relayOnce_retries_with_backoff_then_marks_failed_after_max_retries）：
// 失败 → retryCount 1→2→3 → 超限转 FAILED 死信；退避窗口内 RelayOnce 返回 0。
func TestRelayOnceRetriesWithBackoffThenMarksFailed(t *testing.T) {
	db := newTestDB(t)
	cleanTables(t, db)
	ctx := context.Background()

	msg := savePendingMessage(t, ctx, db, "ORDER-FAIL-1")
	rec := &recordingPublisher{}
	rec.setFailPolicy(func(m *Message) bool { return m.AggregateID == "ORDER-FAIL-1" })
	relay := New(db, rec, testRelayConfig())

	// 第 1 轮：消费失败 → retryCount=1，退避 100ms（backoff(1) = 100ms × 2^0）
	processed, err := relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("relay #1: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed #1=%d, want 1（失败也计入处理条数）", processed)
	}
	afterFirst, err := FindMessage(ctx, db, msg.ID)
	if err != nil {
		t.Fatalf("reload #1: %v", err)
	}
	if afterFirst.Status != StatusPending || afterFirst.RetryCount != 1 {
		t.Fatalf("after #1: status=%s retryCount=%d, want PENDING/1", afterFirst.Status, afterFirst.RetryCount)
	}
	if !afterFirst.NextRetryAt.After(afterFirst.CreatedAt) {
		t.Fatalf("nextRetryAt=%v must be after createdAt=%v（退避窗口生效）",
			afterFirst.NextRetryAt, afterFirst.CreatedAt)
	}

	// 退避窗口内：捞不到（第二次 RelayOnce 返回 0）
	processed, err = relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("relay #2: %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed #2=%d, want 0（退避窗口内不可重投）", processed)
	}
	if rec.invocations != 1 {
		t.Fatalf("invocations=%d, want 1", rec.invocations)
	}

	time.Sleep(150 * time.Millisecond) // > backoff(1)=100ms

	// 第 2 轮：再失败 → retryCount=2，退避 200ms（指数增长）
	processed, err = relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("relay #3: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed #3=%d, want 1", processed)
	}
	afterSecond, err := FindMessage(ctx, db, msg.ID)
	if err != nil {
		t.Fatalf("reload #2: %v", err)
	}
	if afterSecond.Status != StatusPending || afterSecond.RetryCount != 2 {
		t.Fatalf("after #2: status=%s retryCount=%d, want PENDING/2", afterSecond.Status, afterSecond.RetryCount)
	}

	time.Sleep(250 * time.Millisecond) // > backoff(2)=200ms

	// 第 3 轮：retryCount=3 >= maxRetries=3 → FAILED 死信，不再投递
	processed, err = relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("relay #4: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed #4=%d, want 1", processed)
	}
	afterThird, err := FindMessage(ctx, db, msg.ID)
	if err != nil {
		t.Fatalf("reload #3: %v", err)
	}
	if afterThird.Status != StatusFailed {
		t.Fatalf("status=%s, want FAILED（重试超限转死信）", afterThird.Status)
	}
	if afterThird.RetryCount != 3 {
		t.Fatalf("retryCount=%d, want 3", afterThird.RetryCount)
	}
	if rec.invocations != 3 {
		t.Fatalf("invocations=%d, want 3（maxRetries=3 = 至多 3 次投递）", rec.invocations)
	}
}

// 用例四（对应 concurrent_relays_claim_disjoint_batches_without_duplicates）：
// 4 个 relay 并发认领 20 条消息 —— 总处理数恰 20、无重复无遗漏、全部 PUBLISHED。
// SKIP LOCKED 的存在性证明：BatchSize=10 < total=20，若没有「跳过他人锁定行」，
// 并发 relay 要么阻塞要么重复消费。
func TestConcurrentRelaysClaimDisjointBatches(t *testing.T) {
	db := newTestDB(t)
	cleanTables(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const total = 20
	for i := 1; i <= total; i++ {
		savePendingMessage(t, ctx, db, fmt.Sprintf("ORDER-C-%d", i))
	}

	rec := &recordingPublisher{}
	relay := New(db, rec, testRelayConfig())

	const workers = 4
	start := make(chan struct{}) // 对应 Java CountDownLatch start
	var totalProcessed atomic.Int32
	var wg sync.WaitGroup
	for i := range workers {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			n, err := relay.RelayOnce(ctx)
			if err != nil {
				t.Errorf("relay %d: %v", i, err)
				return
			}
			totalProcessed.Add(int32(n))
		}()
	}
	close(start)
	wg.Wait()

	if totalProcessed.Load() != total {
		t.Fatalf("totalProcessed=%d, want %d（少了 = 漏认领，多了 = 重复消费）", totalProcessed.Load(), total)
	}
	if rec.invocations != total {
		t.Fatalf("invocations=%d, want %d（每条消息恰好被投递一次）", rec.invocations, total)
	}
	if len(rec.processedIDs) != total {
		t.Fatalf("processedIDs=%d distinct, want %d（SKIP LOCKED 保证认领不重叠）",
			len(rec.processedIDs), total)
	}
	messages, err := ListMessages(ctx, db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, m := range messages {
		if m.Status != StatusPublished {
			t.Fatalf("message %d status=%s, want all PUBLISHED", m.ID, m.Status)
		}
	}
}

// 用例五（对应 rollback_leaves_no_outbox_row_and_no_business_row）：
// 业务行与事件行同事务回滚 —— 两张表都不残留（outbox 原子性的立身之本）。
func TestRollbackLeavesNoOutboxRowAndNoBusinessRow(t *testing.T) {
	db := newTestDB(t)
	cleanTables(t, db)
	ctx := context.Background()

	outboxBefore, err := CountMessages(ctx, db)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	ordersBefore, err := CountOrders(ctx, db)
	if err != nil {
		t.Fatalf("count orders: %v", err)
	}

	// 同一事务写入订单 + 事件，然后强制回滚。
	// 对应 Java 在 transactionTemplate 里抛 IllegalStateException 触发回滚；
	// Go 没有「异常自动回滚」，教学点相同：业务失败路径绝不提交。
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := SaveOrderTx(ctx, tx, NewOrder("ORDER-ROLLBACK-1")); err != nil {
		t.Fatalf("save order in tx: %v", err)
	}
	if err := SaveMessageTx(ctx, tx, Pending("ORDER", "ORDER-ROLLBACK-1", "ORDER_CREATED", "{}")); err != nil {
		t.Fatalf("save message in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	outboxAfter, err := CountMessages(ctx, db)
	if err != nil {
		t.Fatalf("count outbox after: %v", err)
	}
	ordersAfter, err := CountOrders(ctx, db)
	if err != nil {
		t.Fatalf("count orders after: %v", err)
	}
	if outboxAfter != outboxBefore {
		t.Fatalf("outbox rows %d -> %d, want unchanged（事件泄漏！）", outboxBefore, outboxAfter)
	}
	if ordersAfter != ordersBefore {
		t.Fatalf("order rows %d -> %d, want unchanged（业务行也不该残留）", ordersBefore, ordersAfter)
	}
}
