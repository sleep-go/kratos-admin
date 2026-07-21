# Kratos Admin 单 YAML 配置设计

## 背景与目标

Kratos Admin 正在由独立 Admin、Worker 应用调整为单一 Admin Server：HTTP、gRPC、Asynq Worker 和定时任务由同一进程统一启动。配置也随应用边界收口，只保留一套配置契约，不再依赖 `.env` 或系统环境变量提供运行参数。

本设计必须在 Worker 合并改造完成后实施，避免两个会话同时修改 `app/worker`、启动装配、Compose 和配置文件。

## 配置结构与加载

- `internal/conf/conf.proto` 是配置结构和字段类型的唯一真相源。
- `configs/config.yaml` 是宿主机开发配置，依赖地址使用 `127.0.0.1`。
- `configs/config.docker.yaml` 是完整 Compose 配置，依赖地址使用 `mysql`、`redis`、`mailpit` 服务名，文件路径使用容器路径。
- 两个文件使用同一个 `conf.proto`，仅解决宿主机和容器网络、路径差异，不是 Admin/Worker 双配置。
- 配置加载器只注册 Kratos file source，删除 env source 和 `${KRATOS_ADMIN_*}` 占位符解析。
- Cobra 根命令下需要配置的子命令统一默认读取 `./configs/config.yaml`，仍允许通过 `-c/--conf` 指定另一个完整 YAML 文件。
- 服务启动时继续执行现有必填项、格式和生产环境校验；配置错误应在启动阶段返回明确错误，不允许静默使用环境变量兜底。

## 初始化管理员

在 `conf.proto` 中增加初始化管理员配置，至少包含用户名、显示名称和初始密码。`init-admin` 从 `config.yaml` 读取这些值，不再读取 `KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD`。

初始化操作继续保持幂等。初始密码必须满足现有密码强度规则，缺失或不合法时命令直接失败。

## Compose 与 Makefile

- 根 `docker-compose.yml` 删除所有 service `env_file`。
- MySQL 数据库名、用户名、密码、映射端口以及 Redis、Mailpit、API、gRPC、Frontend 端口直接写入 Compose。
- Admin Server、迁移和初始化命令统一读取 `/app/configs/config.docker.yaml`，不再启动独立 Worker 服务。
- Makefile 删除 `COMPOSE_ENV_FILE`、`KRATOS_ADMIN_ENV_FILE` 和 `--env-file`；本地运行、迁移、初始化和服务启动统一引用 `configs/config.yaml`。
- Goose 仍是唯一数据库迁移机制，不引入 GORM AutoMigrate。沿用 Worker 合并后已有的嵌入式迁移和 MySQL 命名锁：Admin 启动时迁移，`migrate` 子命令提供人工运维入口，Compose 不新增迁移服务。

## 文件收口

- 新增宿主机 `configs/config.yaml` 和容器 `configs/config.docker.yaml`。
- 删除 `configs/admin.yaml`、`configs/worker.yaml` 和 `.env.example`。
- 删除应用代码、Makefile、Compose、README、部署及验收文档中的 `KRATOS_ADMIN_*`、旧配置文件和独立 Worker 启动说明。
- 保留与 Go 工具链自身相关且不属于应用运行配置的环境变量用法，例如验证时使用的 `GOCACHE`。

## 安全边界

仓库内两个 YAML 只提供可启动的本地开发值，不写入真实生产密钥。生产环境必须在部署阶段替换整个配置文件，并限制文件权限；真实数据库密码、JWT 私钥、OSS AccessKey、短信和邮件凭据不得提交 Git。

由于用户明确选择纯 YAML，本项目不再提供环境变量覆盖能力。需要多环境部署时，通过不同的完整 YAML 文件配合 `--conf` 切换。

## 测试与验收

- 配置测试验证完整 YAML 能解析，并验证必填值、时长、密钥和初始化密码错误。
- 架构契约测试验证 `configs/config.yaml`、`configs/config.docker.yaml` 存在，旧两个 YAML 与 `.env.example` 不存在，配置代码不再注册 env source。
- Cobra 测试验证相关子命令默认使用 `configs/config.yaml`，且 `--conf` 覆盖仍生效。
- Compose 配置在没有 `.env` 的全新检出中直接通过解析，并且不存在独立 Worker 服务。
- 执行 Go 测试、vet、前端 lint/typecheck/test/build 和 Compose 镜像构建。

## 实施顺序与并发约束

1. 等待另一个会话完成 Worker 并入 Admin Server，并确认其提交已经出现在当前分支。
2. 基于合并后的启动结构修改配置契约、加载器和 Cobra 默认值。
3. 收口 YAML、Compose、Makefile、README 和部署文档。
4. 验证 Admin Server 同时提供 API 与后台任务能力。
5. 提交本次单配置改造；不得覆盖或回滚另一个会话的 Worker 修改。
