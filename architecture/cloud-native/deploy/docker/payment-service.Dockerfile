# payment 服务镜像 —— Go 多阶段构建（对照 Java deploy/docker/payment-service.Dockerfile）。
# 构建（在 architecture/cloud-native 目录）：
#   docker build -f deploy/docker/payment-service.Dockerfile -t go-tutorials/cn-payment-service:1.0 .
FROM golang:1.26-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY shared/ shared/
COPY payment/ payment/
WORKDIR /src/payment
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/paymentsvc ./cmd/paymentsvc

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/paymentsvc /app/paymentsvc
EXPOSE 8080
ENTRYPOINT ["/app/paymentsvc"]
