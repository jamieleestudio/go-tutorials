.PHONY: test build vet tidy compose-up compose-down run run-mmm run-erp proto run-ms run-ms-product run-ms-payment run-ms-order run-ms-gateway run-eda run-eda-product run-eda-payment run-eda-order run-dist run-dist-product run-dist-payment run-dist-order

# 全部 Go module（workspace 内），按目录逐个执行
MODULES := base \
           architecture/monolithic \
           architecture/monolithic-multi-module/shared \
           architecture/monolithic-multi-module/product \
           architecture/monolithic-multi-module/payment \
           architecture/monolithic-multi-module/order \
           architecture/monolithic-multi-module/bootstrap \
           architecture/erp \
           architecture/microservices/shared \
           architecture/microservices/product \
           architecture/microservices/payment \
           architecture/microservices/order \
           architecture/microservices/gateway \
           architecture/event-driven/shared \
           architecture/event-driven/product \
           architecture/event-driven/payment \
           architecture/event-driven/order \
           architecture/distributed/shared \
           architecture/distributed/product \
           architecture/distributed/payment \
           architecture/distributed/order

## 运行全部测试（逐模块；P1 集成测试无 MySQL 自动跳过，P2/P3 零依赖）
test:
	@for m in $(MODULES); do echo "== $$m =="; (cd $$m && go test ./...) || exit 1; done

build:
	@for m in $(MODULES); do (cd $$m && go build ./...) || exit 1; done

vet:
	@for m in $(MODULES); do (cd $$m && go vet ./...) || exit 1; done

tidy:
	@for m in $(MODULES); do (cd $$m && go mod tidy) || exit 1; done

## 启动 monolithic 的 MySQL（Docker，供 P1 使用；P2 默认 SQLite 内存库无需它）
compose-up:
	docker compose -f architecture/monolithic/docker-compose.yml up -d

compose-down:
	docker compose -f architecture/monolithic/docker-compose.yml down

## 启动 P1 单体（会先拉起 MySQL）
run: compose-up
	go run ./architecture/monolithic/cmd/monolithic

## 启动 P2 多模块单体（SQLite 内存库，零外部依赖，端口 8081）
run-mmm:
	go run ./architecture/monolithic-multi-module/bootstrap/cmd/mmm

## 启动 P3 erp 的 boot 形态（全部端 + job，SQLite 内存库，端口 8080）
run-erp:
	go run ./architecture/erp/app/cmd/erpboot

## 启动 P3 erp 的其他部署形态（make run-erp-admin/mobile/openapi）
run-erp-admin:
	go run ./architecture/erp/app/cmd/erpadmin

run-erp-mobile:
	go run ./architecture/erp/app/cmd/erpmobile

run-erp-openapi:
	go run ./architecture/erp/app/cmd/erpopenapi

## 重新生成 P4 的 protobuf 契约代码（需要 protoc + protoc-gen-go + protoc-gen-go-grpc 在 PATH）
proto:
	protoc -I architecture/microservices/product/proto \
	  --go_out=architecture/microservices/product/proto/productpb \
	  --go_opt=module=github.com/jamieleestudio/go-tutorials/architecture/microservices/product/proto/productpb \
	  --go-grpc_out=architecture/microservices/product/proto/productpb \
	  --go-grpc_opt=module=github.com/jamieleestudio/go-tutorials/architecture/microservices/product/proto/productpb \
	  architecture/microservices/product/proto/product.proto
	protoc -I architecture/microservices/payment/proto \
	  --go_out=architecture/microservices/payment/proto/paymentpb \
	  --go_opt=module=github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/proto/paymentpb \
	  --go-grpc_out=architecture/microservices/payment/proto/paymentpb \
	  --go-grpc_opt=module=github.com/jamieleestudio/go-tutorials/architecture/microservices/payment/proto/paymentpb \
	  architecture/microservices/payment/proto/payment.proto

## 启动 P4 微服务全家桶（3 服务 + 网关；sh 后台 &，Ctrl-C 前台网关）
run-ms:
	go run ./architecture/microservices/product/cmd/productsvc &
	go run ./architecture/microservices/payment/cmd/paymentsvc &
	go run ./architecture/microservices/order/cmd/ordersvc &
	go run ./architecture/microservices/gateway/cmd/gateway

## P4 单独启动某个服务（需要 4 个终端时逐个用）
run-ms-product:
	go run ./architecture/microservices/product/cmd/productsvc

run-ms-payment:
	go run ./architecture/microservices/payment/cmd/paymentsvc

run-ms-order:
	go run ./architecture/microservices/order/cmd/ordersvc

run-ms-gateway:
	go run ./architecture/microservices/gateway/cmd/gateway

## 启动 P5 事件驱动全家桶（3 服务；Saga 投递介质是 HTTP webhook，零外部设施）
run-eda:
	go run ./architecture/event-driven/product/cmd/productsvc &
	go run ./architecture/event-driven/payment/cmd/paymentsvc &
	go run ./architecture/event-driven/order/cmd/ordersvc

## P5 单独启动某个服务
run-eda-product:
	go run ./architecture/event-driven/product/cmd/productsvc

run-eda-payment:
	go run ./architecture/event-driven/payment/cmd/paymentsvc

run-eda-order:
	go run ./architecture/event-driven/order/cmd/ordersvc

## 启动 P6 分布式全栈（3 进程朴素 HTTP RPC，无注册中心/网关）
run-dist:
	go run ./architecture/distributed/product/cmd/productsvc &
	go run ./architecture/distributed/payment/cmd/paymentsvc &
	go run ./architecture/distributed/order/cmd/ordersvc

## P6 单独启动某服务（需要 3 个终端时用）
run-dist-product:
	go run ./architecture/distributed/product/cmd/productsvc

run-dist-payment:
	go run ./architecture/distributed/payment/cmd/paymentsvc

run-dist-order:
	go run ./architecture/distributed/order/cmd/ordersvc
