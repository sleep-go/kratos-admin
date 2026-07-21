# Kratos Admin

基于 go-kratos 与 Vue 3 的前后端分离、多租户通用管理后台。后端以单一 Admin 进程同时承载 HTTP/gRPC 和 RabbitMQ 后台任务，启动时通过 MySQL 命名锁自动执行 Goose 迁移。

## 功能

- 原生多租户、平台治理、租户切换与会话管理
- 用户、成员、部门、岗位和租户管理
- Casbin domain RBAC、动态菜单、按钮/API 权限和五类数据范围
- 登录日志、操作审计、API 访问日志和异步日志导出
- 参数字典、平台/租户设置、安全策略和日志保留策略
- SMTP、阿里云短信、本地模拟 Provider
- 本地文件与阿里云 OSS，支持直传、校验、私有下载和异步清理
- 平台/租户工作台及桌面、平板、手机响应式布局

## 技术栈

- 后端：Go 1.26、go-kratos、GORM Gen、Goose、Casbin、Redis、RabbitMQ
- 前端：Vue 3、Vite、TypeScript、Element Plus、Tailwind CSS、SCSS、Pinia、ECharts
- 数据库：PolarDB MySQL 8；本地开发使用 MySQL 8.4
- 部署：Docker Compose、Nginx、阿里云 OSS

## 目录结构

```text
app/
├── admin/
│   ├── cmd/
│   │   ├── server/             # 官方 Kratos 服务入口与 Wire
│   │   └── tools/              # Cobra 迁移、初始化与代码生成工具
│   └── internal/
│       ├── biz/                # 领域用例与仓储契约
│       ├── conf/               # Bootstrap 配置 Proto 与加载器
│       ├── data/               # MySQL、Redis、GORM 与外部 Provider
│       ├── server/             # HTTP、gRPC 与 RabbitMQ Task Server
│       └── service/            # Protobuf transport 服务实现
└── frontend/                   # Vue 3 前端应用
api/admin/v1/                   # 业务 API Proto
configs/                        # 宿主机与 Compose 的完整 Kratos YAML
migrations/                     # Goose SQL 迁移
deploy/Dockerfile.backend       # 后端多阶段镜像
docker-compose.yml              # 完整本地编排
```

Admin 是唯一后端应用。RabbitMQ 发布扫描、三类消费者、日志维护和连接重试均作为受控协程接入同一 Kratos 生命周期；MySQL 保存任务真相和重试状态。

## 环境要求

- Go 1.26+
- Node.js 24+ 与 pnpm 10+
- Docker Desktop（使用 Compose 时）
- 本地运行依赖时需要 MySQL 8、Redis 7 和 RabbitMQ 4

仓库提供两份完整配置：

- `configs/config.yaml`：宿主机开发，依赖地址使用 `127.0.0.1`。
- `configs/config.docker.yaml`：完整 Compose，依赖地址使用服务名。

首次启动前修改对应 YAML 中的 `auth.secret_key`、`auth.jwt_private_key` 和 `setup.admin.initial_password`。仓库值仅用于本地开发，生产密钥不得提交 Git。

## Docker Compose 完整启动

根目录 Compose 包含 MySQL、Redis、RabbitMQ、Mailpit、Admin、超级管理员初始化和 Frontend：

```bash
make compose-config
make compose-up
```

启动顺序为：MySQL/Redis/RabbitMQ/Mailpit → Admin 启动并自动迁移 → 幂等管理员初始化 → Frontend。RabbitMQ 不作为 Admin 健康启动前置条件，暂时不可用时 API 仍可服务，异步任务保留在 MySQL 并在恢复后补投。

访问地址：

- 管理后台：<http://127.0.0.1:8080>
- API 健康检查：<http://127.0.0.1:8000/api/v1/health>
- Mailpit：<http://127.0.0.1:8025>
- RabbitMQ 管理台：<http://127.0.0.1:15672>

停止服务：

```bash
make compose-down
```

## Compose 启动依赖，前后端本地运行

只启动 MySQL、Redis、RabbitMQ 和 Mailpit：

```bash
make compose-deps-up
```

宿主机命令默认读取 `configs/config.yaml`，其中 MySQL、Redis、RabbitMQ 和 Mailpit 均使用 `127.0.0.1` 映射端口，无需导出环境变量。

先启动 Admin。Admin 会在 HTTP/gRPC 启动前自动执行尚未应用的迁移：

```bash
make run-admin
```

Admin 健康后初始化管理员，再启动前端。管理员资料和初始密码来自 `configs/config.yaml`：

```bash
make init-admin
cd app/frontend
pnpm install
pnpm dev
```

前端开发地址为 <http://127.0.0.1:5173>，Vite 会将 `/api` 代理到 `http://127.0.0.1:8000`。

## 后端服务与运维工具

```bash
make build
./bin/kratos-admin -conf ./configs/config.yaml
./bin/kratos-admin-tools --help
```

`kratos-admin` 是标准 Kratos 服务可执行程序，使用官方 `-conf` 参数；`kratos-admin-tools` 是独立 Cobra 运维工具，提供：

```text
kratos-admin-tools migrate     使用 Goose 执行数据库迁移
kratos-admin-tools init-admin  幂等初始化平台超级管理员
kratos-admin-tools gorm-gen    从 Goose 临时数据库反向生成 Model 与 Query
```

服务默认读取 `configs/config.yaml`，可用 `-conf` 指定配置；`migrate` 和 `init-admin` 可用 `-c/--conf` 指定另一份完整 YAML。

## 配置约定

- `app/admin/internal/conf/conf.proto` 是运行配置结构的真相源。
- `configs/config.yaml` 和 `configs/config.docker.yaml` 使用同一个配置契约，仅区分宿主机与容器网络及路径。
- 应用不读取 `.env` 或系统环境变量；生产发布应替换完整 YAML 并限制文件权限。
- 仓库仅保存本地开发密钥，真实密码、JWT 私钥和 Provider 凭据不得提交 Git。
- Goose SQL 是唯一数据库迁移入口；Admin 启动时使用 MySQL `GET_LOCK` 串行执行，不使用 `AutoMigrate`。
- `make migrate` 仅作为人工排障和受控运维入口，正常部署不依赖独立迁移容器。

## 数据库模型与查询生成

`migrations/*.sql` 是数据库结构的唯一真相源，`app/admin/internal/data/model/*.gen.go` 和 `query/*.gen.go` 均为派生代码，禁止手工修改。修改迁移后执行：

```bash
make gorm-gen
make gorm-gen-check
```

生成工具只复用配置 DSN 的 MySQL 服务器和账号，创建随机临时数据库、执行全部 Goose 迁移、反向生成 Model 与 Query，最后删除临时数据库；不会迁移或清理配置指向的业务数据库。该 MySQL 账号需要临时 `CREATE DATABASE`、`DROP DATABASE` 权限。需要单独指定生成账号时，可设置 `KRATOS_ADMIN_GORM_GEN_DSN`，该变量只供代码生成工具使用。

运行期业务数据访问统一使用 GORM Gen。完整关联对象优先 `Preload`，关联过滤或投影优先 `Join`；确需复杂 SQL 时应针对具体问题评审并封装为自定义 Gen 查询。

## 常用命令

```bash
make help             # 查看全部目标
make all              # 生成 API、配置、Wire 和 GORM Gen
make gorm-gen-check   # 生成并检查 Model/Query 是否幂等
make build            # 构建 bin/kratos-admin 与 bin/kratos-admin-tools
make test             # 后端竞态测试
make vet              # Go 静态检查
make frontend-test    # 前端组件测试
make frontend-build   # 前端类型检查与生产构建
make compose-config   # 校验根级 Compose
```

前端 OpenAPI 客户端生成：

```bash
cd app/frontend
pnpm api:generate
```

## 文档

- [部署、备份、升级与故障排查](docs/deployment.md)
- [工程验收清单](docs/acceptance.md)
- [设计与实施记录](docs/superpowers/)
