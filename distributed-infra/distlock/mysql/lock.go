// Package mysql 对应 java-tutorials/distributed-infra/distributed-lock/mysql：
// MysqlUniqueIndexLock（唯一索引抢占）与 MysqlPessimisticLock（SELECT ... FOR UPDATE）。
//
// Java 版 demo 用 H2 内存库（MODE=MySQL）实现零外部依赖；Go 生态没有嵌入式 MySQL
// 兼容库（SQLite 缺 FOR UPDATE / SKIP LOCKED），所以本包面向真 MySQL（docker compose
// 起 33061），测试探测不可达自动跳过 —— 与 Java 版 Redis/ZK demo 的「探测跳过」同款策略。
// 裸 database/sql 不引 ORM：本模块的教学焦点就是 SQL 语义本身。
//
// 两种锁的对照（Java 版教学主轴）：
//
//	唯一索引锁：INSERT 成功即持锁，DELETE 自己的行释放；无过期 → 崩溃残留需人工清理
//	悲观锁：  FOR UPDATE 锁已有行，commit 即释放；锁生命周期完全绑定事务，无 owner 概念
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"github.com/jamieleestudio/go-tutorials/distributed-infra/distlock"
)

// lockTable 演示用锁表：lock_name 主键 = 唯一索引的载体（对应 Java 建表语句）。
const (
	schemaName = "distributed_infra"
	schemaDB   = "CREATE DATABASE IF NOT EXISTS distributed_infra CHARACTER SET utf8mb4"
	schemaSQL  = `
CREATE TABLE IF NOT EXISTS lock_table (
  lock_name VARCHAR(255) PRIMARY KEY,
  owner_id  VARCHAR(255)
) ENGINE=InnoDB`
)

// Connect 建「库 + 表」并返回绑定 distributed_infra 库的连接。
// serverDSN 不含库名（root:rootpw@tcp(127.0.0.1:33061)/?parseTime=true&loc=UTC）。
// 注意不能先 USE 再复用原连接池 —— database/sql 的连接是池化的，USE 只影响
// 执行它的那一个连接；正确姿势是建库后把 DSN 解析、填上库名再开第二个连接池
// （不能手工字符串拼库名 —— DSN 尾部带查询参数，拼接会把库名塞进参数段）。
func Connect(ctx context.Context, serverDSN string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(serverDSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	server, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open server: %w", err)
	}
	if _, err := server.ExecContext(ctx, schemaDB); err != nil {
		_ = server.Close()
		return nil, fmt.Errorf("create database: %w", err)
	}
	if err := server.Close(); err != nil {
		return nil, err
	}

	cfg.DBName = schemaName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", schemaName, err)
	}
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return db, nil
}

// UniqueIndexLock 唯一索引锁 —— INSERT 成功即持锁（对应 Java MysqlUniqueIndexLock）。
//
// 防误删：解锁 DELETE 带 owner_id 条件 —— 只删自己插入的行，与 Redis 的
// Lua compare-and-del 同一思想。无过期机制：进程崩溃后行残留、锁永久卡死，
// 需人工清理（与 Redis TTL / ZK 会话自愈对比 —— 本模块教学主轴）。
type UniqueIndexLock struct {
	db       *sql.DB
	lockName string
	ownerID  string // 构造时生成的唯一标识，防误删他人锁
}

// NewUniqueIndexLock 建锁（ownerID 自动生成，对应 Java 构造器里的 UUID）。
func NewUniqueIndexLock(db *sql.DB, lockName string) *UniqueIndexLock {
	return &UniqueIndexLock{db: db, lockName: lockName, ownerID: distlock.NewID()}
}

// TryLock 非阻塞抢占：INSERT 撞唯一索引 → 别人持有。
func (l *UniqueIndexLock) TryLock(ctx context.Context) (bool, error) {
	_, err := l.db.ExecContext(ctx,
		"INSERT INTO lock_table(lock_name, owner_id) VALUES(?, ?)", l.lockName, l.ownerID)
	if err != nil {
		if isDuplicateKey(err) {
			return false, nil // 唯一索引冲突 = 锁已被持有（对应 Java tryInsert 的 rollback+false）
		}
		return false, fmt.Errorf("insert lock row %s: %w", l.lockName, err)
	}
	return true, nil
}

// Lock 自旋重试（对应 Java tryLock(deadline) 的 parkNanos(100ms) 骨架）。
func (l *UniqueIndexLock) Lock(ctx context.Context) error {
	return distlock.SpinTryLockForever(ctx, l.TryLock)
}

// Unlock 删除【自己】的锁行（DELETE 带 owner_id —— 防误删的教学点）。
func (l *UniqueIndexLock) Unlock(ctx context.Context) error {
	res, err := l.db.ExecContext(ctx,
		"DELETE FROM lock_table WHERE lock_name = ? AND owner_id = ?", l.lockName, l.ownerID)
	if err != nil {
		return fmt.Errorf("delete lock row %s: %w", l.lockName, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("unlock failed, lock not held by us: %s", l.lockName)
	}
	return nil
}

// PessimisticLock 悲观锁 —— SELECT ... FOR UPDATE 行级排他锁（对应 Java MysqlPessimisticLock）。
//
// 锁的生命周期完全绑定事务：acquire 开事务锁行，unlock = commit 释放。
// 无 owner、无超时 —— 连接断开事务自动回滚，行锁随之释放（比唯一索引锁优雅）。
//
// Java 版注释里点过的语义瑕疵本包如实保留并说明：实现没用 FOR UPDATE NOWAIT，
// 后来者会阻塞在 SELECT 上直到对方提交（自旋更多是防御性代码）。真正的非阻塞
// tryLock 升级点：MySQL 8.0+ 的 FOR UPDATE NOWAIT / SKIP LOCKED（outbox 包用的正是 SKIP LOCKED）。
type PessimisticLock struct {
	db       *sql.DB
	lockName string
	tx       *sql.Tx // 持锁期间的开放式事务（行锁保持到 commit/rollback）
}

// NewPessimisticLock 建锁。注意：锁的行必须预先存在（FOR UPDATE 锁的是已存在的行）。
func NewPessimisticLock(db *sql.DB, lockName string) *PessimisticLock {
	return &PessimisticLock{db: db, lockName: lockName}
}

// TryLock 开事务锁已有行；行不存在 → rollback 返回 false（对应 Java trySelectForUpdate）。
func (l *PessimisticLock) TryLock(ctx context.Context) (bool, error) {
	if l.tx != nil {
		return true, nil // 已持有（对应 Java 可重入判断的简化形态）
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx for %s: %w", l.lockName, err)
	}
	var name string
	err = tx.QueryRowContext(ctx,
		"SELECT lock_name FROM lock_table WHERE lock_name = ? FOR UPDATE", l.lockName).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback() // 行不存在：锁语义不成立，释放事务
		return false, nil
	}
	if err != nil {
		_ = tx.Rollback()
		// 别人持锁时 SELECT FOR UPDATE 会阻塞到锁等待超时（50s 默认）或对方提交；
		// demo 场景持有者都会及时 commit，阻塞窗口极短。
		return false, fmt.Errorf("select for update %s: %w", l.lockName, err)
	}
	l.tx = tx // 行锁保持到 Unlock 的 commit
	return true, nil
}

// Lock 自旋重试。
func (l *PessimisticLock) Lock(ctx context.Context) error {
	return distlock.SpinTryLockForever(ctx, l.TryLock)
}

// Unlock commit 事务 —— 行锁随事务结束自动释放（对应 Java unlock 的 connection.commit()）。
func (l *PessimisticLock) Unlock(ctx context.Context) error {
	if l.tx == nil {
		return fmt.Errorf("unlock failed, lock not held: %s", l.lockName)
	}
	err := l.tx.Commit()
	l.tx = nil
	if err != nil {
		return fmt.Errorf("commit (unlock) %s: %w", l.lockName, err)
	}
	return nil
}

// SeedRow 预插锁行（悲观锁演示前必须预插 —— FOR UPDATE 锁已存在的行；
// 对应 Java MysqlLockDemo.initTable 里的 resource_B）。
func SeedRow(ctx context.Context, db *sql.DB, lockName string) error {
	_, err := db.ExecContext(ctx,
		"INSERT IGNORE INTO lock_table(lock_name, owner_id) VALUES(?, 'seed')", lockName)
	return err
}

// isDuplicateKey 识别 MySQL 唯一索引冲突（错误码 1062）。
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
