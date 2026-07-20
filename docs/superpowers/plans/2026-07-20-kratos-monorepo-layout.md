# Kratos Admin 大仓目录重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 `backend/*` 原子迁移为 go-kratos 大仓结构，以 `app/admin` 和 `app/worker` 交付两个独立应用、四个 Cobra 可执行程序，并用独立 Wire 注入图保持严格 `biz → data/service → server` 边界。

**Architecture:** Protobuf 契约继续位于 `api/admin/v1`，共享领域、配置、仓储和 Provider 迁至根级 `internal/*`；Admin 与 Worker 各自只保留应用专属的 `service`、`server` 和 `cmd`。Goose 是唯一迁移入口，GORM Gen 只生成查询代码；迁移不改变 API、数据库结构、前端行为和运行配置语义。

**Tech Stack:** Go 1.26、go-kratos v2、Google Wire v0.7.0、Cobra v1.10.1、GORM Gen、Goose、MySQL 8、Redis、Asynq、Casbin、Vue 3。

## Global Constraints

- Go module 保持 `github.com/sleep-go/kratos-admin`，不拆分子 module。
- 两个独立应用固定为 `app/admin` 和 `app/worker`。
- 四个独立入口固定为 `app/admin/cmd/server`、`app/admin/cmd/initadmin`、`app/admin/cmd/gormgen`、`app/worker/cmd/worker`，全部使用 `github.com/spf13/cobra`。
- Admin Server 与 Worker 各自维护并提交 `wire.go` 和 `wire_gen.go`；短生命周期的 initadmin、gormgen 不使用完整注入图。
- `internal/data` 不得导入任何 `service` 或 `server`；`internal/biz` 不得导入 `data`、应用 Service 或应用 Server；两个应用的 `internal` 不得交叉导入。
- `api/admin/v1` 的 Protobuf、生成 Go 和 OpenAPI 公开路径保持不变。
- Goose 是唯一迁移入口；服务启动和 Worker 启动禁止调用 GORM `AutoMigrate`。
- 现有五个迁移文件内容和版本号保持不变，仅从 `backend/migrations` 移至根级 `migrations`。
- 不保留 `backend/*` 兼容入口；验收完成时仓库中不存在 `backend/`。
- 不改变前端页面、接口 DTO、数据库表结构、权限规则、认证规则或任务业务语义。
- 所有新增导出 Go 标识符必须有以名称开头的中文 GoDoc；错误使用中文上下文并通过 `%w` 包装。
- 每个任务只提交该任务文件，Git Commit Message 使用中文。

---

## 文件结构锁定

本计划完成后的后端文件职责如下：

```text
api/admin/v1/                         # Protobuf 真相源与生成契约，路径不变
app/admin/cmd/server/                 # Cobra API 入口与 Admin Wire 注入器
app/admin/cmd/initadmin/              # Cobra 超级管理员初始化工具
app/admin/cmd/gormgen/                # Cobra GORM Gen 工具
app/admin/internal/service/           # Protobuf DTO、transport 上下文到 Biz 的适配
app/admin/internal/server/            # HTTP/gRPC 注册与 Kratos App 组装
app/worker/cmd/worker/                # Cobra Worker 入口与 Worker Wire 注入器
app/worker/internal/service/          # Asynq 任务载荷到 Biz 用例的适配
app/worker/internal/server/           # Asynq 路由、轮询和 Kratos 生命周期
internal/biz/management/contracts.go  # 管理资源可信作用域、分页、授权及仓储接口
internal/biz/audit/log.go             # API/登录日志记录及写入接口
internal/biz/{auth,file,...}/          # 现有领域用例
internal/conf/                         # 环境配置
internal/data/                         # MySQL、Redis、GORM Gen、仓储实现
internal/provider/factory.go           # 密钥、消息、存储 Provider 运行时组装
internal/provider/{message,secret,storage}/
internal/architecture/dependency_test.go # 分层依赖守卫
migrations/                            # Goose SQL
deploy/Dockerfile.backend              # Go 多阶段、多 target 镜像
```

共享构造接口在后续任务中固定为：

```go
// internal/data/data.go
func NewData(ctx context.Context, cfg conf.Config) (*Data, func(), error)

// internal/provider/factory.go
type AdminSet struct {
	PrivateKey    ed25519.PrivateKey
	MessageSenders []message.Sender
	Storage       storage.Provider
	LocalStorage  http.Handler
	ConfigCipher  *secret.Cipher
}
func NewAdminSet(cfg conf.Config) (*AdminSet, error)

type WorkerSet struct {
	Storage storage.Provider
}
func NewWorkerSet(cfg conf.Config) (*WorkerSet, error)

// app/admin/internal/service/services.go
type Services struct {
	Health     *HealthService
	Auth       *AuthService
	Management *ManagementService
	File       *FileService
	Log        *LogService
}
func NewServices(cfg conf.Config, resources *data.Data, providers *provider.AdminSet) (*Services, error)

// app/admin/internal/server/app.go
func NewApp(httpServer *khttp.Server, grpcServer *kgrpc.Server, logger log.Logger) *kratos.App

// app/worker/internal/service/tasks.go
type Service struct { /* 私有依赖 */ }
func NewService(resources *data.Data, providers *provider.WorkerSet, logger log.Logger) *Service
func (s *Service) HandleAudit(ctx context.Context, task *asynq.Task) error
func (s *Service) HandleLogExport(ctx context.Context, task *asynq.Task) error
func (s *Service) HandleFileCleanup(ctx context.Context, task *asynq.Task) error
func (s *Service) EnqueuePending(ctx context.Context, now time.Time) error

// app/worker/internal/server/server.go
func NewServer(cfg conf.Config, tasks *service.Service, logger log.Logger) *Server
func NewApp(server *Server, logger log.Logger) *kratos.App
```

### Task 1: 建立共享根目录并保持共享包测试通过

**Files:**
- Move: `backend/internal/biz/**` → `internal/biz/**`
- Move: `backend/internal/conf/**` → `internal/conf/**`
- Move: `backend/internal/data/**` → `internal/data/**`
- Move: `backend/internal/provider/**` → `internal/provider/**`
- Move: `backend/migrations/**` → `migrations/**`
- Modify: all moved `*.go` imports

**Interfaces:**
- Consumes: 当前 `backend/internal/{biz,conf,data,provider}` 和五个 Goose SQL。
- Produces: import 前缀 `github.com/sleep-go/kratos-admin/internal/{biz,conf,data,provider}`；迁移目录 `migrations`。

- [ ] **Step 1: 记录迁移前基线**

Run:

```bash
GOCACHE=/tmp/go-build go test ./backend/internal/biz/... ./backend/internal/conf/... ./backend/internal/provider/...
```

Expected: 所有包输出 `ok` 或 `[no test files]`，退出码为 0。

- [ ] **Step 2: 使用 Git 移动共享包和迁移文件**

Run:

```bash
mkdir -p internal migrations
git mv backend/internal/biz internal/biz
git mv backend/internal/conf internal/conf
git mv backend/internal/data internal/data
git mv backend/internal/provider internal/provider
git mv backend/migrations/00001_init_schema.sql migrations/00001_init_schema.sql
git mv backend/migrations/00002_auth_security.sql migrations/00002_auth_security.sql
git mv backend/migrations/00003_log_exports.sql migrations/00003_log_exports.sql
git mv backend/migrations/00004_file_cleanup_status.sql migrations/00004_file_cleanup_status.sql
git mv backend/migrations/00005_builtin_navigation.sql migrations/00005_builtin_navigation.sql
```

Expected: `git status --short` 将上述路径显示为 rename，不出现 SQL 内容修改。

- [ ] **Step 3: 机械替换共享 import 前缀**

Run:

```bash
rg -l 'github.com/sleep-go/kratos-admin/backend/internal/(biz|conf|data|provider)' --glob '*.go' . | xargs perl -pi -e 's#github\.com/sleep-go/kratos-admin/backend/internal/(biz|conf|data|provider)#github.com/sleep-go/kratos-admin/internal/$1#g'
gofmt -w internal backend
```

Expected: `rg 'github.com/sleep-go/kratos-admin/backend/internal/(biz|conf|data|provider)' --glob '*.go' .` 无输出。

- [ ] **Step 4: 验证共享包移动未改变行为**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/biz/... ./internal/conf/... ./internal/provider/...
git diff --cached --summary --find-renames=100% -- backend/migrations migrations
```

Expected: Go 测试通过；第二条命令将五个 SQL 全部显示为 `rename ... (100%)`，不得出现低于 100% 的相似度。

- [ ] **Step 5: 提交共享根目录迁移**

```bash
git add internal migrations backend
git commit -m "重构：迁移共享领域与基础设施目录"
```

### Task 2: 将 Data 反向依赖的契约下沉到 Biz

**Files:**
- Create: `internal/biz/management/contracts.go`
- Create: `internal/biz/audit/log.go`
- Create: `internal/architecture/dependency_test.go`
- Modify: `app/admin/internal/service/management.go`（该文件在本任务先由 `backend/internal/service/management.go` 移入临时目标路径）
- Modify: `app/admin/internal/service/access.go`
- Modify: `app/admin/internal/service/auth.go`
- Modify: `app/admin/internal/service/file.go`
- Modify: `internal/data/management_repository.go`
- Modify: `internal/data/file_repository.go`
- Modify: `internal/data/log_repository.go`
- Modify: `internal/data/*_test.go`

**Interfaces:**
- Consumes: Service 中现有 `ResourceScope`、`PageQuery`、`RoleGrant`、`ManagementRepository`、`ManagementPermissionChecker`、`FileDataScopeChecker`、`AccessLogRecord`、`AccessLogRecorder`、`LoginLogRecord`、`LoginLogRecorder`。
- Produces: `management.Scope`、`management.PageQuery`、`management.RoleGrant`、`management.Repository`、`management.PermissionChecker`、`management.RecordChecker`；`audit.AccessLogRecord`、`audit.AccessLogRecorder`、`audit.LoginLogRecord`、`audit.LoginLogRecorder`。

- [ ] **Step 1: 先添加会失败的分层依赖测试**

Create `internal/architecture/dependency_test.go`:

```go
package architecture_test

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLayerDependencies(t *testing.T) {
	t.Parallel()
	assertNoImports(t, "../data", "/service", "/server", "/app/admin/internal", "/app/worker/internal")
	assertNoImports(t, "../biz", "/internal/data", "/internal/service", "/internal/server", "/app/admin/internal", "/app/worker/internal")
	assertNoImports(t, "../../app/admin", "/app/worker/internal")
	assertNoImports(t, "../../app/worker", "/app/admin/internal")
}

func assertNoImports(t *testing.T, root string, forbidden ...string) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			for _, fragment := range forbidden {
				if strings.Contains(importPath, fragment) {
					t.Errorf("%s 禁止导入 %s", path, importPath)
				}
			}
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("扫描 %s 失败: %v", root, err)
	}
}
```

- [ ] **Step 2: 运行测试并确认现有反向依赖被捕获**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/architecture -run TestLayerDependencies -v
```

Expected: FAIL，并至少列出 `internal/data/management_repository.go`、`internal/data/file_repository.go` 或 `internal/data/log_repository.go` 导入旧 Service。

- [ ] **Step 3: 创建 Biz 管理契约**

Create `internal/biz/management/contracts.go`，把现有方法签名原样迁入下列类型：

```go
// Package management 定义通用后台资源的领域契约。
package management

import "context"

// Scope 是由认证上下文派生的可信数据边界。
type Scope struct {
	TenantID      uint64
	UserID        uint64
	MemberID      uint64
	PlatformAdmin bool
}

// PageQuery 描述统一分页、排序、关键词与白名单筛选条件。
type PageQuery struct {
	Page     uint32
	PageSize uint32
	Keyword  string
	Sort     string
	Filters  map[string]string
}

// RoleGrant 描述角色对单个资源的动作集合。
type RoleGrant struct {
	ResourceCode string
	Actions      []string
}

// Repository 定义白名单后台资源的统一持久化接口。
type Repository interface {
	List(context.Context, Scope, string, PageQuery) ([]map[string]any, uint64, error)
	Create(context.Context, Scope, string, map[string]any) (uint64, error)
	Update(context.Context, Scope, string, uint64, map[string]any) error
	Delete(context.Context, Scope, string, uint64) error
	EffectiveSettings(context.Context, Scope, string) ([]map[string]any, error)
	TestProviderConnection(context.Context, Scope, uint64) error
	UpdateRoleAuthorization(context.Context, Scope, uint64, uint32, []RoleGrant, []uint64) error
	UpdateTenantFeatures(context.Context, Scope, uint64, []uint64) error
}

// PermissionChecker 定义后台资源动作的 Casbin 权限检查能力。
type PermissionChecker interface {
	Allowed(context.Context, Scope, string, string) (bool, error)
}

// RecordChecker 定义单条资源的数据范围复核能力。
type RecordChecker interface {
	AllowedRecord(context.Context, Scope, string, string) (bool, error)
}
```

- [ ] **Step 4: 创建 Biz 审计日志契约**

Create `internal/biz/audit/log.go`:

```go
package audit

import "context"

// AccessLogRecord 描述不会包含请求正文和敏感字段的 API 访问日志。
type AccessLogRecord struct {
	TenantID uint64
	UserID uint64
	RequestID string
	Method string
	Route string
	StatusCode int
	DurationMS uint32
	IP string
	UserAgent string
	ErrorReason string
}

// AccessLogRecorder 定义 API 访问日志持久化能力。
type AccessLogRecorder interface {
	RecordAccess(context.Context, AccessLogRecord) error
}

// LoginLogRecord 描述已脱敏的登录安全事件。
type LoginLogRecord struct {
	TenantID uint64
	UserID uint64
	Identifier string
	Result uint8
	Reason string
	IP string
	UserAgent string
	RequestID string
}

// LoginLogRecorder 定义登录安全日志持久化能力。
type LoginLogRecorder interface {
	RecordLogin(context.Context, LoginLogRecord) error
}
```

- [ ] **Step 5: 移动 Admin Service 并改用 Biz 契约**

Run:

```bash
mkdir -p app/admin/internal
git mv backend/internal/service app/admin/internal/service
perl -pi -e 's#github\.com/sleep-go/kratos-admin/backend/internal/service#github.com/sleep-go/kratos-admin/app/admin/internal/service#g' $(rg -l 'github.com/sleep-go/kratos-admin/backend/internal/service' --glob '*.go' .)
```

Then replace the Service-owned declarations and usages as follows:

```go
// app/admin/internal/service/management.go
import managementbiz "github.com/sleep-go/kratos-admin/internal/biz/management"

type ManagementService struct {
	v1.UnimplementedManagementServiceServer
	repository  managementbiz.Repository
	permissions managementbiz.PermissionChecker
}

func NewManagementService(repository managementbiz.Repository, checkers ...managementbiz.PermissionChecker) *ManagementService
```

All `ResourceScope`, `PageQuery`, and `RoleGrant` expressions in Admin Service become `managementbiz.Scope`, `managementbiz.PageQuery`, and `managementbiz.RoleGrant`. In `file.go`, `dataScope` becomes `managementbiz.RecordChecker`. In `access.go` and `auth.go`, recorder fields and record expressions become `auditbiz.AccessLogRecorder`, `auditbiz.AccessLogRecord`, `auditbiz.LoginLogRecorder`, and `auditbiz.LoginLogRecord` using import alias `auditbiz`.

- [ ] **Step 6: 改造 Data 实现并删除所有 Data → Service import**

In `internal/data/management_repository.go`, `internal/data/file_repository.go`, `internal/data/log_repository.go` and their tests, use these exact substitutions:

```text
service.ResourceScope              -> managementbiz.Scope
service.PageQuery                  -> managementbiz.PageQuery
service.RoleGrant                  -> managementbiz.RoleGrant
service.ManagementRepository       -> managementbiz.Repository
service.ManagementPermissionChecker -> managementbiz.PermissionChecker
service.AccessLogRecord            -> auditbiz.AccessLogRecord
service.AccessLogRecorder          -> auditbiz.AccessLogRecorder
service.LoginLogRecord             -> auditbiz.LoginLogRecord
service.LoginLogRecorder           -> auditbiz.LoginLogRecorder
```

Add compile-time assertions:

```go
var _ managementbiz.Repository = (*ManagementRepository)(nil)
var _ managementbiz.PermissionChecker = (*ManagementRepository)(nil)
var _ managementbiz.RecordChecker = (*ManagementRepository)(nil)
var _ auditbiz.AccessLogRecorder = (*AuthRepository)(nil)
var _ auditbiz.LoginLogRecorder = (*AuthRepository)(nil)
```

- [ ] **Step 7: 格式化并验证分层与相关回归测试**

Run:

```bash
gofmt -w internal app/admin/internal/service
GOCACHE=/tmp/go-build go test ./internal/architecture ./internal/data/... ./app/admin/internal/service/...
```

Expected: PASS；`rg 'app/admin/internal/service|/internal/service|/internal/server' internal/data internal/biz --glob '*.go'` 无输出。

- [ ] **Step 8: 提交领域契约下沉**

```bash
git add internal app/admin/internal/service
git commit -m "重构：下沉管理与日志领域契约"
```

### Task 3: 建立共享 Provider 构造与 Wire 友好的 Data 生命周期

**Files:**
- Create: `internal/provider/factory.go`
- Create: `internal/provider/factory_test.go`
- Modify: `internal/data/data.go`
- Delete through Task 4 move: `backend/internal/app/app.go`

**Interfaces:**
- Consumes: `conf.Config`、`data.Open`、现有 SMTP/阿里云短信/本地存储/OSS/secret 构造函数。
- Produces: 文件结构锁定章节中的 `data.NewData`、`provider.AdminSet`、`provider.WorkerSet`、`provider.NewAdminSet`、`provider.NewWorkerSet`。

- [ ] **Step 1: 为 Provider 默认行为写失败测试**

Create `internal/provider/factory_test.go`:

```go
package provider

import (
	"testing"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

func TestNewWorkerSetUsesLocalStorage(t *testing.T) {
	set, err := NewWorkerSet(conf.Config{Auth: conf.Auth{SecretKey: "test-secret"}, Storage: conf.Storage{Provider: "local", LocalPath: t.TempDir()}})
	if err != nil {
		t.Fatalf("NewWorkerSet() error = %v", err)
	}
	if set.Storage.Name() != "local" {
		t.Fatalf("Storage.Name() = %q, want local", set.Storage.Name())
	}
}
```

- [ ] **Step 2: 运行新增测试确认接口尚不存在**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/data ./internal/provider
```

Expected: FAIL，错误包含 `undefined: NewWorkerSet`。

- [ ] **Step 3: 为 Data 增加 Wire cleanup 构造器**

Modify `internal/data/data.go`:

```go
// NewData 创建共享数据资源，并返回供 Wire 传播的清理函数。
func NewData(ctx context.Context, cfg conf.Config) (*Data, func(), error) {
	resources, err := Open(ctx, cfg.Data)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = resources.Close() }
	return resources, cleanup, nil
}
```

- [ ] **Step 4: 从旧 App 组装代码提取 Provider 工厂**

Create `internal/provider/factory.go` with package `provider`. 将 `buildPrivateKey`、`buildMessageSenders` 和 `buildStorageProvider` 从 `backend/internal/app/app.go` 原样迁入，保持所有分支条件不变，并导出：

```go
// AdminSet 汇集 Admin Server 运行时 Provider。
type AdminSet struct {
	PrivateKey     ed25519.PrivateKey
	MessageSenders []message.Sender
	Storage        storage.Provider
	LocalStorage   http.Handler
	ConfigCipher   *secret.Cipher
}

// NewAdminSet 创建 Admin Server 所需的密钥、消息、存储和配置加密 Provider。
func NewAdminSet(cfg conf.Config) (*AdminSet, error) {
	privateKey, err := buildPrivateKey(cfg)
	if err != nil { return nil, err }
	senders, err := buildMessageSenders(cfg)
	if err != nil { return nil, err }
	storageProvider, localHandler, err := buildStorageProvider(cfg)
	if err != nil { return nil, err }
	configKey := sha256.Sum256([]byte(cfg.Auth.SecretKey + ":provider-config"))
	cipher, err := secret.NewCipher(configKey[:])
	if err != nil { return nil, fmt.Errorf("初始化 Provider 配置密钥失败: %w", err) }
	return &AdminSet{PrivateKey: privateKey, MessageSenders: senders, Storage: storageProvider, LocalStorage: localHandler, ConfigCipher: cipher}, nil
}

// WorkerSet 汇集 Worker 运行时 Provider。
type WorkerSet struct { Storage storage.Provider }

// NewWorkerSet 创建 Worker 所需的对象存储 Provider。
func NewWorkerSet(cfg conf.Config) (*WorkerSet, error) {
	storageProvider, _, err := buildStorageProvider(cfg)
	if err != nil { return nil, err }
	return &WorkerSet{Storage: storageProvider}, nil
}
```

Keep the helper bodies byte-for-byte equivalent apart from package-qualified imports, so development key generation, production key rejection, local signing and OSS configuration remain unchanged.

- [ ] **Step 5: 验证 Provider 和 Data 构造器**

Run:

```bash
gofmt -w internal/data/data.go internal/provider/factory.go internal/provider/factory_test.go
GOCACHE=/tmp/go-build go test ./internal/data ./internal/provider ./internal/provider/...
```

Expected: PASS。

- [ ] **Step 6: 提交共享运行时构造器**

```bash
git add internal/data internal/provider
git commit -m "重构：提供Wire数据与Provider构造器"
```

### Task 4: 完成 Admin Service、Server 与 Wire 注入图

**Files:**
- Create: `app/admin/internal/service/services.go`
- Move: `backend/internal/server/**` → `app/admin/internal/server/**`
- Create: `app/admin/internal/server/app.go`
- Create: `app/admin/internal/server/app_test.go`
- Create: `app/admin/cmd/server/logger.go`
- Create: `app/admin/cmd/server/wire.go`
- Generate: `app/admin/cmd/server/wire_gen.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Delete: `backend/internal/app/app.go`
- Delete: `backend/internal/app/app_test.go`

**Interfaces:**
- Consumes: `data.NewData`、`provider.NewAdminSet`、现有 Biz 用例和 Admin Service 构造器。
- Produces: `service.NewServices`、`server.NewApp`、`wireAdminApp(context.Context, conf.Config) (*kratos.App, func(), error)`。

- [ ] **Step 1: 添加 Wire/Cobra 依赖**

Run:

```bash
go get github.com/spf13/cobra@v1.10.1
go get -tool github.com/google/wire/cmd/wire@v0.7.0
```

Expected: `go.mod` 将 Cobra 列为 direct dependency，并含 `tool github.com/google/wire/cmd/wire`；`go mod tidy` 不删除二者。

- [ ] **Step 2: 为 Admin 应用组装写失败测试**

Create `app/admin/internal/server/app_test.go`:

```go
package server

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

func TestNewApp(t *testing.T) {
	application := NewApp(khttp.NewServer(), kgrpc.NewServer(), log.NewStdLogger(io.Discard))
	if application == nil {
		t.Fatal("NewApp() = nil")
	}
}
```

- [ ] **Step 3: 移动 Admin Server 并统一一次性注册服务**

Run:

```bash
git mv backend/internal/server app/admin/internal/server
perl -pi -e 's#github\.com/sleep-go/kratos-admin/backend/internal/server#github.com/sleep-go/kratos-admin/app/admin/internal/server#g' $(rg -l 'github.com/sleep-go/kratos-admin/backend/internal/server' --glob '*.go' .)
```

Change constructors to consume the full service set:

```go
func NewHTTPServer(cfg conf.Config, services *service.Services, providers *provider.AdminSet) *khttp.Server
func NewGRPCServer(cfg conf.Config, services *service.Services) *kgrpc.Server
```

Both register Health, Auth, Management, File and Log unconditionally from `services`. `NewHTTPServer` additionally registers `/api/v1/files/local/content` only when `providers.LocalStorage != nil`. Delete the four `Register*HTTP` and four `Register*GRPC` functions after their registrations are folded into constructors.

- [ ] **Step 4: 创建 Admin Service 聚合构造器**

Create `app/admin/internal/service/services.go` by moving the dependency assembly from `app.NewAPIResources` into:

```go
// Services 汇集 Admin Server 注册的全部 gRPC/HTTP 服务。
type Services struct {
	Health     *HealthService
	Auth       *AuthService
	Management *ManagementService
	File       *FileService
	Log        *LogService
}

// NewServices 创建 Admin Server 的认证、管理、文件和日志服务。
func NewServices(cfg conf.Config, resources *data.Data, providers *provider.AdminSet) (*Services, error) {
	repository := data.NewAuthRepository(resources)
	tokenManager := bizauth.NewTokenManager(providers.PrivateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, nil)
	hasher := bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams())
	loginUsecase := bizauth.NewLoginUsecase(repository, repository, hasher, tokenManager, nil)
	sessionUsecase := bizauth.NewSessionUsecase(repository, tokenManager, nil)
	authService := NewAuthService(loginUsecase, cfg.Environment == "production", sessionUsecase)
	authService.ConfigureAccessSecurity(tokenManager, sessionUsecase)
	authService.ConfigureAccessLog(repository)
	authService.ConfigureLoginLog(repository)
	authService.ConfigureCaptcha(bizauth.NewCaptchaUsecase(data.NewCaptchaStore(resources), nil))
	verificationKey := sha256.Sum256([]byte(cfg.Auth.SecretKey + ":verification"))
	verificationUsecase, err := bizauth.NewVerificationUsecase(repository, repository, hasher, verificationKey[:], providers.MessageSenders, nil)
	if err != nil { return nil, fmt.Errorf("初始化验证码用例失败: %w", err) }
	loginUsecase.ConfigureVerification(verificationUsecase)
	authService.ConfigureVerification(verificationUsecase)
	managementRepository := data.NewManagementRepository(resources, providerconfig.NewCodec(providers.ConfigCipher))
	fileUsecase := filebiz.NewUsecase(data.NewFileRepository(resources), providers.Storage, cfg.Storage.MaxFileSize, nil)
	logUsecase := logexport.NewUsecase(data.NewLogExportRepository(resources), providers.Storage, nil)
	return &Services{
		Health: NewHealthService("kratos-admin-api"), Auth: authService,
		Management: NewManagementService(managementRepository, managementRepository),
		File: NewFileService(fileUsecase, managementRepository),
		Log: NewLogService(logUsecase, managementRepository),
	}, nil
}
```

- [ ] **Step 5: 创建 Kratos Admin App 构造器**

Create `app/admin/internal/server/app.go`:

```go
package server

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const version = "0.1.0"

// NewApp 创建同时提供 HTTP 与 gRPC transport 的 Admin 应用。
func NewApp(httpServer *khttp.Server, grpcServer *kgrpc.Server, logger log.Logger) *kratos.App {
	return kratos.New(
		kratos.Name("kratos-admin-api"),
		kratos.Version(version),
		kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer),
	)
}
```

- [ ] **Step 6: 创建 Admin Wire 注入器并生成代码**

Create `app/admin/cmd/server/logger.go`:

```go
package main

import (
	"os"

	"github.com/go-kratos/kratos/v2/log"
)

func newLogger() log.Logger {
	return log.With(log.NewStdLogger(os.Stdout), "ts", log.DefaultTimestamp, "caller", log.DefaultCaller, "component", "admin")
}
```

Create `app/admin/cmd/server/wire.go`:

```go
//go:build wireinject

package main

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	adminserver "github.com/sleep-go/kratos-admin/app/admin/internal/server"
	adminservice "github.com/sleep-go/kratos-admin/app/admin/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
)

func wireAdminApp(ctx context.Context, cfg conf.Config) (*kratos.App, func(), error) {
	wire.Build(data.NewData, provider.NewAdminSet, adminservice.NewServices, adminserver.NewHTTPServer, adminserver.NewGRPCServer, newLogger, adminserver.NewApp)
	return nil, nil, nil
}
```

Run:

```bash
go tool wire ./app/admin/cmd/server
```

Expected: 生成 `app/admin/cmd/server/wire_gen.go`，函数签名与 `wireAdminApp` 完全一致。

- [ ] **Step 7: 删除旧 App 容器并验证 Admin 包**

Run:

```bash
git rm backend/internal/app/app.go backend/internal/app/app_test.go
gofmt -w app/admin internal
GOCACHE=/tmp/go-build go test ./app/admin/internal/... ./internal/...
```

Expected: PASS，且 `rg 'backend/internal/(app|server|service)' --glob '*.go' .` 无输出。

- [ ] **Step 8: 提交 Admin 分层与 Wire 图**

```bash
git add go.mod go.sum app/admin internal backend/internal
git commit -m "重构：建立Admin应用分层与Wire注入"
```

### Task 5: 将三个 Admin 入口改为独立 Cobra 可执行程序

**Files:**
- Create: `app/admin/cmd/server/main.go`
- Create: `app/admin/cmd/server/main_test.go`
- Move/Modify: `backend/cmd/initadmin/main.go` → `app/admin/cmd/initadmin/main.go`
- Create: `app/admin/cmd/initadmin/main_test.go`
- Move/Modify: `backend/cmd/gormgen/main.go` → `app/admin/cmd/gormgen/main.go`
- Create: `app/admin/cmd/gormgen/main_test.go`
- Delete: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: `wireAdminApp`、现有 initadmin 初始化逻辑和现有 GORM Gen 模型清单。
- Produces: 三个 `newRootCommand`，命令名分别为 `admin-server`、`admin-initadmin`、`admin-gormgen`。

- [ ] **Step 1: 为三个 Cobra 根命令写参数与错误传播测试**

Each `main_test.go` uses the package-local `newRootCommand`. Admin Server asserts `cmd.Use` and injected runner error is returned. Initadmin test executes:

```go
func TestRootCommandParsesAdminOptions(t *testing.T) {
	var got initAdminOptions
	cmd := newRootCommand(func(_ context.Context, options initAdminOptions) error {
		got = options
		return nil
	})
	cmd.SetArgs([]string{"--username", "root", "--display-name", "平台管理员", "--email", "root@example.com", "--phone", "13800000000"})
	if err := cmd.ExecuteContext(context.Background()); err != nil { t.Fatal(err) }
	if got.Username != "root" || got.DisplayName != "平台管理员" || got.Email != "root@example.com" || got.Phone != "13800000000" {
		t.Fatalf("options = %+v", got)
	}
}
```

Admin Server test uses an injected runner returning `errors.New("启动失败")`, asserts `errors.Is(cmd.ExecuteContext(context.Background()), want)` and asserts `cmd.Use == "admin-server"`.

Gormgen test executes the exact assertion:

```go
func TestRootCommandParsesOutputPath(t *testing.T) {
	var got genOptions
	cmd := newRootCommand(func(_ context.Context, options genOptions) error {
		got = options
		return nil
	})
	cmd.SetArgs([]string{"--out-path", "internal/data/query"})
	if err := cmd.ExecuteContext(context.Background()); err != nil { t.Fatal(err) }
	if got.OutPath != "internal/data/query" { t.Fatalf("OutPath = %q", got.OutPath) }
}
```

- [ ] **Step 2: 运行命令测试确认 Cobra 接口尚未完成**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/cmd/...
```

Expected: FAIL，错误指向缺少 `newRootCommand`、`initAdminOptions` 或 `genOptions`。

- [ ] **Step 3: 实现统一的 main/ExecuteContext 模式**

Each Admin main uses this exact control flow, with its own Chinese prefix:

```go
func main() {
	if err := newRootCommand(run).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Server root command:

```go
func newRootCommand(run func(context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use: "admin-server", Short: "启动 Kratos Admin HTTP/gRPC 服务",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := run(cmd.Context()); err != nil { return fmt.Errorf("Admin Server 启动失败: %w", err) }
			return nil
		},
	}
}

func run(ctx context.Context) error {
	cfg, err := conf.LoadFromEnv()
	if err != nil { return fmt.Errorf("加载 Admin 配置失败: %w", err) }
	application, cleanup, err := wireAdminApp(ctx, cfg)
	if err != nil { return fmt.Errorf("初始化 Admin 依赖失败: %w", err) }
	defer cleanup()
	if err := application.Run(); err != nil { return fmt.Errorf("Admin 进程退出: %w", err) }
	return nil
}

```

- [ ] **Step 4: 将 initadmin 的 flag 改为 Cobra options**

Define:

```go
type initAdminOptions struct { Username, DisplayName, Email, Phone string }

func newRootCommand(run func(context.Context, initAdminOptions) error) *cobra.Command {
	options := initAdminOptions{Username: "admin", DisplayName: "超级管理员"}
	cmd := &cobra.Command{
		Use: "admin-initadmin", Short: "幂等初始化平台超级管理员",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error { return run(cmd.Context(), options) },
	}
	cmd.Flags().StringVar(&options.Username, "username", options.Username, "平台管理员用户名")
	cmd.Flags().StringVar(&options.DisplayName, "display-name", options.DisplayName, "平台管理员显示名称")
	cmd.Flags().StringVar(&options.Email, "email", "", "平台管理员邮箱")
	cmd.Flags().StringVar(&options.Phone, "phone", "", "平台管理员手机号")
	return cmd
}
```

Move the existing initializer body to `run(ctx, options)`, keep password exclusively in `KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD`, and use `options` fields when constructing `setup.AdminInput`.

- [ ] **Step 5: 将 gormgen 改为 Cobra 并修正生成路径**

Define:

```go
type genOptions struct { OutPath string }

func newRootCommand(run func(context.Context, genOptions) error) *cobra.Command {
	options := genOptions{OutPath: "internal/data/query"}
	cmd := &cobra.Command{
		Use: "admin-gormgen", Short: "生成 GORM Gen 类型安全查询代码",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error { return run(cmd.Context(), options) },
	}
	cmd.Flags().StringVar(&options.OutPath, "out-path", options.OutPath, "查询代码输出目录")
	return cmd
}
```

The runner must set:

```go
gen.Config{
	OutPath: options.OutPath,
	ModelPkgPath: "github.com/sleep-go/kratos-admin/internal/data/model",
	Mode: gen.WithDefaultQuery | gen.WithQueryInterface,
}
```

- [ ] **Step 6: 格式化并验证三个已可独立编译的 Admin 命令**

Run:

```bash
gofmt -w app/admin/cmd
GOCACHE=/tmp/go-build go test ./app/admin/cmd/...
GOCACHE=/tmp/go-build go build ./app/admin/cmd/server ./app/admin/cmd/initadmin ./app/admin/cmd/gormgen
```

Expected: Admin 命令测试和构建通过；Worker 的最终构建在 Task 6 完成。

- [ ] **Step 7: 提交 Cobra 入口**

```bash
git add app/admin/cmd go.mod go.sum
git commit -m "重构：将Admin入口统一为Cobra命令"
```

### Task 6: 拆分 Worker Service/Server 并生成独立 Wire 图

**Files:**
- Create: `app/worker/internal/service/tasks.go`
- Create: `app/worker/internal/service/tasks_test.go`
- Create: `app/worker/internal/server/server.go`
- Create: `app/worker/cmd/worker/main.go`
- Create: `app/worker/cmd/worker/main_test.go`
- Create: `app/worker/cmd/worker/wire.go`
- Generate: `app/worker/cmd/worker/wire_gen.go`
- Delete: `backend/cmd/worker/main.go`
- Delete: `backend/internal/worker/server.go`

**Interfaces:**
- Consumes: `data.Data`、`provider.WorkerSet`、`audit.Processor`、`logexport.Processor` 和现有 Worker 行为。
- Produces: 文件结构锁定章节中的 `service.Service` 四个方法、`server.NewServer`、`server.NewApp`、`wireWorkerApp(context.Context, conf.Config) (*kratos.App, func(), error)` 和 Cobra 命令 `worker`。

- [ ] **Step 1: 为任务载荷校验和 Server 路由写失败测试**

`app/worker/internal/service/tasks_test.go` must cover these exact invalid payloads:

```go
package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	local, err := storage.NewLocalProvider(t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil)
	if err != nil { t.Fatalf("NewLocalProvider() error = %v", err) }
	return NewService(&data.Data{}, &provider.WorkerSet{Storage: local}, log.NewStdLogger(io.Discard))
}

func TestHandleAuditRejectsMissingEventID(t *testing.T) {
	service := newTestService(t)
	err := service.HandleAudit(context.Background(), asynq.NewTask("audit:publish:v1", []byte(`{}`)))
	if !errors.Is(err, asynq.SkipRetry) { t.Fatalf("error = %v", err) }
}

func TestHandleLogExportRejectsInvalidJSON(t *testing.T) {
	service := newTestService(t)
	err := service.HandleLogExport(context.Background(), asynq.NewTask("log:export:v1", []byte(`{`)))
	if !errors.Is(err, asynq.SkipRetry) { t.Fatalf("error = %v", err) }
}

func TestHandleFileCleanupRejectsIncompletePayload(t *testing.T) {
	service := newTestService(t)
	err := service.HandleFileCleanup(context.Background(), asynq.NewTask("file:cleanup:v1", []byte(`{"tenant_id":1}`)))
	if !errors.Is(err, asynq.SkipRetry) { t.Fatalf("error = %v", err) }
}
```

Create `app/worker/cmd/worker/main_test.go`:

```go
package main

import (
	"context"
	"errors"
	"testing"
)

func TestRootCommandReturnsRunnerError(t *testing.T) {
	want := errors.New("启动失败")
	cmd := newRootCommand(func(context.Context) error { return want })
	cmd.SetArgs(nil)
	if err := cmd.ExecuteContext(context.Background()); !errors.Is(err, want) {
		t.Fatalf("ExecuteContext() error = %v, want %v", err, want)
	}
	if cmd.Use != "worker" {
		t.Fatalf("Use = %q, want worker", cmd.Use)
	}
}
```

- [ ] **Step 2: 运行测试确认 Worker Service 尚不存在**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/worker/...
```

Expected: FAIL，错误包含缺少 `Service` 或 `HandleAudit`。

- [ ] **Step 3: 将业务任务处理搬到 Worker Service**

Move from `backend/internal/worker/server.go` into `app/worker/internal/service/tasks.go`:

- constants `audit:publish:v1`、`log:export:v1`、`file:cleanup:v1`，export as `TaskAuditPublish`、`TaskLogExport`、`TaskFileCleanup`；
- three payload structs;
- repositories/processors/storage/client/maintenance state;
- `handleAudit` → `HandleAudit`；
- `handleLogExport` → `HandleLogExport`；
- `handleFileCleanup` → `HandleFileCleanup`；
- `enqueuePending(ctx)` → `EnqueuePending(ctx, now)`，所有该轮维护判断使用传入 `now.UTC()`，下次维护时间设置为 `now.UTC().Add(time.Hour)`。

Keep task IDs, retry counts, timeouts, `asynq.SkipRetry`, cleanup failure marking and Chinese log messages unchanged.

- [ ] **Step 4: 让 Worker Server 只负责 transport 生命周期**

Create `app/worker/internal/server/server.go` with:

```go
type Server struct {
	server *asynq.Server
	tasks *workerservice.Service
	logger *log.Helper
	stop chan struct{}
	stopOnce sync.Once
}

// NewServer 创建 Asynq transport 和周期投递生命周期。
func NewServer(cfg conf.Config, tasks *workerservice.Service, logger log.Logger) *Server {
	return &Server{
		server: asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.Data.RedisAddr, DB: cfg.Data.RedisDB}, asynq.Config{Concurrency: 10}),
		tasks: tasks, logger: log.NewHelper(logger), stop: make(chan struct{}),
	}
}

// Start 启动任务消费，并周期扫描未投递任务。
func (s *Server) Start(ctx context.Context) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(workerservice.TaskAuditPublish, s.tasks.HandleAudit)
	mux.HandleFunc(workerservice.TaskLogExport, s.tasks.HandleLogExport)
	mux.HandleFunc(workerservice.TaskFileCleanup, s.tasks.HandleFileCleanup)
	go s.dispatch(ctx)
	s.logger.Info("异步任务 Worker 已启动")
	if err := s.server.Run(mux); err != nil { return fmt.Errorf("Asynq Worker 退出: %w", err) }
	return nil
}
```

Keep `Stop` behavior. `dispatch` calls `s.tasks.EnqueuePending(ctx, time.Now())` before each select and every two seconds.

- [ ] **Step 5: 创建 Worker Kratos App**

In `app/worker/internal/server/server.go` add:

```go
// NewApp 创建独立 Worker Kratos 应用。
func NewApp(server *Server, logger log.Logger) *kratos.App {
	return kratos.New(
		kratos.Name("kratos-admin-worker"), kratos.Version("0.1.0"),
		kratos.Logger(logger), kratos.Server(server),
	)
}
```

- [ ] **Step 6: 实现 Worker Cobra 入口**

Create `app/worker/cmd/worker/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"
	"github.com/sleep-go/kratos-admin/internal/conf"
)

func main() {
	if err := newRootCommand(run).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(run func(context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use: "worker", Short: "启动 Kratos Admin 异步任务 Worker",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := run(cmd.Context()); err != nil { return fmt.Errorf("Worker 启动失败: %w", err) }
			return nil
		},
	}
}

func run(ctx context.Context) error {
	cfg, err := conf.LoadFromEnv()
	if err != nil { return fmt.Errorf("加载 Worker 配置失败: %w", err) }
	application, cleanup, err := wireWorkerApp(ctx, cfg)
	if err != nil { return fmt.Errorf("初始化 Worker 依赖失败: %w", err) }
	defer cleanup()
	if err := application.Run(); err != nil { return fmt.Errorf("Worker 进程退出: %w", err) }
	return nil
}

func newLogger() log.Logger {
	return log.With(log.NewStdLogger(os.Stdout), "ts", log.DefaultTimestamp, "caller", log.DefaultCaller, "component", "worker")
}
```

- [ ] **Step 7: 创建 Worker Wire 注入器并生成代码**

Create `app/worker/cmd/worker/wire.go`:

```go
//go:build wireinject

package main

import (
	"context"
	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	workerserver "github.com/sleep-go/kratos-admin/app/worker/internal/server"
	workerservice "github.com/sleep-go/kratos-admin/app/worker/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
)

func wireWorkerApp(ctx context.Context, cfg conf.Config) (*kratos.App, func(), error) {
	wire.Build(data.NewData, provider.NewWorkerSet, workerservice.NewService, newLogger, workerserver.NewServer, workerserver.NewApp)
	return nil, nil, nil
}
```

Run:

```bash
go tool wire ./app/worker/cmd/worker
```

Expected: `app/worker/cmd/worker/wire_gen.go` generated with matching signature.

- [ ] **Step 8: 删除旧 Worker 入口和包并验证完整后端编译**

Run:

```bash
git rm backend/cmd/worker/main.go
git rm backend/internal/worker/server.go
gofmt -w app/worker
GOCACHE=/tmp/go-build go test ./app/worker/... ./internal/architecture
GOCACHE=/tmp/go-build go build ./app/admin/cmd/server ./app/admin/cmd/initadmin ./app/admin/cmd/gormgen ./app/worker/cmd/worker
```

Expected: 全部通过，四个独立 main package 均可构建。

- [ ] **Step 9: 提交 Worker 分层、Cobra 与 Wire 图**

```bash
git add app/worker
git commit -m "重构：拆分Worker分层与独立入口"
```

### Task 7: 更新生成、构建、迁移与部署路径

**Files:**
- Modify: `Makefile`
- Move/Modify: `backend/Dockerfile` → `deploy/Dockerfile.backend`
- Modify: `deploy/docker-compose.yml`
- Regenerate: `internal/data/query/*.gen.go`

**Interfaces:**
- Consumes: 四个新命令路径、根级 `migrations`、两个 Wire 包。
- Produces: `make wire`、`make gorm-gen`、`make backend-test`、`make backend-build` 和可构建 Compose targets。

- [ ] **Step 1: 先运行旧 Make 目标并记录路径失败**

Run:

```bash
make backend-build
```

Expected: FAIL，错误指向不存在的 `./backend/cmd/api` 或 `./backend/cmd/worker`。

- [ ] **Step 2: 更新 Makefile 精确目标**

Set:

```make
.PHONY: api wire gorm-gen init-admin backend-test backend-vet backend-build frontend-install frontend-test frontend-build frontend-e2e compose-config compose-up compose-down

wire:
	go tool wire ./app/admin/cmd/server ./app/worker/cmd/worker

gorm-gen:
	GOCACHE=/tmp/go-build go run ./app/admin/cmd/gormgen

init-admin:
	GOCACHE=/tmp/go-build go run ./app/admin/cmd/initadmin

backend-test:
	GOCACHE=/tmp/go-build go test -race ./app/... ./internal/...

backend-vet:
	GOCACHE=/tmp/go-build go vet ./app/... ./internal/...

backend-build:
	GOCACHE=/tmp/go-build go build ./app/admin/cmd/server ./app/admin/cmd/initadmin ./app/admin/cmd/gormgen ./app/worker/cmd/worker
```

Keep existing frontend and Compose targets unchanged.

- [ ] **Step 3: 迁移后端 Dockerfile 并更新 build targets**

Run:

```bash
git mv backend/Dockerfile deploy/Dockerfile.backend
```

Modify its build stage:

```dockerfile
COPY api ./api
COPY app ./app
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/admin-server ./app/admin/cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./app/worker/cmd/worker
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/admin-initadmin ./app/admin/cmd/initadmin
```

The tools stage copies `migrations` to `/app/migrations`. Rename target `api` to `admin-server`; its entrypoint is `/usr/local/bin/admin-server`. Keep targets `worker`, `init-admin`, `tools`, their non-root user, ports and runtime behavior.

- [ ] **Step 4: 更新 Compose 的 Dockerfile、target 和 Goose 路径**

Apply these exact substitutions in `deploy/docker-compose.yml`:

```text
dockerfile: backend/Dockerfile       -> dockerfile: deploy/Dockerfile.backend
target: api                          -> target: admin-server
/app/backend/migrations              -> /app/migrations
```

Keep service name `api` so frontend proxy and existing operational commands remain stable; only image build target and executable change.

- [ ] **Step 5: 重新生成 GORM Gen 并验证 import 路径**

Run:

```bash
make gorm-gen
gofmt -w internal/data/query
rg 'backend/internal/data/model' internal/data/query
```

Expected: GORM Gen succeeds；最后一条命令无输出；generated files import `github.com/sleep-go/kratos-admin/internal/data/model`。

- [ ] **Step 6: 验证生成、构建和 Compose 配置**

Run:

```bash
make wire
make backend-build
make compose-config
```

Expected: 三个命令退出码为 0；Compose 的 migrate command contains `goose -dir /app/migrations`。

- [ ] **Step 7: 提交构建与部署路径**

```bash
git add Makefile deploy internal/data/query
git commit -m "构建：更新大仓生成与部署路径"
```

### Task 8: 同步文档并完成原子迁移验收

**Files:**
- Modify: `README.md`
- Modify: `docs/deployment.md`
- Create: `docs/acceptance.md`
- Modify: `.env.example` only if it contains an old migration or binary path
- Delete: empty `backend/`

**Interfaces:**
- Consumes: final app paths, Make targets, Compose targets and Goose path.
- Produces: no stale `backend/*` operational instructions and a clean repository tree.

- [ ] **Step 1: 更新不使用 Docker 的启动文档**

Document this exact order in `README.md` and `docs/deployment.md`:

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
goose -dir migrations mysql "$KRATOS_ADMIN_MYSQL_DSN" up
KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD='replace-with-strong-password' go run ./app/admin/cmd/initadmin
go run ./app/admin/cmd/server
go run ./app/worker/cmd/worker
cd frontend
pnpm install
pnpm dev
```

Also list `go run ./app/admin/cmd/gormgen --out-path internal/data/query` and `make wire` as development commands. Do not document service startup as a migration mechanism.

- [ ] **Step 2: 更新验收文档的后端命令和目录树**

Write `docs/acceptance.md` with the final directory constraints and these backend commands:

```bash
make wire
make gorm-gen
make backend-test
make backend-vet
make backend-build
go test ./app/... ./internal/...
go vet ./app/... ./internal/...
```

Describe applications as Admin Server and Worker and list the four executable paths exactly.

- [ ] **Step 3: 清除旧路径并运行静态守卫**

Run:

```bash
find backend -type f -print
rg 'github.com/sleep-go/kratos-admin/backend|backend/cmd|backend/internal|backend/migrations|backend/Dockerfile' --glob '!docs/superpowers/specs/2026-07-20-kratos-monorepo-layout-design.md' --glob '!docs/superpowers/plans/2026-07-20-kratos-monorepo-layout.md' .
```

Expected: 两条命令均无输出。确认 `backend/` 不含文件后，运行 `find backend -depth -type d -empty -delete` 清除空目录；该命令不得删除任何文件或非空目录。

- [ ] **Step 4: 验证 Wire、GORM Gen 与 API 生成幂等**

Run:

```bash
make wire
git diff --exit-code -- app/admin/cmd/server/wire_gen.go app/worker/cmd/worker/wire_gen.go
make gorm-gen
git diff --exit-code -- internal/data/query
make api
cd frontend && pnpm api:generate && cd ..
git diff --exit-code -- api frontend/src/api/generated
```

Expected: 每个 `git diff --exit-code` 返回 0，生成器重复执行不产生差异。

- [ ] **Step 5: 运行后端验收**

Run:

```bash
make backend-test
make backend-vet
make backend-build
```

Expected: 所有命令退出码为 0，race detector 和 vet 无新增报告，四个二进制均构建成功。

- [ ] **Step 6: 在 MySQL 8 与 Redis 环境执行集成和迁移验收**

Run:

```bash
goose -dir migrations mysql "$KRATOS_ADMIN_TEST_MYSQL_DSN" up
goose -dir migrations mysql "$KRATOS_ADMIN_TEST_MYSQL_DSN" status
KRATOS_ADMIN_INTEGRATION=1 KRATOS_ADMIN_TEST_MYSQL_DSN="$KRATOS_ADMIN_TEST_MYSQL_DSN" KRATOS_ADMIN_TEST_REDIS_ADDR="$KRATOS_ADMIN_TEST_REDIS_ADDR" GOCACHE=/tmp/go-build go test ./internal/data/... ./app/worker/...
```

Expected: Goose status shows versions `00001` through `00005` applied；integration tests pass；第二次 `goose ... up` reports no migrations to run.

- [ ] **Step 7: 运行前端和 Compose 验收**

Run:

```bash
cd frontend && pnpm lint && pnpm typecheck && pnpm test:run && pnpm build
cd .. && make compose-config
docker compose --env-file .env -f deploy/docker-compose.yml build migrate init-admin api worker frontend
```

Expected: lint、typecheck、Vitest、Vite build、Compose config 和六个镜像 target 全部成功。若本机具备 Playwright 浏览器，再执行 `make frontend-e2e`，预期桌面和手机项目通过。

- [ ] **Step 8: 最终检查并提交文档与清理**

Run:

```bash
git status --short
git diff --check
```

Expected: 只包含 README、部署/验收文档和必要的空目录清理；`git diff --check` 无输出。

```bash
git add README.md docs .env.example
git commit -m "文档：同步大仓启动与验收说明"
```

## 完成标准

- `tree app internal migrations -L 4` 与设计文档的目标树一致。
- `internal/architecture` 测试持续阻止 Data → Service/Server、Biz → Data/App 和 Admin ↔ Worker 内部交叉依赖。
- 四个 Cobra 命令的命令名、参数解析和错误传播测试通过。
- Admin 与 Worker 的 Wire 生成结果已提交且重复生成无差异。
- GORM Gen 和 Protobuf/TypeScript 客户端重复生成无差异。
- Goose 仍是唯一迁移入口，五个 SQL 的内容未变化。
- 后端测试、race、vet、四入口构建、前端校验和 Compose 构建通过。
- 除历史设计/计划文档的上下文说明外，仓库中不存在 `backend/` 路径引用，实际 `backend/` 目录不存在。
