package persistence

import (
	"context"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
	"gorm.io/gorm"
)

// 编译期断言：GormTxRunner 实现了 shared.TxRunner 端口。
var _ shared.TxRunner = (*GormTxRunner)(nil)

type txKey struct{}

// GormTxRunner 是 shared.TxRunner 的 GORM 实现。
//
// 事务传播语义与 Spring @Transactional(REQUIRED) 一致：
//   - ctx 中已有事务 → 直接加入，不再新开；
//   - ctx 中没有事务 → 新开一个，并把事务句柄放进 ctx 传给内层。
type GormTxRunner struct{ db *gorm.DB }

func NewGormTxRunner(db *gorm.DB) *GormTxRunner {
	return &GormTxRunner{db: db}
}

func (r *GormTxRunner) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx) // 已在事务中（REQUIRED 传播：加入当前事务）
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DBFromContext 仓储实现用它获取数据库句柄：
// 在事务中返回事务句柄，否则返回基础连接。
// 领域层和应用层全程只接触 context，不感知 gorm —— 这就是事务传播的秘密。
func DBFromContext(ctx context.Context, base *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return base.WithContext(ctx)
}
