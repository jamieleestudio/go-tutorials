// Package integration 是平台层的外部系统集成底座（对应 Java 版 erp-platform-integration）。
//
// ExternalSystemClient 是端口；HttpExternalSystemClient 是 HTTP 实现。
// attendance 域的 FaceRecognitionClient 会用到它 —— 领域端口在域内定义，
// 平台端口在平台层定义，适配器组合两者。
package integration

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// ExternalSystemClient 外部系统客户端端口（对应 Java 版 ExternalSystemClient 接口）。
type ExternalSystemClient interface {
	System() string
	Post(path, body string) (string, error)
}

// HttpExternalSystemClient HTTP 实现（对应 Java 版 HttpExternalSystemClient）。
type HttpExternalSystemClient struct {
	baseURL string
	client  *http.Client
}

// NewHttpExternalSystemClient 构造。baseURL 为空时表示未配置外部系统
// （System() 返回空串，业务侧据此走本地降级逻辑 —— 对应 Java 版的 "http" 常量，
// Go 版加了可配置性以支撑测试）。
func NewHttpExternalSystemClient(baseURL string) *HttpExternalSystemClient {
	return &HttpExternalSystemClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// System 返回集成类型标识。
func (c *HttpExternalSystemClient) System() string {
	if c.baseURL == "" {
		return ""
	}
	return "http"
}

// Post 发送 POST 请求并返回响应体。
func (c *HttpExternalSystemClient) Post(path, body string) (string, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("post %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	return buf.String(), nil
}
