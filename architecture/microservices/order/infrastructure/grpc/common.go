// Package grpc order 服务的基础设施 RPC 客户端。
//
// 对应 Java 版 infrastructure/feign 包：把远程调用（Go 用 gRPC + protobuf）
// 适配为 provider 定义的契约接口（productapi.ProductService / paymentapi.PaymentService），
// 并叠加微服务治理能力：
//   - gobreaker 熔断器（对应 Resilience4j：失败率 ≥50% 且样本 ≥10 触发打开，
//     open 10s 后半开探测）；
//   - 3s gRPC deadline（对应 Feign/Resilience4j 的超时控制）；
//   - 失败降级（对应 Feign fallback：基础设施性失败 → 业务规则异常）。
//
// 与 Java 版的一个差异：Java 的 Feign fallback 会吞掉所有远程错误（包括 404），
// Go 版在进入熔断/fallback 之前先识别 status.NotFound 并还原为
// EntityNotFound —— 保留"资源不存在"的业务语义。
package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// remoteTimeout 远程调用 deadline（对应 Resilience4j TimeLimiter）。
const remoteTimeout = 3 * time.Second

// BreakerSettings 熔断器配置（对应 Java README 里 Resilience4j 的配置语义）。
//
//	Resilience4j                          gobreaker
//	slidingWindowSize=10                  ReadyToTrip 中 Requests>=10
//	failureRateThreshold=50               失败数*2 >= 总请求数
//	waitDurationInOpenState=10s           Timeout: 10s
//	halfOpenPermittedCalls=3              MaxRequests: 3
func BreakerSettings(name string) gobreaker.Settings {
	return gobreaker.Settings{
		Name:        name,
		MaxRequests: 3, // half-open 时允许的探测请求
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.Requests >= 10 && c.TotalFailures*2 >= c.Requests
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Warn("circuit breaker state change", "breaker", name, "from", from.String(), "to", to.String())
		},
	}
}

// remoteBizError 远程服务的"业务性应答"（NotFound/InvalidArgument）。
// 用类型标记区分，保证它不计入熔断器的失败统计。
type remoteBizError struct{ err error }

func (e remoteBizError) Error() string { return e.err.Error() }

// remoteCall 执行一次 gRPC 调用，完成三件事：
//  1. 熔断器包裹（open 时直接快速失败，不发请求）；
//  2. deadline 传播（给远程调用加 3s 超时）；
//  3. 错误分类 —— 业务性应答不算熔断失败，基础设施性失败才算。
func remoteCall[T any](ctx context.Context, cb *gobreaker.CircuitBreaker, fn func(ctx context.Context) (T, error)) (T, error) {
	result, err := cb.Execute(func() (any, error) {
		cctx, cancel := context.WithTimeout(ctx, remoteTimeout)
		defer cancel()
		v, err := fn(cctx)
		if err != nil {
			switch status.Code(err) {
			case codes.NotFound, codes.InvalidArgument:
				// 远程服务的业务性应答（服务端 GlobalExceptionHandler 映射而来）：
				// 服务是"健康地拒绝"，不计入熔断失败统计。
				return remoteBizError{err}, nil
			default:
				return nil, err // 网络/超时/不可用：计入熔断失败
			}
		}
		return v, nil
	})

	var zero T
	if err != nil {
		// 熔断器自身拒绝（ErrOpenState / ErrTooManyRequests）或基础设施性失败
		return zero, err
	}
	if biz, ok := result.(remoteBizError); ok {
		return zero, biz
	}
	return result.(T), nil
}

// translateError 把 remoteCall 的错误翻译为领域错误。
//
// 优先级（与 Java fallback 的差异点）：
//  1. status.NotFound → EntityNotFound（Go 改进：保留资源不存在语义）
//  2. status.InvalidArgument → BusinessRuleViolation（保留远程的错误码）
//  3. 其余（网络失败/熔断打开/超时）→ fallback：BusinessRuleViolation(<SERVICE>_SERVICE_UNAVAILABLE)
func translateError(err error, service, fallbackCode string) error {
	if biz, ok := err.(remoteBizError); ok {
		code, message := splitStatusMessage(biz.err)
		switch status.Code(biz.err) {
		case codes.NotFound:
			return shared.NewEntityNotFound(code, message)
		default: // codes.InvalidArgument
			return shared.NewBusinessRuleViolation(code, message)
		}
	}
	return shared.NewBusinessRuleViolation(fallbackCode,
		fmt.Sprintf("%s service unavailable: %v", service, err))
}

// splitStatusMessage 服务端把错误码放在 status message 前缀（"CODE: message"），
// 这里切分还原；解析不出前缀时用通用码。
func splitStatusMessage(err error) (string, string) {
	s, ok := status.FromError(err)
	if !ok {
		return "REMOTE_ERROR", err.Error()
	}
	msg := s.Message()
	for i := 0; i < len(msg); i++ {
		if msg[i] == ':' && i+1 < len(msg) && msg[i+1] == ' ' && isErrorCodePrefix(msg[:i]) {
			return msg[:i], msg[i+2:]
		}
	}
	return "REMOTE_ERROR", msg
}

// isErrorCodePrefix 粗判前缀是否像错误码（全大写/下划线），
// 避免把普通 message 里的冒号误切。
func isErrorCodePrefix(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// dial 建 gRPC 连接（进程内固定地址；服务发现的演进点见 README）。
func dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc connect %s: %w", addr, err)
	}
	return conn, nil
}
