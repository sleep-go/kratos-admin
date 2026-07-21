# 部署与运维

## 首次启动

1. 修改 `configs/config.docker.yaml` 中的认证、初始化管理员和 Provider 开发值；生产环境必须替换为受控配置文件。
2. 执行 `make compose-config`，再执行 `make compose-up`。
3. 访问管理后台 `http://127.0.0.1:8080`、API 健康接口 `http://127.0.0.1:8000/api/v1/health`、RabbitMQ 管理台 `http://127.0.0.1:15672` 或 Mailpit `http://127.0.0.1:8025`。

Compose 启动链路为 MySQL/Redis/RabbitMQ/Mailpit → Admin → init-admin → Frontend。Admin 在启动 HTTP/gRPC 前通过 MySQL 命名锁 `kratos_admin_schema_migration` 串行执行 Goose 迁移；迁移失败会阻止该实例启动。RabbitMQ 不属于 API readiness 前置条件，连接失败只会造成后台任务积压。

## 不使用 Docker 启动

本机准备 Go 1.26、MySQL 8、Redis 7、RabbitMQ 4、Node.js 和 pnpm。宿主机配置全部位于 `configs/config.yaml`，无需导出环境变量。

终端一启动 Admin：

```bash
go run ./app/admin/cmd/server -conf ./configs/config.yaml
```

健康检查通过后，终端二初始化管理员：

```bash
go run ./app/admin/cmd/tools init-admin --conf ./configs/config.yaml
```

终端三启动前端：

```bash
cd app/frontend
pnpm install
pnpm dev
```

`make migrate` 保留为人工检查、故障排查或受控维护入口，正常启动不需要先手工执行。修改依赖注入后执行 `make wire`，只生成 Admin 的 `wire_gen.go`。

## Docker Compose 仅启动依赖

```bash
make compose-deps-up
```

该命令只启动 MySQL、Redis、RabbitMQ 和 Mailpit。宿主机进程使用 `127.0.0.1` 及映射端口；Compose 内的 Admin 使用服务名 `mysql`、`redis`、`rabbitmq`。停止依赖使用 `make compose-down`。

## 生产与 Kubernetes

- 生产只部署一个 Admin Deployment，不再部署独立 Worker Deployment 或 migrate Job；通过 `-conf` 指定生产完整 YAML。
- 每个 Admin Pod 都执行相同的启动迁移检查，MySQL 命名锁确保同一时刻只有一个实例应用迁移；其他实例等待后再次检查。
- 发布中的迁移必须向前兼容滚动期间同时运行的新旧版本。破坏性变更应拆成“先扩展、再切换、最后清理”的多个版本。
- RabbitMQ 建议在目标 vhost 上统一默认队列类型为 quorum，并监控三条业务队列、统一死信队列、未确认消息和连接重试日志。
- Admin 使用的数据库账号需要目标库 DDL/DML 权限；应用回滚不会自动回退数据库。
- 对象存储使用私有 Bucket，浏览器仅使用短期直传凭证与下载签名。

## 备份与恢复

升级前备份 MySQL、Redis AOF 和对象存储清单。MySQL 保存异步任务真相、投递时间和重试状态；RabbitMQ 只负责分发。RabbitMQ 数据丢失或短暂不可用时，未完成任务会根据 MySQL 状态重新发布。Redis 仅承载缓存和验证码，不再承担任务队列。

恢复顺序为 MySQL → 对象存储 → Redis/RabbitMQ → Admin。文件元数据与对象必须成对核对，不要只恢复数据库后直接清理“孤儿对象”。

## 升级与回滚

1. 备份并记录当前镜像标签、Goose 版本和配置文件摘要。
2. 部署新 Admin 镜像；各实例在启动阶段通过 MySQL 锁串行检查迁移。
3. 检查健康接口、登录、权限、RabbitMQ 队列、死信、日志导出和文件清理。
4. 只有迁移明确提供安全 `Down` 且已验证数据兼容性时，才允许手工执行 Goose 回退。

## 故障排查

- Admin 启动失败：检查 DSN、MySQL DDL 权限、Redis 地址、32 字节配置密钥及生产 JWT 私钥；查看 `goose_db_version`，不要手工伪造迁移成功版本。
- RabbitMQ 不可用：API 应保持健康；检查连接日志、vhost、权限和 TLS。任务会在 MySQL 积压，恢复后自动补投。
- 日志导出停在等待：检查 `log_exports` 的 `status`、`dispatched_at`、`next_retry_at`，以及 `kratos.admin.log_export.v1` 队列和死信队列。
- 审计停滞：检查 `audit_outbox`、`kratos.admin.audit.v1` 和 `failed_tasks`。
- 文件停在等待清理：检查 `files.cleanup_*` 字段、Provider 权限、`kratos.admin.file_cleanup.v1` 和 `failed_tasks`，不要直接删除元数据。
- 非法消息：检查 `kratos.admin.dead.v1`；修复生产者后再决定是否重放，避免直接反复入队。

## 验收命令

```bash
make api
make config
make wire
make gorm-gen
make test
make vet
make build
make compose-config
docker compose build api init-admin frontend
cd app/frontend && pnpm lint && pnpm typecheck && pnpm test:run && pnpm build
```
