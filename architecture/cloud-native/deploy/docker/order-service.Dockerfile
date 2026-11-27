# order 服务镜像 —— Go 多阶段构建（对照 Java deploy/docker/order-service.Dockerfile）。
# 构建（在 architecture/cloud-native 目录，构建上下文 = 4 个 module 的父目录）：
#   docker build -f deploy/docker/order-service.Dockerfile -t go-tutorials/cn-order-service:1.0 .
FROM golang:1.26-alpine AS build
# 国内网络必备（仓库统一 GOPROXY 决策）
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
# workspace 里的 replace 是相对路径（../shared 等），镜像内按同样的目录结构摆放
COPY shared/ shared/
COPY product/ product/
COPY payment/ payment/
COPY order/ order/
WORKDIR /src/order
# CGO_ENABLED=0：glebarez/sqlite 是纯 Go 驱动，静态编译出无依赖二进制
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/ordersvc ./cmd/ordersvc

# distroless/static：无 shell、无包管理器、非 root —— K8s 镜像最佳实践
# （对照 Java 的 eclipse-temurin:21-jre-jammy；Java 只能带整个 JRE，Go 二进制 ~15MB）
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ordersvc /app/ordersvc
EXPOSE 8080
ENTRYPOINT ["/app/ordersvc"]
