# Kratos Admin Official Layout Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有大仓后端完整对齐 go-kratos `kratos-layout` v2.9.2，并把 Cobra 运维工具从 Admin 服务入口拆到 `app/admin/cmd/tools`。

**Architecture:** Admin 应用在 `app/admin` 内拥有完整的 `biz/data/service/server/conf` 私有分层，服务入口采用官方 `flag + config + Wire + app.Run()`。Provider 作为 data adapter，后台任务作为 server adapter；Tools 作为同一应用边界内的独立 Cobra 可执行程序复用内部能力。

**Tech Stack:** Go 1.26、go-kratos v2.9.2、Wire、Cobra、GORM Gen、Goose、RabbitMQ。

## Global Constraints

- 保持 Go module `github.com/sleep-go/kratos-admin` 不变。
- 不升级 Kratos 主版本，不改变 API、数据库结构和 YAML 配置字段。
- 所有导出 Go 标识符保留或补充中文 GoDoc。
- 只调整本任务涉及的后端目录、构建部署配置和文档。
- Git Commit Message 使用中文。

---

### Task 1: 建立官方应用分层

**Files:**
- Move: `internal/biz/**` → `app/admin/internal/biz/**`
- Move: `internal/conf/**` → `app/admin/internal/conf/**`
- Move: `internal/data/**` → `app/admin/internal/data/**`
- Move: `internal/provider/**` → `app/admin/internal/data/provider/**`
- Move: `app/admin/internal/task/**` → `app/admin/internal/server/task/**`
- Modify: 全部 Go 文件、`conf.proto`、Buf 配置中的旧 import/go_package
- Move/Modify: `internal/architecture/dependency_test.go` → `app/admin/internal/architecture/dependency_test.go`

**Interfaces:**
- Consumes: 现有 `biz` 仓储接口、`data` 实现、`provider` factory 和 task Server。
- Produces: 模块内路径 `github.com/sleep-go/kratos-admin/app/admin/internal/...`，并由架构测试固定 `biz ← data/service` 依赖方向。

- [x] **Step 1: 写目录契约失败测试**

在架构测试中断言 Admin 的 data/biz/service/server 只能按 Kratos 依赖方向导入，并断言源文件不再引用 `github.com/sleep-go/kratos-admin/internal/`。

- [x] **Step 2: 运行测试确认旧结构失败**

Run: `GOCACHE=/tmp/go-build go test ./internal/architecture -count=1`

Expected: FAIL，指出旧根级 internal 或旧 import 仍存在。

- [x] **Step 3: 使用 git mv 迁移目录并机械更新 import**

迁移后将 provider import 统一为 `app/admin/internal/data/provider`，task import 统一为 `app/admin/internal/server/task`，配置生成包改为 `app/admin/internal/conf;conf`，GORM Gen 输出和 model package 改为 `app/admin/internal/data/{query,model}`。

- [x] **Step 4: 重新生成配置与 GORM 查询并运行分层测试**

Run: `make config && go run ./app/admin/cmd/tools gorm-gen`（Tools 在 Task 2 完成后执行）

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/... -count=1`

Expected: PASS。

### Task 2: 拆分官方 Server 与 Cobra Tools 入口

**Files:**
- Create: `app/admin/cmd/server/main.go`
- Create: `app/admin/cmd/server/wire.go`
- Generate: `app/admin/cmd/server/wire_gen.go`
- Create: `app/admin/cmd/tools/{main.go,root.go,run.go}`
- Move/Modify tests: `app/admin/cmd/kratos-admin/*_test.go` → `app/admin/cmd/tools/*_test.go`
- Delete after replacement: `app/admin/{app.go,logger.go,wire.go,wire_gen.go}`
- Delete after replacement: `app/admin/cmd/kratos-admin/**`
- Modify: `app/admin/internal/server/app.go`

**Interfaces:**
- Consumes: `conf.Bootstrap`、`conf.Config`、`data.Migrate`、`data.ProviderSet`、`provider.ProviderSet`、`server.ProviderSet` 和 task Server。
- Produces: server executable `./app/admin/cmd/server`; tools executable `./app/admin/cmd/tools` with `migrate`、`init-admin`、`gorm-gen`。

- [x] **Step 1: 先迁移并修改入口测试**

测试 Cobra 根命令只含三个工具子命令，并新增 Server 配置装配测试，确保 Server 入口不依赖 Cobra、配置加载错误会返回明确错误。

- [x] **Step 2: 运行测试确认新入口尚不存在**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/cmd/server ./app/admin/cmd/tools -count=1`

Expected: FAIL，缺少新入口或函数。

- [x] **Step 3: 按官方模板实现 main 和 Wire**

`main.go` 使用 `flag.StringVar(&flagconf, "conf", "./configs/config.yaml", ...)`、Kratos file source 扫描 `conf.Bootstrap`、标准 Logger、`wireApp` 和 `app.Run()`。`wireApp` 组合 data/provider/server provider sets 与 composition root 的 `provideServices`；启动迁移作为 Wire 构造前的显式步骤保留。

- [x] **Step 4: 实现独立 Tools Cobra 命令**

根命令 `Use: "kratos-admin-tools"`；`migrate` 和 `init-admin` 共享 `-c/--conf`；`gorm-gen` 默认输出 `./app/admin/internal/data/query`。移除服务子命令及其测试。

- [x] **Step 5: 生成 Wire 并验证两个入口**

Run: `GOCACHE=/tmp/go-build go tool wire ./app/admin/cmd/server`

Run: `GOCACHE=/tmp/go-build go test ./app/admin/cmd/server ./app/admin/cmd/tools -count=1`

Run: `GOCACHE=/tmp/go-build go build ./app/admin/cmd/server ./app/admin/cmd/tools`

Expected: 全部 PASS。

### Task 3: 更新生成、构建和部署链路

**Files:**
- Modify: `Makefile`
- Modify: `buf.gen.config.yaml`
- Modify: `deploy/Dockerfile.backend`
- Modify: `docker-compose.yml`
- Modify: `.dockerignore`（如旧路径存在）

**Interfaces:**
- Consumes: Task 2 的两个独立入口。
- Produces: `bin/kratos-admin`、`bin/kratos-admin-tools` 和可构建的 `admin-server`、`init-admin` images。

- [x] **Step 1: 更新 Makefile 的包集合与命令**

后端包集合收敛为 `./app/admin/...`；`wire` 指向 `cmd/server`；服务命令指向 `cmd/server`；工具命令指向 `cmd/tools`；`build` 生成两个二进制。

- [x] **Step 2: 更新 Docker 多阶段构建与 Compose 命令**

服务镜像默认执行 `/usr/local/bin/kratos-admin -conf ...`；初始化镜像执行 `/usr/local/bin/kratos-admin-tools init-admin --conf ...`，不再依赖旧统一入口。

- [x] **Step 3: 验证生成和部署配置**

Run: `make config && make wire && make gorm-gen`

Run: `docker compose config --quiet`

Expected: 命令退出码均为 0。

### Task 4: 文档、全量验证与提交

**Files:**
- Modify: `README.md`
- Modify: `docs/acceptance.md`
- Modify: `docs/deployment.md`
- Modify: 仍将旧路径描述为当前状态的设计文档

**Interfaces:**
- Consumes: 新目录、命令和构建方式。
- Produces: 与实际仓库一致的开发、部署和验收文档。

- [x] **Step 1: 更新目录树和所有可执行命令**

README 明确依赖服务、迁移、初始化、服务和前端的无 Docker 启动顺序；部署及验收文档使用新路径和两个二进制名称。

- [x] **Step 2: 搜索旧路径残留**

Run: `rg -n 'app/admin/cmd/kratos-admin|github.com/sleep-go/kratos-admin/internal/' --glob '!docs/superpowers/plans/**' --glob '!docs/superpowers/specs/**' .`

Expected: 无当前代码或当前文档残留。

- [x] **Step 3: 运行后端完整验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/... -count=1`

Run: `GOCACHE=/tmp/go-build go vet ./app/admin/...`

Run: `make build`

Expected: 全部退出码为 0。

- [x] **Step 4: 检查生成一致性**

Run: `make config && make wire && make gorm-gen && git diff --exit-code -- app/admin/internal/conf app/admin/internal/data/query app/admin/cmd/server/wire_gen.go`

Expected: 无生成差异。

- [x] **Step 5: 提交并推送 main**

```bash
git add app/admin api configs migrations Makefile buf.gen.config.yaml deploy/Dockerfile.backend docker-compose.yml README.md docs
git commit -m "重构：按官方 Kratos Layout 整理后台"
git push origin main
```
