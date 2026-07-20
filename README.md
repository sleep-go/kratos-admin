# Kratos Admin

基于 go-kratos 与 Vue 3 的前后端分离、多租户通用管理后台。项目采用 `app/*` 大仓结构，后端通过单一 `kratos-admin` Cobra 命令运行 Admin、Worker 和运维工具，数据库迁移统一由 Goose 管理。

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

- 后端：Go 1.26、go-kratos、GORM Gen、Goose、Casbin、Redis、Asynq
- 前端：Vue 3、Vite、TypeScript、Element Plus、Tailwind CSS、SCSS、Pinia、ECharts
- 数据库：PolarDB MySQL 8；本地开发使用 MySQL 8.4
- 部署：Docker Compose、Nginx、阿里云 OSS

## 目录结构

```text
app/
├── admin/
│   ├── cmd/kratos-admin/       # 统一 Cobra 命令入口
│   └── internal/{server,service}
├── worker/
│   └── internal/{server,service}
└── frontend/                   # Vue 3 前端应用
api/admin/v1/                   # 业务 API Proto
configs/                        # Admin、Worker Kratos YAML
internal/
├── biz/                        # 领域用例与仓储契约
├── conf/                       # Bootstrap 配置 Proto 与加载器
├── data/                       # MySQL、Redis、Casbin、GORM Gen
└── provider/                   # 邮件、短信、存储、密钥 Provider
migrations/                     # Goose SQL 迁移
deploy/Dockerfile.backend       # 后端多阶段镜像
docker-compose.yml              # 完整本地编排
```

API 和 Worker 是独立 Kratos 应用，复用根级 `biz/data/provider`。`kratos-admin` 只是统一运行入口，不会把两个应用合并成同一进程。

## 环境要求

- Go 1.26+
- Node.js 24+ 与 pnpm 10+
- Docker Desktop（使用 Compose 时）
- 本地运行依赖时需要 MySQL 8 和 Redis 7

首次使用先复制环境变量：

```bash
cp .env.example .env
```

至少修改 `.env` 中的：

- `KRATOS_ADMIN_SECRET_KEY`：精确 32 字节。
- `KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD`：不少于 12 位。
- `KRATOS_ADMIN_JWT_PRIVATE_KEY`：生产环境必须配置持久化 Ed25519 私钥。

## Docker Compose 完整启动

根目录 Compose 包含 MySQL、Redis、Mailpit、Goose migration、超级管理员初始化、API、Worker 和 Frontend：

```bash
make compose-config
make compose-up
```

启动顺序为：MySQL 健康检查 → Goose 迁移 → 幂等管理员初始化 → API/Worker → Frontend。

访问地址：

- 管理后台：<http://127.0.0.1:8080>
- API 健康检查：<http://127.0.0.1:8000/api/v1/health>
- Mailpit：<http://127.0.0.1:8025>

停止服务：

```bash
make compose-down
```

## Compose 启动依赖，前后端本地运行

只启动 MySQL、Redis 和 Mailpit：

```bash
make compose-deps-up
```

Compose 内部使用 `mysql:3306` 和 `redis:6379`，宿主机 Go 进程必须使用 `127.0.0.1`：

```bash
export KRATOS_ADMIN_MYSQL_DSN='kratos:kratos@tcp(127.0.0.1:3306)/kratos_admin?charset=utf8mb4&parseTime=True&loc=Local'
export KRATOS_ADMIN_REDIS_ADDR='127.0.0.1:6379'
export KRATOS_ADMIN_SECRET_KEY='0123456789abcdef0123456789abcdef'
```

安装 Goose、执行迁移并初始化管理员：

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
goose -dir migrations mysql "$KRATOS_ADMIN_MYSQL_DSN" up
KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD='replace-with-strong-password' \
  go run ./app/admin/cmd/kratos-admin init-admin --conf ./configs/admin.yaml
```

分别启动 Admin、Worker 和前端：

```bash
make run-admin
```

```bash
make run-worker
```

```bash
cd app/frontend
pnpm install
pnpm dev
```

前端开发地址为 <http://127.0.0.1:5173>，Vite 会将 `/api` 代理到 `http://127.0.0.1:8000`。

## 统一命令行

```bash
make build
./bin/kratos-admin --help
```

可用子命令：

```text
kratos-admin server      启动 Admin HTTP/gRPC
kratos-admin worker      启动 Asynq Worker
kratos-admin init-admin  幂等初始化平台超级管理员
kratos-admin gorm-gen    生成 GORM Gen 查询代码
```

Admin 默认读取 `configs/admin.yaml`，Worker 默认读取 `configs/worker.yaml`；可通过 `-c/--conf` 指定其他文件。

## 配置约定

- `internal/conf/conf.proto` 是运行配置结构的真相源。
- `configs/*.yaml` 保存结构和安全默认值，并使用 `${KRATOS_ADMIN_*}` 占位符接收部署覆盖。
- 密钥、密码和 Provider 凭据只通过环境变量或外部密钥服务提供，不写入仓库。
- Goose 是唯一数据库迁移入口，API 和 Worker 启动时不会执行 `AutoMigrate`。

## 常用命令

```bash
make help             # 查看全部目标
make all              # 生成 API、配置、Wire 和 GORM Gen
make build            # 构建 bin/kratos-admin
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
