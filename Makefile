.PHONY: test build vet tidy compose-up compose-down run run-mmm run-erp

# 全部 Go module（workspace 内），按目录逐个执行
MODULES := architecture/monolithic \
           architecture/monolithic-multi-module/shared \
           architecture/monolithic-multi-module/product \
           architecture/monolithic-multi-module/payment \
           architecture/monolithic-multi-module/order \
           architecture/monolithic-multi-module/bootstrap \
           architecture/erp

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
