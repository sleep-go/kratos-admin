# 单 Admin 进程验收

## 目录与依赖

- 后端应用仅位于 `app/admin`，Vue 前端位于 `app/frontend`；仓库不存在 `app/worker`。
- `app/admin/cmd/server` 是不依赖 Cobra 的标准 Kratos 服务入口；`app/admin/cmd/tools` 提供 `migrate`、`init-admin`、`gorm-gen` 三个运维子命令。
- Admin 的 Kratos 生命周期同时注册 HTTP、gRPC 和 RabbitMQ task Server。
- `app/admin/internal/data` 不导入 Service/Server，`app/admin/internal/biz` 不导入 Data/Service/Server，Service 不导入 Data/Server。
- 现行 Go 代码不依赖 Asynq，Redis 不承担异步任务队列。

## 后端与生成代码

```bash
make wire
make gorm-gen
make test
make vet
make build
GOCACHE=/tmp/go-build go test ./app/admin/... -count=1
GOCACHE=/tmp/go-build go vet ./app/admin/...
```

依次执行 `make config`、`make wire`、`make gorm-gen`、`make api` 和 `cd app/frontend && pnpm api:generate`，随后确认生成目录没有非预期 Git 差异。

## 数据库迁移

- 全新 MySQL 8 测试库启动 Admin 后，`00001` 至 `00006` 均处于已应用状态。
- 两个 Admin 实例同时启动时，仅一个实例持有 `kratos_admin_schema_migration` 锁执行迁移，两个实例最终均可成功启动。
- 迁移失败必须阻止业务 Server 启动；RabbitMQ 连接失败不得阻止业务 Server 启动。
- `make migrate` 可用于人工运维，但 Compose 和 Kubernetes 不包含独立迁移服务或 Job。

## RabbitMQ 与恢复

- 三个 durable 队列分别为 `kratos.admin.audit.v1`、`kratos.admin.log_export.v1`、`kratos.admin.file_cleanup.v1`。
- 发布启用 persistent、mandatory 和 publisher confirms；消费使用 manual ACK 和有限 prefetch。
- 停止 RabbitMQ 后 API 健康检查仍成功，新任务保留在 MySQL；恢复 RabbitMQ 后任务自动补投并完成。
- 消费连接在未 ACK 时中断，同一 message ID 会重新投递；业务处理结果保持幂等。
- 非法版本或缺少必填字段的消息进入 `kratos.admin.dead.v1`，不反复 requeue。
- 第 10 次可重试失败或永久失败写入 `failed_tasks`，业务记录进入最终失败状态。
- 多实例日志维护通过 `kratos_admin_log_maintenance` 命名锁避免重复清理。

可选真实依赖集成测试：

```bash
KRATOS_ADMIN_TEST_MYSQL_DSN='kratos:kratos@tcp(127.0.0.1:3306)/kratos_admin?charset=utf8mb4&parseTime=True&loc=Local' \
KRATOS_ADMIN_TEST_RABBITMQ_URL='amqp://kratos:kratos@127.0.0.1:5672/kratos_admin' \
GOCACHE=/tmp/go-build go test ./app/admin/internal/data ./app/admin/internal/server/task \
  -run 'Migration|RabbitMQ|Recovery' -count=1 -timeout=60s
```

## Compose 与镜像

```bash
make compose-config
docker compose config --services
docker compose build api init-admin frontend
```

服务集合必须为 `mysql redis rabbitmq mailpit api init-admin frontend`，不得出现 `migrate` 或 `worker`。RabbitMQ 管理端口默认为 `15672`，AMQP 端口默认为 `5672`；Admin 不依赖 RabbitMQ health 才启动。

## 前端

```bash
cd app/frontend
pnpm lint
pnpm typecheck
pnpm test:run
pnpm build
pnpm e2e
```
