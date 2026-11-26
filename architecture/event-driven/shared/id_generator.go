package shared

import "github.com/google/uuid"

// NewID 生成 UUID（对应 Java 的 shared.IdGenerator.newId()）。
func NewID() string {
	return uuid.NewString()
}
