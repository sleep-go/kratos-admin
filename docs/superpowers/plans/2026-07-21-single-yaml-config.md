# Kratos Admin Single YAML Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Admin Server 收口为只从完整 YAML 文件读取运行配置的单应用，彻底移除 `.env`、系统环境变量覆盖和 Admin/Worker 双配置。

**Architecture:** `internal/conf/conf.proto` 继续定义唯一配置结构，Kratos file source 将 `configs/config.yaml`（宿主机开发）或 `configs/config.docker.yaml`（Compose）解析为共享配置。统一 Cobra 可执行程序负责 server、migrate、init-admin 和 gorm-gen；Worker 合并后的后台任务随 server 启动，Goose 迁移和管理员初始化也从同一 YAML 读取所需值。

**Tech Stack:** Go 1.26、go-kratos config、Protobuf、Cobra、Goose、GORM、Docker Compose、Buf、Go test

## Global Constraints

- 实施前必须确认另一个会话的 Worker 合并提交已经位于当前分支，且 `server` 已负责启动 API 与后台任务。
- 只保留一个应用配置契约，不得重新引入 Admin/Worker 两套配置。
- `configs/config.yaml` 面向宿主机，依赖地址使用 `127.0.0.1`；`configs/config.docker.yaml` 面向 Compose，依赖地址使用服务名 `mysql`、`redis`、`mailpit`。
- 应用不得读取 `.env` 或 `KRATOS_ADMIN_*` 系统环境变量；`--conf` 是环境切换的唯一入口。
- Goose 是唯一迁移入口，禁止 GORM AutoMigrate。
- 仓库 YAML 只包含可启动的开发值，不得提交真实生产数据库密码、JWT 私钥、OSS、短信或邮件凭据。
- 不覆盖、回滚或重新实现另一个会话已经完成的 Worker 合并代码。

---

### Task 1: 确认 Worker 合并基线与单配置契约

**Files:**
- Modify: `internal/conf/conf.proto`
- Modify: `internal/conf/config.go`
- Modify: `internal/conf/config_test.go`
- Modify: `internal/architecture/dependency_test.go`
- Generate: `internal/conf/conf.pb.go`

**Interfaces:**
- Consumes: Worker 合并后的 `app/admin.NewApplication(context.Context, conf.Config)`，其 `Run()` 同时启动 API 与后台任务。
- Produces: `conf.Config.Setup setup.Admin`、`conf.Data.MigrationsDir string`、`conf.Load(path string) (conf.Config, error)`；加载器只接受 YAML 文件值。

- [ ] **Step 1: 验证 Worker 合并前置条件**

Run:

```bash
test ! -d app/worker
! rg -n 'newWorkerCommand|runWorker|configs/worker.yaml' app internal Makefile docker-compose.yml
rg -n 'asynq|Worker|background' app/admin internal
```

Expected: 前两条退出码为 0；最后一条能定位 Admin Server 内的后台任务装配。如果前置条件不满足，停止实施并等待另一个会话完成，不修改其文件。

- [ ] **Step 2: 编写失败的配置契约测试**

在 `internal/conf/config_test.go` 增加测试，明确 Setup 和迁移目录必须来自 YAML，且环境变量不能覆盖文件值：

```go
func TestLoadReadsSetupAndMigrationFromYAML(t *testing.T) {
	t.Setenv("KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD", "EnvironmentPassword!2026")
	path := writeConfig(t, `
data:
  database:
    source: user:password@tcp(127.0.0.1:3306)/admin
    migrations_dir: migrations
setup:
  admin:
    username: admin
    display_name: 超级管理员
    email: admin@example.local
    phone: "13800000000"
    initial_password: YAMLPassword!2026
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Data.MigrationsDir != "migrations" {
		t.Fatalf("MigrationsDir = %q", cfg.Data.MigrationsDir)
	}
	if cfg.Setup.Admin.InitialPassword != "YAMLPassword!2026" {
		t.Fatalf("InitialPassword = %q", cfg.Setup.Admin.InitialPassword)
	}
}
```

将 `TestLoadResolvesEnvironmentPlaceholders` 替换为：

```go
func TestLoadDoesNotReadEnvironmentOverrides(t *testing.T) {
	t.Setenv("SERVER_HTTP_ADDR", "127.0.0.1:28000")
	path := writeConfig(t, "server:\n  http:\n    addr: 127.0.0.1:18000\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.HTTPAddr != "127.0.0.1:18000" {
		t.Fatalf("HTTPAddr = %q", cfg.Server.HTTPAddr)
	}
}
```

在 `internal/architecture/dependency_test.go` 的入口契约中增加：

```go
assertPathExists(t, "../../configs/config.yaml")
assertPathExists(t, "../../configs/config.docker.yaml")
assertPathMissing(t, "../../configs/admin.yaml")
assertPathMissing(t, "../../configs/worker.yaml")
assertPathMissing(t, "../../.env.example")
```

- [ ] **Step 3: 运行测试确认失败原因正确**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/conf ./internal/architecture -count=1
```

Expected: FAIL，错误指向 `Setup`、`MigrationsDir` 尚不存在以及新 YAML 尚未创建。

- [ ] **Step 4: 扩展 Protobuf 配置契约**

在 `Bootstrap` 中增加字段：

```proto
SetupConfig setup = 7;
```

在 `DataConfig.Database` 中增加：

```proto
string migrations_dir = 3;
```

新增：

```proto
message SetupConfig {
  message Admin {
    string username = 1;
    string display_name = 2;
    string email = 3;
    string phone = 4;
    string initial_password = 5;
  }
  Admin admin = 1;
}
```

Run:

```bash
make config
```

Expected: `internal/conf/conf.pb.go` 重新生成且包含 `SetupConfig`、`GetMigrationsDir()`。

- [ ] **Step 5: 将加载器改为纯 file source**

从 `internal/conf/config.go` 删除 `config/env` import，并将加载器改为：

```go
source := config.New(config.WithSource(configfile.NewSource(path)))
```

增加领域配置：

```go
type Setup struct {
	Admin AdminSetup
}

type AdminSetup struct {
	Username        string
	DisplayName     string
	Email           string
	Phone           string
	InitialPassword string
}
```

为 `Data` 增加 `MigrationsDir string`，为 `Config` 增加 `Setup Setup`；`fromBootstrap` 映射 YAML 值，并仅为非敏感结构值提供代码默认值：迁移目录 `migrations`、用户名 `admin`、显示名称 `超级管理员`。初始密码不得提供代码默认值。

- [ ] **Step 6: 创建两个完整 YAML 使契约测试转绿**

创建 `configs/config.yaml` 和 `configs/config.docker.yaml`。两者字段完整且不包含 `${...}`；差异只允许出现在网络地址和容器文件路径。初始化段固定包含：

```yaml
setup:
  admin:
    username: admin
    display_name: 超级管理员
    email: admin@example.local
    phone: ""
    initial_password: ChangeMe!2026
```

宿主机数据库段：

```yaml
data:
  database:
    driver: mysql
    source: kratos:kratos@tcp(127.0.0.1:3306)/kratos_admin?charset=utf8mb4&parseTime=True&loc=Local
    migrations_dir: migrations
  redis:
    network: tcp
    addr: 127.0.0.1:6379
```

Docker 数据库段将主机名改为 `mysql`、`redis`，本地存储路径改为 `/data/files`，SMTP 地址改为 `mailpit:1025`。

- [ ] **Step 7: 运行测试并提交**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/conf ./internal/architecture -count=1
git diff --check
```

Expected: PASS，且没有空白错误。

Commit:

```bash
git add internal/conf configs internal/architecture/dependency_test.go
git commit -m "配置：建立单应用YAML配置契约"
```

---

### Task 2: 统一 Cobra 配置入口、迁移和管理员初始化

**Files:**
- Modify: `app/admin/cmd/kratos-admin/root.go`
- Modify: `app/admin/cmd/kratos-admin/root_test.go`
- Modify: `app/admin/cmd/kratos-admin/run.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: `conf.Config.Data.MySQLDSN`、`conf.Config.Data.MigrationsDir`、`conf.Config.Setup.Admin`。
- Produces: Cobra `server --conf`、`migrate --conf`、`init-admin --conf`，三者默认 `./configs/config.yaml`；`runMigrate(context.Context, string) error` 和 `runInitAdmin(context.Context, string) error`。

- [ ] **Step 1: 编写失败的 Cobra 默认值测试**

调整 `commandRunners`，预期只包含 `server`、`migrate`、`initAdmin`、`gormGen`。增加测试分别执行三个命令，并让 runner 捕获配置路径：

```go
if got != "./configs/config.yaml" {
	t.Fatalf("conf = %q, want ./configs/config.yaml", got)
}
```

增加 `--conf /tmp/custom.yaml` 覆盖测试，预期 runner 收到 `/tmp/custom.yaml`；断言根命令不再包含 `worker` 子命令。

- [ ] **Step 2: 运行测试确认旧默认值和缺少 migrate 导致失败**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/cmd/kratos-admin -count=1
```

Expected: FAIL，错误指向 `admin.yaml`、缺少 migrate runner 或仍存在 worker 命令。

- [ ] **Step 3: 收口 Cobra 命令**

所有需要配置的命令使用常量：

```go
const defaultConfigPath = "./configs/config.yaml"
```

新增：

```go
func newMigrateCommand(run func(context.Context, string) error) *cobra.Command
```

其 `Use` 为 `migrate`，`Args` 为 `cobra.NoArgs`，`-c/--conf` 默认 `defaultConfigPath`。删除独立 Worker 命令；`init-admin` 只保留 `--conf`，管理员资料不再由 CLI flags 提供。

- [ ] **Step 4: 让 Goose 从 YAML 执行迁移**

将 Goose 作为已有技术栈的运行库加入模块：

```bash
go get github.com/pressly/goose/v3@v3.26.0
```

实现：

```go
func runMigrate(ctx context.Context, confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return fmt.Errorf("加载迁移配置失败: %w", err)
	}
	db, err := sql.Open("mysql", cfg.Data.MySQLDSN)
	if err != nil {
		return fmt.Errorf("创建迁移数据库连接失败: %w", err)
	}
	defer db.Close()
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("设置 Goose 方言失败: %w", err)
	}
	if err := goose.UpContext(ctx, db, cfg.Data.MigrationsDir); err != nil {
		return fmt.Errorf("执行 Goose 迁移失败: %w", err)
	}
	return nil
}
```

保留 MySQL driver 注册；不得调用 AutoMigrate。

- [ ] **Step 5: 将初始化管理员完全改为 YAML 输入**

将签名改为：

```go
func runInitAdmin(ctx context.Context, confPath string) error
```

删除 `os.Getenv` 和 `initAdminOptions`，把 `cfg.Setup.Admin` 映射为：

```go
setup.AdminInput{
	Username: cfg.Setup.Admin.Username,
	DisplayName: cfg.Setup.Admin.DisplayName,
	Email: cfg.Setup.Admin.Email,
	Phone: cfg.Setup.Admin.Phone,
	Password: cfg.Setup.Admin.InitialPassword,
}
```

由现有 `AdminInitializer.Ensure` 和密码校验返回缺失或弱密码错误；日志使用 YAML 中的用户名。

- [ ] **Step 6: 运行测试并提交**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/cmd/kratos-admin ./internal/biz/setup -count=1
GOCACHE=/tmp/go-build go vet ./app/admin/cmd/kratos-admin
```

Expected: PASS。

Commit:

```bash
git add app/admin/cmd/kratos-admin go.mod go.sum
git commit -m "命令：统一YAML迁移与初始化入口"
```

---

### Task 3: 收口 Makefile、Dockerfile 和根 Compose

**Files:**
- Modify: `Makefile`
- Modify: `docker-compose.yml`
- Modify: `deploy/Dockerfile.backend`
- Delete: `.env.example`
- Test: `internal/architecture/dependency_test.go`

**Interfaces:**
- Consumes: `kratos-admin migrate|init-admin|server --conf <path>`。
- Produces: 无外部 env 文件即可启动的完整 Compose；宿主机 Make 命令默认使用 `configs/config.yaml`。

- [ ] **Step 1: 扩展失败的文本契约测试**

在架构测试中读取 `Makefile` 和 `docker-compose.yml`，断言不包含：

```text
--env-file
env_file:
KRATOS_ADMIN_
configs/admin.yaml
configs/worker.yaml
```

并断言 Compose 不存在顶层 `worker:` 服务。测试必须以文件内容为准，不能只检查单个旧路径。

- [ ] **Step 2: 运行契约测试确认失败**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/architecture -run TestMonorepoEntrypoints -count=1
```

Expected: FAIL，输出列出当前 Makefile/Compose 的 env、旧配置或 Worker 服务残留。

- [ ] **Step 3: 修改 Makefile**

删除 `COMPOSE_ENV_FILE`、独立 `run-worker` 和外部 Goose CLI 安装依赖。目标统一为：

```make
migrate:
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin migrate --conf ./configs/config.yaml

init-admin:
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin init-admin --conf ./configs/config.yaml

run-admin:
	GOCACHE=$(GOCACHE) go run ./app/admin/cmd/kratos-admin server --conf ./configs/config.yaml

compose-config:
	docker compose config --quiet
```

`compose-deps-up`、`compose-up`、`compose-down` 同样直接执行根 Compose，不传 `--env-file`。

- [ ] **Step 4: 修改 Dockerfile**

删除仅安装 Goose CLI 的 `tools` stage。runtime 必须包含：

```dockerfile
COPY configs ./configs
COPY migrations ./migrations
```

保留 `admin-server` 和 `init-admin` target，新增从 runtime 继承且默认执行 `migrate` 的轻量 `migrate` target；三个默认命令分别使用 `/app/configs/config.docker.yaml`。不得保留 Worker target。

- [ ] **Step 5: 修改 Compose**

删除 `.env.example` 和全部 `env_file`。端口固定为 `3306:3306`、`6379:6379`、`8025:8025`、`8000:8000`、`9000:9000`、`8080:80`。

迁移、初始化和 API 命令分别为：

```yaml
command: ["migrate", "--conf", "/app/configs/config.docker.yaml"]
command: ["init-admin", "--conf", "/app/configs/config.docker.yaml"]
command: ["server", "--conf", "/app/configs/config.docker.yaml"]
```

删除独立 Worker 服务。MySQL 官方镜像所需的 `environment` 保留为 Compose YAML 内的固定字面量，它不是外部 env 注入；数据库用户名和密码必须与 `config.docker.yaml` 一致。

- [ ] **Step 6: 运行配置和镜像验证**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/architecture -count=1
make compose-config
docker compose build migrate init-admin api frontend
```

Expected: 全部退出码为 0；构建日志不读取 `.env`，不构建 Worker 镜像。

- [ ] **Step 7: 提交**

```bash
git add Makefile docker-compose.yml deploy/Dockerfile.backend internal/architecture/dependency_test.go .env.example
git commit -m "部署：移除环境变量与独立Worker入口"
```

---

### Task 4: 文档收口与最终验收

**Files:**
- Modify: `README.md`
- Modify: `docs/deployment.md`
- Modify: `docs/acceptance.md`
- Modify: `docs/superpowers/specs/2026-07-21-single-yaml-config-design.md`

**Interfaces:**
- Consumes: 最终 Cobra、Makefile、Compose 和两个环境 YAML。
- Produces: 与真实命令一致的启动、部署和验收说明。

- [ ] **Step 1: 更新设计文档中的环境文件结论**

将“只保留 `configs/config.yaml`”修正为“一套配置契约、两个完整环境值文件”：

```text
configs/config.yaml          宿主机开发
configs/config.docker.yaml   Docker Compose
```

明确这不是 Admin/Worker 双配置，两个文件仅解决宿主机与容器 DNS/路径差异。

- [ ] **Step 2: 重写启动文档**

README 和 deployment 文档必须给出可复制命令：

```bash
make compose-deps-up
make migrate
make init-admin
make run-admin
cd app/frontend && pnpm dev
```

完整容器启动只使用：

```bash
make compose-up
```

删除 `cp .env.example .env`、`export KRATOS_ADMIN_*`、独立 Worker 启动和旧 YAML 路径。说明生产环境复制并修改完整 YAML，通过 `--conf` 指定，且不得提交真实密钥。

- [ ] **Step 3: 扫描残留入口**

Run:

```bash
! rg -n 'KRATOS_ADMIN_|\.env.example|configs/admin.yaml|configs/worker.yaml|run-worker|compose.*--env-file' README.md docs Makefile docker-compose.yml configs app internal --glob '!docs/superpowers/plans/**' --glob '!docs/superpowers/specs/2026-07-21-single-yaml-config-design.md'
```

Expected: 退出码为 0。历史实施计划允许保留旧文本作为记录；当前设计文档必须已经修订。

- [ ] **Step 4: 执行完整生成与后端验收**

Run:

```bash
make all
make build
make test
make vet
git diff --exit-code -- internal/conf/conf.pb.go internal/data/query api docs/openapi
```

Expected: 全部退出码为 0；生成产物与仓库一致。

- [ ] **Step 5: 执行前端与 Compose 验收**

Run:

```bash
cd app/frontend
pnpm lint
pnpm typecheck
pnpm test:run
pnpm build
cd ../..
docker compose config --quiet
docker compose build migrate init-admin api frontend
```

Expected: 前端 13 个测试文件、22 项测试全部通过；Compose 配置和四个应用镜像构建成功。若测试数量因另一个会话的有效提交增加，以零失败为准。

- [ ] **Step 6: 提交文档并做只读代码审查**

```bash
git add README.md docs
git commit -m "文档：更新单配置启动与部署说明"
```

审查范围从实施前基线到当前 HEAD，重点检查：真实生产秘密、环境变量残留、Worker 双入口、宿主机/容器地址混用、Goose 唯一迁移入口和文档命令可执行性。Critical 与 Important 必须修复后重新运行相应验证。

- [ ] **Step 7: 合并与推送**

确认工作区干净、当前分支包含另一个会话的 Worker 合并提交和本计划全部提交。按用户已确认交付方式快进合并到 `main`（若实施直接发生在 `main`，则不重复合并），最终执行：

```bash
git status --short --branch
git push origin main
git rev-parse HEAD
git rev-parse origin/main
```

Expected: 工作区干净，`HEAD` 与 `origin/main` SHA 完全一致。
