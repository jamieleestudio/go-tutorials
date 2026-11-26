// Package processed 消费端幂等去重（processed_events 表）。
//
// at-least-once 投递意味着重复事件必然出现：消费端先查 eventId，
// 已处理则跳过 —— 处理完成与标记在同一事务里（由外层 TxRunner 保证）。
package processed

import (
	"context"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared/persistence"
	"gorm.io/gorm"
)

// ProcessedEventEntity 已处理事件表（eventId 主键，对应 Java 的 ProcessedEventEntity）。
type ProcessedEventEntity struct {
	EventID     string    `gorm:"column:event_id;size:36;primaryKey"`
	ProcessedAt time.Time `gorm:"column:processed_at;not null"`
}

func (ProcessedEventEntity) TableName() string { return "processed_events" }

// DeduplicationService application.EventDeduplicationService 端口的实现。
type DeduplicationService struct{ db *gorm.DB }

func NewDeduplicationService(db *gorm.DB) *DeduplicationService {
	return &DeduplicationService{db: db}
}

func (s *DeduplicationService) IsDuplicate(ctx context.Context, eventID string) (bool, error) {
	db := persistence.DBFromContext(ctx, s.db)
	var count int64
	err := db.Model(&ProcessedEventEntity{}).Where("event_id = ?", eventID).Count(&count).Error
	return count > 0, err
}

func (s *DeduplicationService) MarkProcessed(ctx context.Context, eventID string) error {
	db := persistence.DBFromContext(ctx, s.db)
	return db.Create(&ProcessedEventEntity{EventID: eventID, ProcessedAt: time.Now()}).Error
}
