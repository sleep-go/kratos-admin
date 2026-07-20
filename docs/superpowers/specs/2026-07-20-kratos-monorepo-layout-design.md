# Kratos Admin 大仓目录重构设计

> 后续变更：运行入口已按用户确认收敛为 `app/admin/cmd/kratos-admin` 单一 Cobra 可执行程序，原四个命令改为 `server`、`worker`、`init-admin`、`gorm-gen` 子命令。本文中的旧路径仅表示此前设计阶段。

## 背景与目标

当前后端集中在 `backend/cmd`、`backend/internal` 和 `backend/migrations`。API、Worker、运维命令及共享业务代码处于同一目录层级，不符合用户确认的 go-kratos 大仓结构。

本次重构目标是：

- 采用根级 `app/*` 组织独立应用。
- API 与 Worker 分别作为 `app/admin`、`app/worker` 两个独立应用。
- 严格执行 `biz`、`data`、`service`、`server` 分层和单向依赖。
- 使用 Google Wire 为两个常驻应用生成独立依赖注入图。
- 四个独立可执行程序全部使用 Cobra。
- 保持 Go module、Protobuf API、数据库结构和前后端业务行为不变。

## 方案选择

### 采用方案

共享领域与基础设施放在根级 `internal/*`，应用适配代码放在各自 `app/*/internal`：

```text
api/
  admin/v1/

app/
  admin/
    cmd/
      server/
        main.go
        wire.go
        wire_gen.go
      initadmin/
        main.go
      gormgen/
        main.go
    internal/
      service/
      server/

  worker/
    cmd/
      worker/
        main.go
        wire.go
        wire_gen.go
    internal/
      service/
      server/

internal/
  biz/
    audit/
    auth/
    file/
    logexport/
    permission/
    providerconfig/
    setting/
    setup/
  conf/
  data/
    model/
    query/
  provider/
    message/
    secret/
    storage/

migrations/
frontend/
deploy/
docs/
```

根级 `internal` 允许同一 Go module 内的两个应用共享实现，同时禁止外部 module 直接依赖业务内部包。该方案不复制领域、模型、仓储和 Provider，也不把业务实现暴露到公共 `pkg`。

### 不采用的方案

- 共享代码放 `pkg/*`：会把认证、权限和仓储等内部实现变成公共依赖面。
- 两个应用完全自包含：会复制领域、模型、仓储和 Provider，增加行为漂移风险。
- 单应用多命令：不能体现已确认的 API 与 Worker 独立应用边界。

## 分层职责

### Biz

`internal/biz` 保存领域实体、用例、仓储接口、权限规则和数据范围规则。分页、认证作用域、资源作用域等跨适配层契约归属 Biz，不归属 Service。

Biz 不依赖 Data、Admin Service、Worker Service 或具体 Provider 实现。

### Data

`internal/data` 保存 GORM/GORM Gen 模型与查询、MySQL/Redis/Casbin 持久化和仓储实现。Data 实现 Biz 定义的接口，只能向 Biz 方向依赖。

现有 `data → service.ResourceScope` 等反向依赖必须消除，相关类型和接口迁移至 Biz。

### Service

`app/admin/internal/service` 负责 Protobuf DTO、Kratos transport 上下文与 Biz 用例之间的转换，不编写 SQL 或业务规则。

`app/worker/internal/service` 负责 Asynq 任务载荷、幂等信息与 Biz 用例之间的转换，不直接复制 API Service 逻辑。

### Server

`app/admin/internal/server` 创建 Kratos HTTP/gRPC Server，注册中间件、健康检查和生成服务。

`app/worker/internal/server` 管理 Asynq Server、任务路由、优雅停机和 Worker 生命周期。

### Conf 与 Provider

`internal/conf` 保存 API、Worker 和命令行工具共享的环境配置加载。

`internal/provider` 保存 SMTP、阿里云短信、本地文件、阿里云 OSS 和密钥加密实现。Provider 对 Biz 暴露的小接口负责，不向 Service 层泄漏供应商错误。

## 依赖方向

允许的主要依赖方向为：

```text
api/admin/v1
  ↓
app/admin/internal/server
  ↓
app/admin/internal/service
  ↓
internal/biz
  ↑
internal/data + internal/provider

app/worker/internal/server
  ↓
app/worker/internal/service
  ↓
internal/biz
  ↑
internal/data + internal/provider
```

禁止以下依赖：

- `internal/data` 导入任何 `service` 或 `server` 包。
- `internal/biz` 导入 `data`、应用 Service 或应用 Server。
- Admin Service 与 Worker Service 相互导入。
- Worker 导入 `app/admin/internal`，Admin 导入 `app/worker/internal`。

## 命令行与进程

现行实现保留一个 `kratos-admin` 二进制：

| 子命令 | 源码入口 | 职责 |
| --- | --- | --- |
| `server` | `app/admin/cmd/kratos-admin` | Kratos HTTP/gRPC API |
| `init-admin` | `app/admin/cmd/kratos-admin` | 幂等初始化平台管理员 |
| `gorm-gen` | `app/admin/cmd/kratos-admin` | 生成 GORM Gen 查询代码 |
| `worker` | `app/admin/cmd/kratos-admin` | Asynq、审计、导出与文件清理 |

统一入口使用 `github.com/spf13/cobra`：

- `main` 只调用根命令并设置退出码。
- 业务执行使用 `RunE` 返回带中文上下文的错误。
- `init-admin` 保留用户名、显示名称、邮箱、手机号参数，密码只从 `KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD` 读取。
- `gorm-gen` 默认输出到 `internal/data/query`，允许通过参数覆盖输出目录。
- Server 和 Worker 保留信号处理与优雅停机。

## Wire 依赖注入

Admin Server 与 Worker 各自维护 `wire.go` 和已提交的 `wire_gen.go`：

- Admin 注入图组合 Conf、Data、Provider、Biz Usecase、Admin Service、HTTP/gRPC Server 和 Kratos App。
- Worker 注入图组合 Conf、Data、Provider、Biz Usecase、Worker Service 和 Asynq Server。
- 两个图不得通过一个全局容器间接拉入对方不需要的依赖。
- `initadmin` 和 `gormgen` 是短生命周期工具，使用小型显式构造，不复用常驻应用的完整注入图。

提供 `make wire` 重复生成两个图，并将生成一致性纳入验收。

## 数据库与生成代码

- `backend/migrations` 移至根级 `migrations`。
- Goose 继续是唯一数据库迁移入口，服务启动不得执行 `AutoMigrate`。
- 当前迁移版本保持为 5，迁移文件内容不因目录重构发生业务变化。
- GORM Gen 模型和查询迁移到 `internal/data/model` 与 `internal/data/query`。
- `admin-gormgen` 的 `ModelPkgPath` 更新为 `github.com/sleep-go/kratos-admin/internal/data/model`。
- Protobuf 真相源继续位于 `api/admin/v1`，生成 Go 和 OpenAPI 契约不改变公开 import 路径。

## 构建与部署

- Makefile 更新全部 Go 入口、Wire、GORM Gen、测试和构建路径。
- Docker 构建上下文改为 `app/admin`、`app/worker`、根级 `internal`、`api` 和 `migrations`。
- Compose 的 Goose 路径改为 `/app/migrations`，API、Worker 和 initadmin 镜像入口更新为新二进制。
- README、部署文档、验收命令和排障路径同步修改。
- 重构完成后删除空的 `backend/`，不保留旧路径兼容入口。

## 迁移策略

采用一次性原子迁移：

1. 移动共享 Biz、Data、Conf、Provider。
2. 拆分 Admin 与 Worker 专属 Service、Server 和进程编排。
3. 将四个入口改为独立 Cobra 命令。
4. 建立并生成两个 Wire 注入图。
5. 更新全部 import、生成路径、Makefile、Docker、Compose 和文档。
6. 删除旧目录并执行完整验证。

同一阶段内不同时保留新旧入口，避免双路径长期漂移。

## 错误处理

- Cobra `RunE` 向上返回错误，命令入口不吞掉失败。
- 配置、Wire 构造、数据库连接、Redis、Provider 和服务启动错误保留中文业务上下文并使用 `%w` 包装。
- `main` 只负责输出最终错误和非零退出码。
- Server 与 Worker 收到退出信号后执行有界优雅停机。

## 测试与验收

- 增加架构约束测试，阻止 Data 反向导入 Service。
- 增加 Cobra 命令名、参数解析和错误返回测试。
- `make wire` 重复生成后 Git 无差异。
- `make gorm-gen` 重复生成后 Git 无差异。
- `make api` 与 TypeScript 客户端重复生成后 Git 无差异。
- 在全新 MySQL 8 执行 Goose 迁移到版本 5，再次执行无待执行迁移。
- 执行后端单元测试、MySQL 集成测试、`-race`、`go vet` 和四个二进制构建。
- 执行前端 lint、typecheck、Vitest、build 及桌面/手机 E2E。
- 构建 API、Worker、initadmin、Goose、Frontend 镜像并启动完整 Compose 健康检查。
- 最终确认所有旧 `backend/` import 和路径已清除，Git 工作区干净。

## 非目标

- 不拆分 Go module。
- 不改变 Protobuf API、数据库表结构或前端业务功能。
- 不把模块化单体改造成网络微服务。
- 不引入 Kubernetes 或新的配置中心。
- 不为了目录重构改写现有业务规则。
