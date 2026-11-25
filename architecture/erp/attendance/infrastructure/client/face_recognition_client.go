// Package client 是 attendance 域的基础设施适配器（对应 Java 版 infrastructure.client）。
package client

import (
	"strings"

	"github.com/jamieleestudio/go-tutorials/architecture/erp/attendance/domain/service"
	"github.com/jamieleestudio/go-tutorials/architecture/erp/platform/integration"
)

// 编译期断言：FaceRecognitionClient 实现领域端口。
var _ service.FaceRecognitionPort = (*FaceRecognitionClient)(nil)

// FaceRecognitionClient 人脸识别适配器：组合【平台层】ExternalSystemClient 端口，
// 实现【领域层】FaceRecognitionPort —— 平台能力与领域需求的桥接点。
type FaceRecognitionClient struct {
	externalSystemClient integration.ExternalSystemClient
}

// NewFaceRecognitionClient 构造适配器。
func NewFaceRecognitionClient(externalSystemClient integration.ExternalSystemClient) *FaceRecognitionClient {
	return &FaceRecognitionClient{externalSystemClient: externalSystemClient}
}

// Verify 演示规则（与 Java 版一致）：faceToken 以 "face-" 开头且外部系统已配置。
// 未配置外部系统时 System() 返回空串 → 一律校验失败（走降级：不带 faceToken 即跳过校验）。
func (c *FaceRecognitionClient) Verify(studentID, faceToken string) bool {
	return faceToken != "" &&
		strings.HasPrefix(faceToken, "face-") &&
		c.externalSystemClient.System() != ""
}
