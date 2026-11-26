// Package shared 是共享内核（Shared Kernel）。
//
// 存放三个限界上下文（order / product / payment）都需要的最小公共抽象：
// 领域错误、聚合根基类、领域事件、进程内事件总线、ID 生成器、事务端口。
//
// 铁律（由架构守护测试保证）：shared 不允许 import 任何业务上下文的包，
// 也不允许 import 任何框架（gin / gorm）—— 保持纯净，对应 Java 版
// shared 包 "zero framework dependencies" 的约束。
package shared

import "fmt"

// ErrorKind 领域错误分类，供上层（HTTP 中间件）决定响应码。
//
// Java 版用异常类型区分（BusinessRuleViolationException / EntityNotFoundException），
// Go 只有一个错误类型 + Kind 字段区分。
type ErrorKind string

const (
	// KindBusinessRule 业务规则被违反 → HTTP 400
	KindBusinessRule ErrorKind = "BUSINESS_RULE_VIOLATION"
	// KindEntityNotFound 实体不存在 → HTTP 404
	KindEntityNotFound ErrorKind = "ENTITY_NOT_FOUND"
)

// DomainError 领域层唯一错误类型（对应 Java 的 DomainException 继承体系）。
type DomainError struct {
	Kind    ErrorKind
	Code    string
	Message string
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewBusinessRuleViolation 构造业务规则错误（对应 BusinessRuleViolationException）。
func NewBusinessRuleViolation(code, message string) *DomainError {
	return &DomainError{Kind: KindBusinessRule, Code: code, Message: message}
}

// NewEntityNotFound 构造实体不存在错误（对应 EntityNotFoundException）。
func NewEntityNotFound(code, message string) *DomainError {
	return &DomainError{Kind: KindEntityNotFound, Code: code, Message: message}
}
