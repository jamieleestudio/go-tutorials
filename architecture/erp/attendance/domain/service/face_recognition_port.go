// Package service 是 attendance 域的领域服务端口（对应 Java 版 domain.service）。
package service

// FaceRecognitionPort 人脸识别端口 —— 领域层定义的能力接口，
// 由 infrastructure/client 的 FaceRecognitionClient 适配外部系统（依赖倒置）。
type FaceRecognitionPort interface {
	// Verify 校验学生的人脸令牌。
	Verify(studentID, faceToken string) bool
}
