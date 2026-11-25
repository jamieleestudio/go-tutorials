// Package kernel 是平台层的最基础包（对应 Java 版 erp-platform-kernel）。
//
// Result/PageResult 是 erp 全部 HTTP 响应的统一包装 —— 与 P1/P2 直接返回业务对象不同，
// erp 版模仿企业级 API 网关风格：所有响应都包一层 {success, code, message, data}。
//
// 本包只允许标准库依赖（架构护栏保证）。
package kernel

import "encoding/json"

// Result 统一响应包装（对应 Java 版 kernel.Result 泛型 record）。
type Result[T any] struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// OK 成功响应（对应 Java 的 Result.ok，code 固定 "0"）。
func OK[T any](data T) Result[T] {
	return Result[T]{Success: true, Code: "0", Message: "ok", Data: data}
}

// Fail 失败响应（对应 Java 的 Result.fail）。
func Fail(code, message string) Result[json.RawMessage] {
	return Result[json.RawMessage]{Success: false, Code: code, Message: message}
}

// PageResult 分页响应（对应 Java 版 kernel.PageResult 泛型 record）。
type PageResult[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// PageOf 构造分页对象（对应 Java 的 PageResult.of）。
func PageOf[T any](items []T, total int64, page, size int) PageResult[T] {
	if items == nil {
		items = []T{}
	}
	return PageResult[T]{Items: items, Total: total, Page: page, Size: size}
}
