// Package zookeeper 对应 java-tutorials/distributed-infra/distributed-lock/zookeeper：
// ZookeeperDistributedLock（包装 Curator InterProcessMutex）。
//
// Java 版是 Curator 黑盒（注释原话：「InterProcessMutex 是可重入锁，基于临时顺序节点 +
// Watch 机制实现公平锁。释放时需要调用 release()，否则会残留临时节点（会话断开时自动清理）」）。
// Go 生态没有 Curator 级别的封装库是常态 —— 本包用 go-zookeeper 原始客户端【手写算法】，
// 正好把黑盒展开成四条教学点：
//
//  1. 临时顺序节点（EphemeralSequential）：/locks/{name}/lock-0000000001 ——
//     会话断开节点自动清理（持有者崩溃自愈，对照 MySQL 唯一索引锁的残留、
//     Redis TTL 的过期自愈 —— 本模块失效模式对照主轴）。
//  2. 公平锁：序列号最小者持锁，后来者排队（对应 Curator InterProcessMutex 的公平语义）。
//  3. 只 watch 前驱（Java 版注释未展开的羊群效应教学点）：若所有人 watch 「锁节点删除」，
//     锁一释放全体被唤醒重新竞争（羊群）；只监听自己的前驱，释放时只有一个 goroutine
//     被唤醒 —— O(1) 唤醒。ZK 序列号零填充，字典序 == 数值序。
//  4. 可重入：Curator 的 InterProcessMutex 同线程重复 acquire 计数；本包是基础版
//     （同 goroutine 重复 TryLock 直接返回已持有），完整可重入需要计数器封装。
//
// 失效模式对照总结：ZK 会话断开 → 临时节点自清理 + 排队者被 watch 唤醒 ——
// 三种介质中最优雅的自愈；代价是性能最低（每次加解锁都是写操作）。
// Go 生态的惯用替代是 etcd（clientv3 concurrency.Session/：租约 + revision 排序 +
// watch 前驱，与本算法完全同构）。
package zookeeper

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	zk "github.com/go-zookeeper/zk"
)

// lockRoot 演示锁的根目录（对应 Java demo 的 /locks/resource_A）。
const lockRoot = "/locks"

// ZkLock 手写 ZK 分布式锁：临时顺序节点 + watch 前驱。
type ZkLock struct {
	conn *zk.Conn
	dir  string // /locks/{name}
	path string // 持有的顺序节点完整路径（空 = 未持有）
}

// New 建锁（name = 锁资源名，对应 Java 的 /locks/resource_A）。
func New(conn *zk.Conn, name string) *ZkLock {
	return &ZkLock{conn: conn, dir: lockRoot + "/" + name}
}

// Connect 连接 ZK（server 不可达时返回错误 —— 测试据此 skip，Java 同款探测策略）。
func Connect(addr string, sessionTimeout time.Duration) (*zk.Conn, error) {
	conn, _, err := zk.Connect([]string{addr}, sessionTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}
	return conn, nil
}

// ensureDir mkdir -p /locks 与 /locks/{name}（ZK create 要求父节点存在）。
func (l *ZkLock) ensureDir() error {
	for _, p := range []string{lockRoot, l.dir} {
		exists, _, err := l.conn.Exists(p)
		if err != nil {
			return fmt.Errorf("exists %s: %w", p, err)
		}
		if exists {
			continue
		}
		_, err = l.conn.Create(p, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists { // 并发创建竞争：存在即成功
			return fmt.Errorf("create %s: %w", p, err)
		}
	}
	return nil
}

// TryLock 非阻塞尝试：创建临时顺序节点 → 序号最小即持锁；
// 非最小则【删除自己的排队节点】返回 false（真非阻塞，对应 Java 的 acquire(0, MILLISECONDS)）。
func (l *ZkLock) TryLock(ctx context.Context) (bool, error) {
	if l.path != "" {
		return true, nil // 基础可重入：本 goroutine 已持有
	}
	if err := l.ensureDir(); err != nil {
		return false, err
	}
	seqPath, err := l.conn.Create(l.dir+"/lock-", nil, zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return false, fmt.Errorf("create sequential node in %s: %w", l.dir, err)
	}
	mine := baseName(l.dir, seqPath)
	children, _, err := l.conn.Children(l.dir)
	if err != nil {
		_ = l.conn.Delete(seqPath, -1) // 失败清理排队节点（对应 Java 释放临时节点的纪律）
		return false, fmt.Errorf("children %s: %w", l.dir, err)
	}
	if isLowest(children, mine) {
		l.path = seqPath // 序号最小 = 持锁（公平锁：先到先得）
		return true, nil
	}
	_ = l.conn.Delete(seqPath, -1) // 非最小：撤队，返回 false
	return false, nil
}

// Lock 公平排队直到拿到锁：创建节点 → watch 前驱 → 前驱删除时重判位次。
// 只 watch 前驱 = 避免羊群效应（见包注释教学点 3）。
func (l *ZkLock) Lock(ctx context.Context) error {
	if err := l.ensureDir(); err != nil {
		return err
	}
	seqPath, err := l.conn.Create(l.dir+"/lock-", nil, zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return fmt.Errorf("create sequential node in %s: %w", l.dir, err)
	}
	mine := baseName(l.dir, seqPath)

	for {
		children, _, err := l.conn.Children(l.dir)
		if err != nil {
			_ = l.conn.Delete(seqPath, -1)
			return fmt.Errorf("children %s: %w", l.dir, err)
		}
		if isLowest(children, mine) {
			l.path = seqPath // 持锁
			return nil
		}
		// 只 watch 自己的前驱：前驱释放时仅自己被唤醒（羊群效应的解法）
		pred := l.dir + "/" + predecessorOf(children, mine)
		exists, _, events, err := l.conn.ExistsW(pred)
		if err != nil {
			_ = l.conn.Delete(seqPath, -1)
			return fmt.Errorf("watch %s: %w", pred, err)
		}
		if !exists {
			continue // 前驱恰好在 Children 与 ExistsW 之间消失：立即重判
		}
		select {
		case ev := <-events:
			if ev.Type == zk.EventNodeDeleted {
				continue // 前驱释放 → 重判位次（一轮一个排队者前进，公平）
			}
		case <-ctx.Done():
			_ = l.conn.Delete(seqPath, -1) // 取消排队：清理节点（会话也在则节点也会自清理）
			return ctx.Err()
		}
	}
}

// Unlock 删除持有的临时节点（对应 Java mutex.release()；
// 不释放则节点残留到会话断开才清理 —— Java 注释原话的对应纪律）。
func (l *ZkLock) Unlock(ctx context.Context) error {
	if l.path == "" {
		return fmt.Errorf("unlock failed, lock not held: %s", l.dir)
	}
	if err := l.conn.Delete(l.path, -1); err != nil {
		return fmt.Errorf("delete %s: %w", l.path, err)
	}
	l.path = ""
	return nil
}

// baseName 取顺序节点名（去目录前缀）。
func baseName(dir, path string) string {
	return strings.TrimPrefix(path, dir+"/")
}

// isLowest 判断 mine 是否是当前最小的序列节点（字典序 == 数值序：ZK 序列号零填充）。
func isLowest(children []string, mine string) bool {
	return predecessorOf(children, mine) == ""
}

// predecessorOf 返回排在 mine 之前的最大序列节点名；mine 是最小者时返回 ""。
func predecessorOf(children []string, mine string) string {
	sorted := append([]string(nil), children...)
	sort.Strings(sorted)
	var pred string
	for _, c := range sorted {
		if c == mine {
			return pred
		}
		if c < mine {
			pred = c
		}
	}
	return pred
}
