package shared

import "context"

// TxRunner 事务边界端口。
//
// 对应 Java 版应用服务上的 @Transactional 注解：
//   - Java 由 Spring AOP 在方法外围自动开事务；
//   - Go 没有注解，由应用服务显式调用 WithinTx 把一段逻辑包进事务；
//   - 事务句柄通过 context 向下传播，仓储实现用 DBFromContext 取出。
//
// 接口定义在 shared（端口），实现在 platform/persistence（适配器）—— 依赖倒置。
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
