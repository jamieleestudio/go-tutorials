// Package shared 是 erp 的共享内核（对应 Java 版 erp-shared-kernel）。
//
// 跨域共享的最小模型：领域错误、聚合根基类、领域事件、ID 生成器。
// 零框架依赖 —— 不允许 import gin / gorm（架构护栏保证）。
package shared

// ErrorKind 领域错误分类，供平台层（platform/web）决定 HTTP 状态码。
type ErrorKind string

const (
	// KindBusinessRule 业务规则被违反 → HTTP 422（erp 版与 P1/P2 的 400 不同）
	KindBusinessRule ErrorKind = "BUSINESS_RULE_VIOLATION"
	// KindEntityNotFound 实体不存在 → HTTP 404
	KindEntityNotFound ErrorKind = "ENTITY_NOT_FOUND"
)

// DomainError 领域层唯一错误类型（对应 Java 的 DomainException 继承体系）。
type DomainError struct {
	Kind    ErrorKind
	Message string
}

func (e *DomainError) Error() string { return e.Message }

// NewBusinessRuleViolation 构造业务规则错误（对应 BusinessRuleViolationException）。
func NewBusinessRuleViolation(message string) *DomainError {
	return &DomainError{Kind: KindBusinessRule, Message: message}
}

// NewEntityNotFound 构造实体不存在错误（对应 EntityNotFoundException）。
func NewEntityNotFound(message string) *DomainError {
	return &DomainError{Kind: KindEntityNotFound, Message: message}
}
