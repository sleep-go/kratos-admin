# Kratos Admin

Kratos Admin 是基于 go-kratos 与 Vue 3 的前后端分离、多租户通用管理后台。

## 目标能力

- 原生多租户与平台治理
- 用户、部门、岗位和租户成员管理
- Casbin domain RBAC、动态菜单及仓储强制数据范围
- 登录、操作审计和 API 访问日志
- 参数字典、系统设置及可插拔邮件/短信渠道
- 本地文件与阿里云 OSS 对象存储
- API 与 Worker 分进程运行

## 技术栈

- 后端：Go 1.26、go-kratos、GORM Gen、Goose、Casbin、Redis、Asynq
- 前端：Vue 3、TypeScript、Element Plus、Tailwind CSS、SCSS、Pinia
- 数据库：PolarDB MySQL 8（本地开发使用 MySQL 8）

## 大仓目录

- `app/admin`：Admin HTTP/gRPC 应用及统一 `kratos-admin` Cobra 命令入口。
- `app/worker`：由 `kratos-admin worker` 启动的 Asynq Worker 应用。
- `configs`：Admin 与 Worker 的 Kratos YAML 运行配置。
- `internal/conf`：由 Proto 定义并生成的 Bootstrap 配置契约。
- `internal/biz`：共享领域用例与仓储契约。
- `internal/data`：MySQL、Redis、Casbin 和 GORM Gen 仓储实现。
- `internal/provider`：SMTP、阿里云短信、本地文件、阿里云 OSS 和密钥实现。
- `migrations`：Goose 数据库迁移，服务启动不会执行 `AutoMigrate`。

详细设计和实施计划见 `docs/superpowers/`。

本地与生产部署、备份、升级及故障排查见 [`docs/deployment.md`](docs/deployment.md)。首次 Compose 启动会按 MySQL → Goose → 幂等超级管理员初始化 → API/Worker → Frontend 的顺序执行。

## Docker Compose 启动依赖，前后端本地启动

复制环境变量文件，并只启动 MySQL、Redis 与 Mailpit：

```bash
cp .env.example .env
make compose-deps-up
```

Compose 中的应用使用 `mysql:3306`，宿主机运行的 Go 进程必须改用 `127.0.0.1:3306`。在所有后端终端导出本机连接配置，然后执行 Goose 迁移和幂等初始化：

```bash
export KRATOS_ADMIN_MYSQL_DSN='kratos:kratos@tcp(127.0.0.1:3306)/kratos_admin?charset=utf8mb4&parseTime=True&loc=Local'
export KRATOS_ADMIN_REDIS_ADDR='127.0.0.1:6379'
export KRATOS_ADMIN_SECRET_KEY='0123456789abcdef0123456789abcdef'
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
goose -dir migrations mysql "$KRATOS_ADMIN_MYSQL_DSN" up
KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD='replace-with-strong-password' go run ./app/admin/cmd/kratos-admin init-admin --conf ./configs/admin.yaml
```

随后分别启动三个常驻开发进程；Admin Server 与 Worker 终端都需要具备上述环境变量：

```bash
go run ./app/admin/cmd/kratos-admin server --conf ./configs/admin.yaml
go run ./app/admin/cmd/kratos-admin worker --conf ./configs/worker.yaml
cd frontend && pnpm install && pnpm dev
```

所有后端能力都由一个二进制提供，可先执行 `make build`，再使用 `./bin/kratos-admin server|worker|init-admin|gorm-gen`。也可以分别使用 `make run-admin`、`make run-worker`。YAML 管理配置结构与安全默认值，`${KRATOS_ADMIN_*}` 环境变量只负责部署差异和敏感值。执行 `make help` 可查看官方语义目标及大仓扩展目标。

## 配置与 Provider

- 系统设置按代码安全默认、平台默认、租户允许覆盖值三级解析。
- 敏感设置及 Provider JSON 使用 AES-256-GCM 加密，读取接口仅返回配置状态。
- 内置本地邮件/短信模拟器、SMTP、阿里云短信、本地文件及阿里云 OSS。
- SMTP、短信与 OSS 连接测试使用已保存密文配置；阿里云短信只查询已配置签名，不发送付费短信。

五类数据范围支持租户内全部、本部门及下级、本部门、仅本人和自定义部门。多角色取并集，但成员、部门、文件和日志查询始终先强制认证上下文中的 `tenant_id`，前端提交的租户或数据范围参数不会被信任。

## 前端体验

- 工作台按平台/租户视角读取真实成员、角色、API 日志、异常请求和审计动态，并以 ECharts 展示七日请求趋势。
- 菜单由后端授权资源动态生成，只能映射到前端编译期注册的组件键；平台治理视角不会暴露租户组织与角色入口。
- 角色页面一次提交 Casbin 资源动作、数据范围和自定义部门，避免权限版本在多次请求间失效；平台可通过功能树原子配置租户可用能力。
- 个人中心支持资料更新、MFA 状态查看、设备会话列表及会话撤销；桌面、平板和手机均可切换租户。

## 文件生命周期

- 文件采用“预登记、短期签名直传、服务端元数据校验”链路，私有下载每次重新校验租户、资源权限与数据范围。
- 业务模块通过文件引用接口绑定或解绑资源；仍被引用的文件不能删除。
- 删除请求只进入待清理状态，由 Worker 幂等删除对象并完成软删除；重试耗尽或 Provider 不匹配时标记为清理失败，避免元数据提前消失。
