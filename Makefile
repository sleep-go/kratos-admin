GOHOSTOS := $(shell go env GOHOSTOS)
GOPATH := $(shell go env GOPATH)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo unknown)
GOCACHE ?= /tmp/go-build
GO_APP_PACKAGES := ./app/admin/... ./app/worker/...
GO_PACKAGES := $(GO_APP_PACKAGES) ./internal/...

ifeq ($(GOHOSTOS),windows)
	GIT_BASH := $(subst \,/,$(subst cmd\git.exe,bin\bash.exe,$(shell where git)))
	SHELL := $(GIT_BASH)
endif

.PHONY: init
init: ## 安装 Proto、Wire 与迁移工具
	go install github.com/bufbuild/buf/cmd/buf@v1.66.0
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@v2.9.2
	go install github.com/go-kratos/kratos/cmd/protoc-gen-openapi/v2@v2.9.2
	go install github.com/google/wire/cmd/wire@v0.7.0
	go install github.com/pressly/goose/v3/cmd/goose@v3.26.0

.PHONY: config
config: ## 生成 internal/conf 配置代码
	buf generate --template buf.gen.config.yaml

.PHONY: api
api: ## 检查并生成业务 API、gRPC、HTTP 与 OpenAPI 代码
	buf lint
	buf generate

.PHONY: build
build: ## 构建统一的 kratos-admin 命令行到 bin
	mkdir -p bin
	GOCACHE=$(GOCACHE) go build -trimpath -o bin/kratos-admin ./app/admin/cmd/kratos-admin

.PHONY: generate
generate: ## 执行 Go Generate、GORM Gen 并校验模块依赖
	GOCACHE=$(GOCACHE) go generate $(GO_PACKAGES)
	$(MAKE) gorm-gen
	go mod verify

.PHONY: all
all: ## 生成 API、配置及依赖注入代码
	$(MAKE) api
	$(MAKE) config
	$(MAKE) generate

.PHONY: wire
wire: ## 生成 Admin 与 Worker 的 Wire 依赖注入代码
	GOCACHE=$(GOCACHE) go tool wire ./app/admin ./app/worker

.PHONY: gorm-gen
gorm-gen: ## 生成 GORM Gen 类型安全查询代码
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin gorm-gen

.PHONY: migrate
migrate: ## 使用 Goose 执行数据库迁移
	goose -dir migrations mysql "$(KRATOS_ADMIN_MYSQL_DSN)" up

.PHONY: init-admin
init-admin: ## 幂等初始化平台超级管理员
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin init-admin --conf ./configs/admin.yaml

.PHONY: run-admin
run-admin: ## 启动 Admin HTTP/gRPC 服务
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin server --conf ./configs/admin.yaml

.PHONY: run-worker
run-worker: ## 启动异步任务 Worker
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin worker --conf ./configs/worker.yaml

.PHONY: test
test: ## 运行后端测试
	GOCACHE=$(GOCACHE) go test -race $(GO_PACKAGES)

.PHONY: vet
vet: ## 运行后端静态检查
	GOCACHE=$(GOCACHE) go vet $(GO_PACKAGES)

.PHONY: frontend-install
frontend-install: ## 安装前端依赖
	cd app/frontend && pnpm install

.PHONY: frontend-test
frontend-test: ## 运行前端组件测试
	cd app/frontend && pnpm test:run

.PHONY: frontend-build
frontend-build: ## 检查并构建前端
	cd app/frontend && pnpm build

.PHONY: frontend-e2e
frontend-e2e: ## 运行前端端到端测试
	cd app/frontend && pnpm e2e

.PHONY: compose-config
compose-config: ## 校验 Docker Compose 配置
	docker compose --env-file .env config --quiet

.PHONY: compose-deps-up
compose-deps-up: ## 仅启动本地 MySQL、Redis 与 Mailpit 依赖
	docker compose --env-file .env up -d mysql redis mailpit

.PHONY: compose-up
compose-up: ## 构建并启动完整 Docker Compose 环境
	docker compose --env-file .env up --build

.PHONY: compose-down
compose-down: ## 停止 Docker Compose 环境
	docker compose --env-file .env down

# 兼容原有命令名称。
.PHONY: backend-test backend-vet backend-build
backend-test: test
backend-vet: vet
backend-build: build

.PHONY: help
help: ## 显示帮助
	@echo "Kratos Admin $(VERSION)"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
