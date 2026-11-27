# product 服务镜像 —— Go 多阶段构建（对照 Java deploy/docker/product-service.Dockerfile）。
# 构建（在 architecture/cloud-native 目录）：
#   docker build -f deploy/docker/product-service.Dockerfile -t go-tutorials/cn-product-service:1.0 .
FROM golang:1.26-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY shared/ shared/
COPY product/ product/
WORKDIR /src/product
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/productsvc ./cmd/productsvc

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/productsvc /app/productsvc
EXPOSE 8080
ENTRYPOINT ["/app/productsvc"]
