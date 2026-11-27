// Package distid 对应 java-tutorials/distributed-infra/distributed-id ——
// Java 版是空壳 pom（TODO 注释原文：「雪花算法 / Leaf / 号段模式 / UUID 等实现」），
// Go 版直接落地 TODO 里排第一的雪花算法（Snowflake），纯标准库零依赖。
//
// 雪花算法 = 64 bit 一体三段：
//
//	1 bit 符号（恒 0）| 41 bit 毫秒时间戳 | 10 bit 节点 ID | 12 bit 序列
//
// 对应 Java TODO 里另外三个方案的 Go 对照（README 展开）：
//   - UUID      → 随机 128 bit，无序、索引写放大 —— Go 的 crypto/rand 可零依赖生成
//   - 号段模式  → DB 批量取号段内存分配，双 buffer —— Go 实现就是「atomic 区间 + 预取」
//   - Leaf      = 美团开源的号段+雪花混合 —— 思路同上，工程封装
//
// 雪花的著名坑（本包逐一处理）：
//  1. 时钟回拨：NTP 校准回退 → 可能发出重复 ID。处理：回拨直接报错（保守策略），
//     生产实现可改为「等待追平」或「备用位/扩展位」；
//  2. 单毫秒序列耗尽（4096 个）：自旋等到下一毫秒；
//  3. 节点 ID 分配：手工分配易撞 —— 实际可用 DB/Redis/etcd 租约分配 nodeID。
package distid

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// epoch 发号起点：2025-01-01T00:00:00Z（Twitter 原版用 2010-11-04，本包起点更新，
// 41 bit 时间戳可用 69 年 —— 到 2094 年）。
const epoch int64 = 1735689600000

const (
	timestampBits = 41
	nodeBits      = 10
	sequenceBits  = 12

	maxNodeID   = -1 ^ (-1 << nodeBits)     // 1023
	maxSequence = -1 ^ (-1 << sequenceBits) // 4095

	nodeShift      = sequenceBits            // 节点段左移位数
	timestampShift = sequenceBits + nodeBits // 时间戳段左移位数
)

// ErrClockMovedBackwards 时钟回拨错误（雪花算法的著名坑，保守策略直接拒绝发号）。
var ErrClockMovedBackwards = errors.New("clock moved backwards")

// Snowflake 单节点发号器。并发安全（互斥保护 lastTS/sequence 两个游标）。
type Snowflake struct {
	mu       sync.Mutex
	nodeID   int64
	lastTS   int64 // 上次发号的毫秒时间戳
	sequence int64 // 当前毫秒内已用序列
}

// New 建 nodeID 号发号器（nodeID ∈ [0, 1023]）。
func New(nodeID int64) (*Snowflake, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("node id %d out of range [0, %d]", nodeID, maxNodeID)
	}
	return &Snowflake{nodeID: nodeID}, nil
}

// NextID 发一个全局唯一 ID。同一毫秒序列递增，耗尽则自旋到下一毫秒；
// 时钟回拨返回 ErrClockMovedBackwards。
func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < s.lastTS {
		return 0, fmt.Errorf("%w: now=%d last=%d", ErrClockMovedBackwards, now, s.lastTS)
	}
	if now == s.lastTS {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// 单毫秒 4096 个序列耗尽：自旋等下一毫秒（对应 Java 版常见实现的 waitNextMillis）
			for now <= s.lastTS {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}
	s.lastTS = now

	return (now-epoch)<<timestampShift | s.nodeID<<nodeShift | s.sequence, nil
}

// Decompose 反解 ID：回到发号时刻、节点、序列（雪花 ID 自带元数据的价值所在）。
func Decompose(id int64) (at time.Time, nodeID, sequence int64) {
	ms := (id >> timestampShift) + epoch
	at = time.UnixMilli(ms)
	nodeID = (id >> nodeShift) & maxNodeID
	sequence = id & maxSequence
	return at, nodeID, sequence
}
