package outbox

import (
	"testing"
	"time"
)

// TestNextBackoff 指数退避公式：initial × multiplier^(retryCount-1)
// （application.yml 默认 5s × 2.0 的整条曲线，不连库验证）。
func TestNextBackoff(t *testing.T) {
	cfg := Config{InitialBackoff: 5 * time.Second, BackoffMultiplier: 2.0}
	cases := []struct {
		retryCount int
		want       time.Duration
	}{
		{1, 5 * time.Second},
		{2, 10 * time.Second},
		{3, 20 * time.Second},
		{4, 40 * time.Second},
		{5, 80 * time.Second},
	}
	for _, c := range cases {
		if got := NextBackoff(cfg, c.retryCount); got != c.want {
			t.Errorf("NextBackoff(retry=%d)=%v, want %v", c.retryCount, got, c.want)
		}
	}
	// 倍数 1.0 → 恒定退避（固定间隔重试的退化形态）
	constant := Config{InitialBackoff: 300 * time.Millisecond, BackoffMultiplier: 1.0}
	if got := NextBackoff(constant, 7); got != 300*time.Millisecond {
		t.Errorf("multiplier=1 retry=7: got %v, want constant 300ms", got)
	}
}

// TestResolveFailureStateMachine 失败状态机（不连库）：
// PENDING 退避 → 计数递增 → 达到 maxRetries 转 FAILED 死信。
func TestResolveFailureStateMachine(t *testing.T) {
	cfg := Config{MaxRetries: 3, InitialBackoff: 100 * time.Millisecond, BackoffMultiplier: 2.0}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 第 1 次失败（RetryCount 0→1）：PENDING + 退避 100ms
	first := resolveFailure(cfg, &Message{RetryCount: 0}, now)
	if first.status != StatusPending || first.retryCount != 1 {
		t.Fatalf("first: status=%s retryCount=%d, want PENDING/1", first.status, first.retryCount)
	}
	if first.nextRetryAt != now.Add(100*time.Millisecond) {
		t.Fatalf("first: nextRetryAt=%v, want %v", first.nextRetryAt, now.Add(100*time.Millisecond))
	}
	if first.backoff != 100*time.Millisecond {
		t.Fatalf("first: backoff=%v, want 100ms", first.backoff)
	}

	// 第 2 次失败（1→2）：PENDING + 退避 200ms（指数增长）
	second := resolveFailure(cfg, &Message{RetryCount: 1}, now)
	if second.status != StatusPending || second.retryCount != 2 {
		t.Fatalf("second: status=%s retryCount=%d, want PENDING/2", second.status, second.retryCount)
	}
	if second.backoff != 200*time.Millisecond {
		t.Fatalf("second: backoff=%v, want 200ms（multiplier=2 指数增长）", second.backoff)
	}

	// 第 3 次失败（2→3，3 >= maxRetries=3）：FAILED 死信，不再安排退避
	third := resolveFailure(cfg, &Message{RetryCount: 2}, now)
	if third.status != StatusFailed || third.retryCount != 3 {
		t.Fatalf("third: status=%s retryCount=%d, want FAILED/3", third.status, third.retryCount)
	}
	if third.backoff != 0 || !third.nextRetryAt.IsZero() {
		t.Fatalf("third: backoff=%v nextRetryAt=%v, want 零值（死信不再退避）", third.backoff, third.nextRetryAt)
	}
}

// TestResolveSuccess 成功路径：PUBLISHED + publishedAt 落点。
func TestResolveSuccess(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out := resolveSuccess(now)
	if out.status != StatusPublished {
		t.Fatalf("status=%s, want PUBLISHED", out.status)
	}
	if out.publishedAt == nil || !out.publishedAt.Equal(now) {
		t.Fatalf("publishedAt=%v, want %v", out.publishedAt, now)
	}
	if out.retryCount != 0 || out.backoff != 0 {
		t.Fatalf("success must not touch retry counters: retryCount=%d backoff=%v", out.retryCount, out.backoff)
	}
}

// TestConfigDefaults 零值配置回落 Java @ConfigurationProperties 默认值，
// 显式值不被覆盖（对应 ${outbox.relay.batch-size:50} 的占位符兜底语义）。
func TestConfigDefaults(t *testing.T) {
	cfg := Config{}.WithDefaults()
	if cfg.PollInterval != time.Second {
		t.Errorf("pollInterval=%v, want 1s", cfg.PollInterval)
	}
	if cfg.BatchSize != 50 {
		t.Errorf("batchSize=%d, want 50", cfg.BatchSize)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("maxRetries=%d, want 5", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != 5*time.Second {
		t.Errorf("initialBackoff=%v, want 5s", cfg.InitialBackoff)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("backoffMultiplier=%v, want 2.0", cfg.BackoffMultiplier)
	}

	custom := Config{BatchSize: 10, MaxRetries: 3}.WithDefaults()
	if custom.BatchSize != 10 || custom.MaxRetries != 3 {
		t.Errorf("explicit values must be kept: batchSize=%d maxRetries=%d", custom.BatchSize, custom.MaxRetries)
	}
	if custom.InitialBackoff != 5*time.Second {
		t.Errorf("unspecified fields still get defaults: initialBackoff=%v", custom.InitialBackoff)
	}
}

// TestPendingFactory 工厂语义：PENDING、零重试、nextRetryAt == createdAt（落库即到期）。
func TestPendingFactory(t *testing.T) {
	m := Pending("ORDER", "ORDER-1", "ORDER_CREATED", `{"orderNo":"ORDER-1"}`)
	if m.Status != StatusPending {
		t.Errorf("status=%s, want PENDING", m.Status)
	}
	if m.RetryCount != 0 {
		t.Errorf("retryCount=%d, want 0", m.RetryCount)
	}
	if !m.NextRetryAt.Equal(m.CreatedAt) {
		t.Errorf("nextRetryAt=%v createdAt=%v, want equal（落库即到期）", m.NextRetryAt, m.CreatedAt)
	}
	if m.PublishedAt != nil {
		t.Errorf("publishedAt must be nil for pending message")
	}
}
