.PHONY: test build vet tidy compose-up compose-down run

## 运行全部测试（含架构守护测试；依赖 MySQL 的集成测试在没有数据库时自动跳过）
test:
	go test ./...

build:
	go build ./...

vet:
	go vet ./...

tidy:
	go mod tidy

## 启动 monolithic 的 MySQL（Docker）
compose-up:
	docker compose -f architecture/monolithic/docker-compose.yml up -d

compose-down:
	docker compose -f architecture/monolithic/docker-compose.yml down

## 启动应用（会先拉起 MySQL）
run: compose-up
	go run ./architecture/monolithic/cmd/monolithic
