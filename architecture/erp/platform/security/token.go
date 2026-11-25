// Package security 是平台层的鉴权底座（对应 Java 版 erp-platform-security）。
//
// 三个角色：TokenService（令牌签发/解析）、ApiAuthFilter（解析 Bearer 令牌）、
// AuthMiddleware（校验 X-App-Id，只拦 /openapi/ 前缀）。演示用的极简实现。
package security

import (
	"strings"
	"sync"

	"github.com/google/uuid"
)

// TokenService 令牌服务（对应 Java 版 TokenService）。
//
// 演示实现：签发随机 UUID 并在内存中记住 appId→token 映射；
// 解析时反查。真实项目会换 JWT/OAuth。
type TokenService struct {
	mu     sync.Mutex
	appIDs map[string]string // token -> appId
}

// NewTokenService 构造令牌服务。
func NewTokenService() *TokenService {
	return &TokenService{appIDs: make(map[string]string)}
}

// Issue 为指定应用签发令牌（对应 Java 的 issue，Java 版甚至没存映射 —— Go 版存了，
// 这样 ResolveAppId 语义才闭环）。
func (s *TokenService) Issue(appID string) string {
	token := uuid.NewString()
	s.mu.Lock()
	s.appIDs[token] = appID
	s.mu.Unlock()
	return token
}

// ResolveAppId 从令牌反查应用 ID（对应 Java 的 resolveAppId）。
// 未知令牌返回空串。与 Java 版一致：空令牌返回空。
func (s *TokenService) ResolveAppID(token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appIDs[token]
}
