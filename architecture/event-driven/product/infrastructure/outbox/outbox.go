// Package outbox 本地消息表（Outbox 模式）—— 可靠消息的基石。
//
// 对应 Java 版 infrastructure/outbox 包：
//   - OutboxEventPublisher 实现 application.EventPublisherService：
//     事件与业务数据写入【同一个本地事务】，保证不丢单；
//   - OutboxRelay 1s 轮询未发布消息 → 经投递通道发出（对应 Java 版发 Kafka）→ 标记 published_at。
//
// 投递语义 at-least-once：投递失败不标记，下一轮重投；消费端靠 eventId 去重兜底。
// 投递通道（DeliverFunc）是接缝：教学版默认 HTTP webhook（零外部设施），
// 生产演进为 Kafka/NATS 实现（接口不变）。
package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/event"
	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/persistence"
	"gorm.io/gorm"
)

// OutboxMessageEntity 本地消息表实体（对应 Java 的 OutboxMessageEntity）。
type OutboxMessageEntity struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	EventID     string `gorm:"column:event_id;size:36;not null;uniqueIndex"`
	Topic       string `gorm:"column:topic;size:64;not null;index"`
	AggregateID string `gorm:"column:aggregate_id;size:36;not null"`
	EventType   string `gorm:"column:event_type;size:64;not null"`
	Payload     string `gorm:"column:payload;type:text;not null"`
	CreatedAt   time.Time
	PublishedAt *time.Time `gorm:"column:published_at"`
}

func (OutboxMessageEntity) TableName() string { return "outbox_messages" }

// OutboxRepository 本地消息表端口 —— Relay 与 Publisher 共用，可 mock 测试。
type OutboxRepository interface {
	Save(ctx context.Context, msg *OutboxMessageEntity) error
	FindPending(ctx context.Context, limit int) ([]*OutboxMessageEntity, error)
	MarkPublished(ctx context.Context, id int64) error
}

// OutboxRepositoryImpl GORM 实现。
type OutboxRepositoryImpl struct{ db *gorm.DB }

func NewOutboxRepository(db *gorm.DB) *OutboxRepositoryImpl {
	return &OutboxRepositoryImpl{db: db}
}

func (r *OutboxRepositoryImpl) Save(ctx context.Context, msg *OutboxMessageEntity) error {
	return persistence.DBFromContext(ctx, r.db).Create(msg).Error
}

// FindPending 取未发布消息（对应 findTop50ByPublishedAtIsNullOrderByCreatedAtAsc）。
func (r *OutboxRepositoryImpl) FindPending(ctx context.Context, limit int) ([]*OutboxMessageEntity, error) {
	db := persistence.DBFromContext(ctx, r.db)
	var msgs []*OutboxMessageEntity
	err := db.Where("published_at IS NULL").Order("created_at").Limit(limit).Find(&msgs).Error
	return msgs, err
}

func (r *OutboxRepositoryImpl) MarkPublished(ctx context.Context, id int64) error {
	db := persistence.DBFromContext(ctx, r.db)
	return db.Model(&OutboxMessageEntity{}).Where("id = ?", id).Update("published_at", time.Now()).Error
}

// OutboxEventPublisher application.EventPublisherService 端口的实现。
//
// 事件写 outbox_messages 表 —— 与业务数据同一本地事务（ctx 携带事务句柄，
// 由外层 TxRunner 保证），实际投递由 Relay 异步完成。
// 这就是"业务+消息原子落库"的 Outbox 核心。
// （Java 版靠 @Transactional 线程绑定拿事务，Go 版的对应物就是 ctx。）
type OutboxEventPublisher struct {
	repo OutboxRepository
}

func NewOutboxEventPublisher(repo OutboxRepository) *OutboxEventPublisher {
	return &OutboxEventPublisher{repo: repo}
}

func (p *OutboxEventPublisher) Publish(ctx context.Context, evt event.IntegrationEvent) error {
	envelope, err := event.Encode(evt)
	if err != nil {
		return fmt.Errorf("serialize integration event: %w", err)
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("serialize envelope: %w", err)
	}
	return p.repo.Save(ctx, &OutboxMessageEntity{
		EventID:     envelope.EventID,
		Topic:       envelope.Topic,
		AggregateID: envelope.AggregateID,
		EventType:   envelope.Type,
		Payload:     string(payload),
		CreatedAt:   time.Now(),
	})
}

// DeliverFunc 投递通道接缝：教学版 = HTTP webhook；演进点 = Kafka/NATS producer。
type DeliverFunc func(ctx context.Context, envelope event.Envelope) error

// Relay 轮询本地消息表并投递（对应 Java 的 @Scheduled(fixedDelay=1000) OutboxRelay）。
type Relay struct {
	repo     OutboxRepository
	deliver  DeliverFunc
	interval time.Duration
}

func NewRelay(repo OutboxRepository, deliver DeliverFunc, interval time.Duration) *Relay {
	return &Relay{repo: repo, deliver: deliver, interval: interval}
}

// RelayOnce 单轮投递：最多 50 条未发布消息（测试与调度共用）。
func (r *Relay) RelayOnce(ctx context.Context) error {
	pending, err := r.repo.FindPending(ctx, 50)
	if err != nil {
		return err
	}
	for _, msg := range pending {
		var envelope event.Envelope
		if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
			slog.Error("outbox message unparsable, skipping", "eventId", msg.EventID, "error", err)
			continue
		}
		if err := r.deliver(ctx, envelope); err != nil {
			// at-least-once：不标记，下一轮重投（消费端 eventId 去重兜底）
			slog.Warn("outbox delivery failed, will retry", "eventId", msg.EventID, "topic", msg.Topic, "error", err)
			continue
		}
		if err := r.repo.MarkPublished(ctx, msg.ID); err != nil {
			return err
		}
		slog.Debug("relayed outbox message", "eventId", msg.EventID, "topic", msg.Topic)
	}
	return nil
}

// Run 周期调度（阻塞直到 ctx 取消，对应 @Scheduled）。
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.RelayOnce(ctx); err != nil {
				slog.Error("outbox relay round failed", "error", err)
			}
		}
	}
}

// HTTPDeliver webhook 投递实现（Kafka 的教学对应物）。
// subscribers: topic → 订阅者 HTTP 端点列表（POST /internal/events）。
// 任一订阅者失败即返回错误 → 本条消息下轮重投（at-least-once）。
func HTTPDeliver(subscribers map[string][]string) DeliverFunc {
	return func(ctx context.Context, envelope event.Envelope) error {
		urls, ok := subscribers[envelope.Topic]
		if !ok || len(urls) == 0 {
			return nil // 无订阅者的 topic：直接视为已发布
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			return err
		}
		for _, url := range urls {
			if err := postEvent(ctx, url, body); err != nil {
				return fmt.Errorf("deliver %s to %s: %w", envelope.EventID, url, err)
			}
		}
		return nil
	}
}

func postEvent(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("subscriber returned %d", resp.StatusCode)
	}
	return nil
}
