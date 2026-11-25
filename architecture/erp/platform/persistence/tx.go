package persistence

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// TxRunner 事务运行器（Go 版的 @Transactional 传播原语，从 P2 的 shared/persistence 演化而来）。
//
// erp 版里它归 platform 所有（Java 版由 Spring @Transactional 提供）。
type TxRunner interface {
	// WithinTx REQUIRED 传播语义：ctx 已有事务则加入，否则新开一个。
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// GormTxRunner TxRunner 的 GORM 实现。
type GormTxRunner struct{ db *gorm.DB }

// NewGormTxRunner 构造事务运行器。
func NewGormTxRunner(db *gorm.DB) *GormTxRunner { return &GormTxRunner{db: db} }

// WithinTx 事务传播：与 Spring @Transactional(REQUIRED) 语义一致。
func (r *GormTxRunner) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx) // 已在事务中：加入当前事务
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DBFromContext 仓储实现用它获取数据库句柄：
// 在事务中返回事务句柄，否则返回基础连接。
// 领域层和应用层全程只接触 context，不感知 gorm —— 事务传播的秘密。
func DBFromContext(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return base.WithContext(ctx)
}
