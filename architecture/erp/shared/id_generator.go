package shared

import "github.com/google/uuid"

// NewID 生成 UUID（对应 Java 的 shared.IdGenerator.next()；
// Java 的 UUID 在 JDK 标准库里，Go 侧用事实标准库 google/uuid 等价替代）。
func NewID() string { return uuid.NewString() }
