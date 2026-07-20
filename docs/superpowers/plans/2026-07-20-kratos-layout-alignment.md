# Kratos Layout 官方规范对齐 Implementation Plan

> 后续变更：用户已将“四个独立可执行程序”调整为单一 `kratos-admin` Cobra 入口；命令统一为 `server`、`worker`、`init-admin`、`gorm-gen` 子命令。本文后续出现的旧命令路径仅保留为实施历史。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留多应用大仓、Cobra 和 Kratos v2.9.2 的前提下，补齐官方 kratos-layout 的配置 Proto、YAML、`--conf` 和 Makefile 工作流。

**Architecture:** `internal/conf/conf.proto` 是 Bootstrap 配置真相源，Buf 生成 Go 类型；`conf.Load(path)` 使用 Kratos file/env Source 扫描配置并转换为现有 `conf.Config`，避免业务层大范围改签名。Admin、Worker 和 initadmin 通过 Cobra `--conf` 传入各自 YAML；Docker 内嵌 YAML，敏感值继续由环境变量替换。

**Tech Stack:** Go 1.26、go-kratos v2.9.2 config、Protobuf、Buf v2、Cobra、Wire、GORM Gen、Goose、Docker Compose。

## Global Constraints

- Go module 保持 `github.com/sleep-go/kratos-admin`，Kratos 保持 v2.9.2，不升级 v3。
- 保持 `app/admin`、`app/worker`、根级共享 `internal/biz` 与 `internal/data`。
- 业务 API、数据库迁移、RBAC、认证和前端 DTO 不变。
- `configs/*.yaml` 不写入真实密码、JWT 私钥或云 AccessKey。
- 全部命令继续使用 Cobra，`--conf` 默认值必须可从仓库根目录直接运行。
- 新增导出 Go 标识符必须有中文 GoDoc。
- 每个任务验证后使用中文提交。

---

### Task 1: 建立配置 Proto 与 YAML 真相源

**Files:**
- Create: `internal/conf/conf.proto`
- Create: `internal/conf/conf.pb.go`（生成）
- Create: `buf.gen.config.yaml`
- Create: `configs/admin.yaml`
- Create: `configs/worker.yaml`
- Modify: `buf.yaml`

**Interfaces:**
- Produces: `conf.Bootstrap`、`conf.Server`、`conf.Data`、`conf.Auth`、`conf.Storage`、`conf.Messaging` 生成类型。
- YAML 使用 `${KRATOS_ADMIN_*:default}`，保持当前环境变量契约。

- [ ] **Step 1: 添加配置契约生成检查并确认失败**

在 `internal/conf/config_test.go` 增加编译期测试，构造 `Bootstrap{Server: &Server{Http: &Server_HTTP{Addr: ":8000"}}}` 并断言字段；运行：

```bash
GOCACHE=/tmp/go-build go test ./internal/conf -run TestBootstrapGeneratedContract -count=1
```

Expected: FAIL，提示 `Bootstrap` 或生成类型未定义。

- [ ] **Step 2: 创建 conf.proto、Buf 配置与 YAML**

`Bootstrap` 完整覆盖当前 `Config` 六个分组；HTTP/gRPC、认证 TTL、Redis timeout 使用 `google.protobuf.Duration`。创建官方语义生成模板：

```yaml
version: v2
inputs:
  - directory: internal
plugins:
  - local: ["go", "run", "google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"]
    out: internal
    opt: paths=source_relative
```

- [ ] **Step 3: 生成并验证契约**

```bash
buf generate --template buf.gen.config.yaml
GOCACHE=/tmp/go-build go test ./internal/conf -run TestBootstrapGeneratedContract -count=1
```

Expected: 生成 `internal/conf/conf.pb.go`，测试 PASS。

- [ ] **Step 4: 提交配置契约**

```bash
git add internal/conf/conf.proto internal/conf/conf.pb.go internal/conf/config_test.go buf.gen.config.yaml buf.yaml configs
git commit -m "配置：建立Kratos配置契约与YAML"
```

### Task 2: 使用 Kratos Config 加载并转换配置

**Files:**
- Modify: `internal/conf/config.go`
- Modify: `internal/conf/config_test.go`

**Interfaces:**
- Produces: `func Load(path string) (Config, error)`。
- Removes runtime usage of: `LoadFromEnv()`；环境变量通过 YAML placeholder 继续兼容。

- [ ] **Step 1: 添加失败测试**

测试使用 `t.TempDir()` 写入最小 YAML，调用 `Load(path)` 并断言 MySQL、Redis、HTTP 和 TTL；另用 `t.Setenv` 断言 `${KRATOS_ADMIN_HTTP_ADDR:...}` 被覆盖，并覆盖缺失文件、生产密钥缺失。

```bash
GOCACHE=/tmp/go-build go test ./internal/conf -run 'TestLoad' -count=1
```

Expected: FAIL，提示 `Load` 未定义或仍未读取 YAML。

- [ ] **Step 2: 实现最小 Loader**

`Load` 使用：

```go
c := config.New(config.WithSource(file.NewSource(path), env.NewSource()))
if err := c.Load(); err != nil { /* 中文上下文 */ }
var bootstrap Bootstrap
if err := c.Scan(&bootstrap); err != nil { /* 中文上下文 */ }
```

将生成类型转换为现有值对象；缺少 duration 时使用原安全默认值，敏感值校验保持现有语义。

- [ ] **Step 3: 验证并清理旧环境解析辅助函数**

```bash
GOCACHE=/tmp/go-build go test ./internal/conf -count=1
```

Expected: 全部 PASS；`config.go` 不再自行逐项读取 `os.Getenv`。

- [ ] **Step 4: 提交 Loader**

```bash
git add internal/conf
git commit -m "配置：接入Kratos YAML加载器"
```

### Task 3: 为三个运行命令接入 Cobra --conf

**Files:**
- Modify: `app/admin/cmd/server/main.go`
- Modify: `app/admin/cmd/server/main_test.go`
- Modify: `app/worker/cmd/worker/main.go`
- Modify: `app/worker/cmd/worker/main_test.go`
- Modify: `app/admin/cmd/initadmin/main.go`
- Modify: `app/admin/cmd/initadmin/main_test.go`

**Interfaces:**
- Admin/Worker runner: `func(context.Context, string) error`，第二参数是配置路径。
- initadmin options 新增 `Conf string`。

- [ ] **Step 1: 修改命令测试并确认失败**

Admin 与 Worker 执行 `--conf ./testdata/config.yaml`，断言 runner 收到路径；initadmin 同样断言 `options.Conf`。运行：

```bash
GOCACHE=/tmp/go-build go test ./app/admin/cmd/server ./app/admin/cmd/initadmin ./app/worker/cmd/worker -count=1
```

Expected: FAIL，当前命令没有 `--conf` 或 runner 签名不匹配。

- [ ] **Step 2: 接入 Load(confPath)**

默认路径：Admin/initadmin 为 `./configs/admin.yaml`，Worker 为 `./configs/worker.yaml`。所有加载错误使用命令名和 `%w` 包装。

- [ ] **Step 3: 验证命令和后端回归**

```bash
GOCACHE=/tmp/go-build go test ./app/admin/cmd/server ./app/admin/cmd/initadmin ./app/worker/cmd/worker ./internal/conf -count=1
```

Expected: PASS。

- [ ] **Step 4: 提交命令接入**

```bash
git add app/admin/cmd/server app/admin/cmd/initadmin app/worker/cmd/worker
git commit -m "重构：统一命令配置入口"
```

### Task 4: 恢复官方 Makefile 语义并收口部署文档

**Files:**
- Modify: `Makefile`
- Modify: `deploy/Dockerfile.backend`
- Modify: `deploy/docker-compose.yml`
- Modify: `README.md`
- Modify: `docs/deployment.md`
- Modify: `docs/acceptance.md`
- Modify: `.env.example`

**Interfaces:**
- Standard targets: `init config api build generate all help`。
- Extensions: `wire gorm-gen migrate test vet frontend-* compose-*`。

- [ ] **Step 1: 添加 Makefile 契约检查并确认失败**

运行 `make help`、`make config`、`make build`；当前 `help/config/build` 不存在或语义不符合官方模板，记录 RED。

- [ ] **Step 2: 重写 Makefile 并更新 Docker**

以官方 Makefile 的变量、注释式帮助和默认 `help` 为骨架；`build` 输出四个命令到 `bin/`。Docker 复制 `configs`、设置 `/app` 工作目录，并显式传入对应 `--conf`。

- [ ] **Step 3: 同步启动和生成文档**

所有宿主机命令改为：

```bash
go run ./app/admin/cmd/server --conf ./configs/admin.yaml
go run ./app/worker/cmd/worker --conf ./configs/worker.yaml
```

说明 YAML 管理结构配置、环境变量只覆盖部署值和敏感值。

- [ ] **Step 4: 完整验证**

```bash
make all
make build
make test
make vet
cd frontend && pnpm lint && pnpm typecheck && pnpm test:run && pnpm build
docker compose --env-file .env -f deploy/docker-compose.yml config --quiet
docker compose --env-file .env -f deploy/docker-compose.yml build migrate init-admin api worker frontend
```

Expected: 全部退出码 0；重复执行 `make config make api make wire make gorm-gen` 后无意外生成差异。

- [ ] **Step 5: 提交并推送**

```bash
git add Makefile deploy README.md docs .env.example
git commit -m "构建：对齐Kratos Layout工程工作流"
git push origin main
```
