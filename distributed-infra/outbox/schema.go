// 库表与数据访问（对应 Java schema.sql + DemoOrder/DemoOrderRepository）。
// 裸 database/sql 不引 ORM：outbox 的教学焦点是 SQL 语义本身（认领 SQL、退避字段、
// 同事务写入），每一行 SQL 都要看得见。
package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// schemaName 演示专用库（与 distlock 的 distributed_infra 分离 —— 各模块管各模块的表）。
const schemaName = "outbox_demo"

// DDL 逐条对照 Java schema.sql（幂等 CREATE TABLE IF NOT EXISTS，启动即执行）。
const (
	schemaDB = "CREATE DATABASE IF NOT EXISTS outbox_demo CHARACTER SET utf8mb4"

	tableDDL = `
CREATE TABLE IF NOT EXISTS outbox_messages (
  id             BIGINT       NOT NULL AUTO_INCREMENT,
  aggregate_type VARCHAR(64)  NOT NULL COMMENT '聚合类型，如 ORDER',
  aggregate_id   VARCHAR(64)  NOT NULL COMMENT '聚合业务 ID，如订单号',
  event_type     VARCHAR(64)  NOT NULL COMMENT '事件类型，如 ORDER_CREATED',
  payload        VARCHAR(4096) NULL COMMENT '事件内容（JSON）',
  status         VARCHAR(16)  NOT NULL COMMENT '状态: PENDING / PUBLISHED / FAILED',
  retry_count    INT          NOT NULL DEFAULT 0 COMMENT '已失败重试次数',
  next_retry_at  DATETIME(3)  NOT NULL COMMENT '到期才可被捞取（退避）',
  created_at     DATETIME(3)  NOT NULL COMMENT '创建时间',
  published_at   DATETIME(3)  NULL COMMENT '消费成功时间',
  PRIMARY KEY (id),
  KEY idx_status_next_retry (status, next_retry_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	orderDDL = `
CREATE TABLE IF NOT EXISTS demo_orders (
  id         BIGINT      NOT NULL AUTO_INCREMENT,
  order_no   VARCHAR(64) NOT NULL COMMENT '订单号',
  status     VARCHAR(32) NOT NULL COMMENT '订单状态',
  created_at DATETIME(3) NOT NULL COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_no (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
)

// Connect 建「库 + 两表」并返回绑定 outbox_demo 库的连接。
// serverDSN 不含库名（root:rootpw@tcp(127.0.0.1:33061)/?parseTime=true&loc=UTC）。
// 不能 USE 后复用原连接池（池化连接，USE 只影响单个连接），也不能手工字符串拼库名
// —— DSN 尾部带查询参数，拼接会把库名塞进参数段；用 ParseDSN/FormatDSN 才稳。
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
	for name, ddl := range map[string]string{"outbox_messages": tableDDL, "demo_orders": orderDDL} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create table %s: %w", name, err)
		}
	}
	return db, nil
}

// Order 演示用业务表实体（对应 Java DemoOrder）：与 outbox 记录同事务写入，展示原子性
// —— 「业务成功则事件必然存在」的另一半。
type Order struct {
	ID        int64
	OrderNo   string
	Status    string
	CreatedAt time.Time
}

// NewOrder 新建演示订单（对应 Java DemoOrder 构造器：状态即 CREATED）。
func NewOrder(orderNo string) *Order {
	return &Order{OrderNo: orderNo, Status: "CREATED", CreatedAt: time.Now().UTC()}
}

// 两个 Java JpaRepository 的裸 SQL 对应物。事务变体（SaveXxxTx）是原子性的落点：
// 业务行与消息行必须共用同一个 *sql.Tx。

// SaveMessage 单条写消息（自管事务，测试预置数据用）。
func SaveMessage(ctx context.Context, db *sql.DB, m *Message) error {
	return withTx(ctx, db, func(tx *sql.Tx) error { return SaveMessageTx(ctx, tx, m) })
}

// SaveMessageTx 事务内写消息（写回自增 ID）。
func SaveMessageTx(ctx context.Context, tx *sql.Tx, m *Message) error {
	res, err := tx.ExecContext(ctx, `
INSERT INTO outbox_messages
  (aggregate_type, aggregate_id, event_type, payload, status, retry_count, next_retry_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.AggregateType, m.AggregateID, m.EventType, m.Payload,
		m.Status, m.RetryCount, m.NextRetryAt, m.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert outbox message %s: %w", m.AggregateID, err)
	}
	if m.ID, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	return nil
}

// SaveOrder 单条写订单（自管事务）。
func SaveOrder(ctx context.Context, db *sql.DB, o *Order) error {
	return withTx(ctx, db, func(tx *sql.Tx) error { return SaveOrderTx(ctx, tx, o) })
}

// SaveOrderTx 事务内写订单（写回自增 ID）。
func SaveOrderTx(ctx context.Context, tx *sql.Tx, o *Order) error {
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now().UTC()
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO demo_orders (order_no, status, created_at) VALUES (?, ?, ?)`,
		o.OrderNo, o.Status, o.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert demo order %s: %w", o.OrderNo, err)
	}
	if o.ID, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	return nil
}

// FindMessage 按 ID 读取（测试断言状态流转用，对应 Java findById + orElseThrow）。
func FindMessage(ctx context.Context, db *sql.DB, id int64) (*Message, error) {
	m, err := scanMessage(db.QueryRowContext(ctx,
		`SELECT `+messageColumns+` FROM outbox_messages WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("outbox message %d not found", id)
	}
	return m, err
}

// FindMessageByAggregate 按聚合 ID 读取最早一条（demo 的 awaitStatus 用）。
func FindMessageByAggregate(ctx context.Context, db *sql.DB, aggregateID string) (*Message, error) {
	m, err := scanMessage(db.QueryRowContext(ctx,
		`SELECT `+messageColumns+` FROM outbox_messages WHERE aggregate_id = ? ORDER BY id LIMIT 1`, aggregateID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("outbox message for %s not found", aggregateID)
	}
	return m, err
}

// ListMessages 全表读取（demo 的结果汇总用，对应 Java findAll）。
func ListMessages(ctx context.Context, db *sql.DB) ([]*Message, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+messageColumns+` FROM outbox_messages ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list outbox messages: %w", err)
	}
	defer rows.Close()
	var out []*Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListOrders 全表读取（demo 的结果汇总用，对应 Java orderRepository.findAll()）。
func ListOrders(ctx context.Context, db *sql.DB) ([]*Order, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, order_no, status, created_at FROM demo_orders ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list demo orders: %w", err)
	}
	defer rows.Close()
	var out []*Order
	for rows.Next() {
		o := &Order{}
		if err := rows.Scan(&o.ID, &o.OrderNo, &o.Status, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan demo order: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// messageColumns 列清单（SELECT * 的显式化写法，列序与 scanMessage 对齐）。
const messageColumns = `id, aggregate_type, aggregate_id, event_type, payload,
  status, retry_count, next_retry_at, created_at, published_at`

// rowScanner QueryRow 与 Rows 的公共接口（database/sql 的惯用小抽象）。
type rowScanner interface{ Scan(dest ...any) error }

func scanMessage(rs rowScanner) (*Message, error) {
	m := &Message{}
	if err := rs.Scan(&m.ID, &m.AggregateType, &m.AggregateID, &m.EventType, &m.Payload,
		&m.Status, &m.RetryCount, &m.NextRetryAt, &m.CreatedAt, &m.PublishedAt); err != nil {
		return nil, fmt.Errorf("scan outbox message: %w", err)
	}
	return m, nil
}

// CountMessages / CountOrders 行数统计（回滚测试的前后对照，对应 Java count()）。
func CountMessages(ctx context.Context, db *sql.DB) (int64, error) {
	return count(ctx, db, "SELECT COUNT(*) FROM outbox_messages")
}

func CountOrders(ctx context.Context, db *sql.DB) (int64, error) {
	return count(ctx, db, "SELECT COUNT(*) FROM demo_orders")
}

func count(ctx context.Context, db *sql.DB, q string) (int64, error) {
	var n int64
	if err := db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return n, nil
}

// withTx 自管事务小 helper（单条写入场景）。
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
