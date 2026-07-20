# 单 Admin 进程、RabbitMQ 异步任务与启动迁移设计

## 1. 背景与目标

当前后端由 Admin Server 与独立 Worker 两个进程组成。Worker 使用 Redis/Asynq 消费审计发布、日志导出和文件清理任务；数据库迁移由 Docker Compose 中独立的 `migrate` 一次性服务执行。

本次调整目标如下：

- 后端只保留一个 `kratos-admin server` 常驻进程。
- Admin 启动阶段使用 MySQL 命名锁串行执行 Goose 迁移，迁移成功后才启动业务服务。
- Admin 进程内启动 RabbitMQ 发布与消费协程，替代独立 Worker 和 Redis/Asynq 队列。
- MySQL 保存异步任务真相、投递状态和重试状态；RabbitMQ 只负责可靠分发。
- RabbitMQ 暂时不可用时，HTTP/gRPC 服务仍可启动并提供服务，待办任务不得丢失。
- 支持多个 Admin 实例同时启动、发布和消费，保证至少一次投递与业务幂等。

本次不移除 Redis。Redis 继续承载验证码等非异步队列能力。本次也不改变三类任务的业务结果、日志导出上限、文件生命周期和日志保留规则。

## 2. 总体架构

统一后的 Admin 进程包含四类 Kratos 生命周期组件：

1. 启动迁移器：在应用依赖组装前执行，仅迁移成功后继续启动。
2. HTTP/gRPC Server：保留现有 API 对外契约。
3. RabbitMQ 后台运行时：管理连接恢复、拓扑声明、发布扫描和三类消费者。
4. 周期维护器：执行日志保留清理等原 Worker 周期任务。

RabbitMQ 后台运行时以 Kratos Server 组件接入统一生命周期。它的 `Start` 只启动受控协程，不因 RabbitMQ 首次连接失败而阻止 API 启动；连接 supervisor 在后台持续重连。`Stop` 停止新发布和新消费，等待在途任务至退出超时，再关闭 AMQP 连接。

删除以下旧边界，不保留兼容入口：

- `kratos-admin worker` Cobra 子命令。
- `app/worker` 应用、Wire 图、Service 和 Server。
- Worker Docker 构建阶段、Compose 服务及 Makefile 启动命令。
- `github.com/hibiken/asynq` 依赖和 `Data.AsynqClient`。

原 Worker 的任务适配逻辑迁入 Admin 内部后台任务包；领域处理器和 Data 仓储继续复用，不复制业务逻辑。

## 3. Admin 启动迁移

### 3.1 启动顺序

`kratos-admin server` 按以下顺序启动：

1. 加载并校验 Admin 配置。
2. 创建专用 MySQL 连接并完成 Ping。
3. 在专用 `*sql.Conn` 上执行 `GET_LOCK('kratos_admin_schema_migration', 300)`。
4. 获得锁后，使用 Goose 对编译进二进制的迁移文件执行 `up`。
5. 成功后执行 `RELEASE_LOCK` 并关闭迁移连接。
6. 通过现有 Wire 图组装 MySQL、Redis、Provider、HTTP/gRPC 和 RabbitMQ 后台运行时。
7. 启动统一 Kratos App。

所有 Admin 实例都会进入迁移检查，但同一时刻只有持有 MySQL 锁的实例可以执行 Goose。后续实例获得锁后再次执行 `up`，无待执行版本时为空操作。

### 3.2 失败语义

- MySQL 不可用、300 秒内未获得锁、Goose 执行失败或无法确认锁释放时，Admin 启动失败并返回中文上下文错误。
- 持锁连接断开时，MySQL 自动释放命名锁。
- Admin 不在 HTTP/gRPC 已监听后执行迁移。
- 数据库迁移不自动执行 `down`；应用回滚也不自动回退数据库。
- 迁移 SQL 必须按向前兼容的 expand/contract 方式设计，允许滚动发布期间旧实例短暂共存。

### 3.3 迁移文件分发

根级 `migrations` 增加 Go 文件并使用 `go:embed` 嵌入同目录 SQL。运行镜像只需统一的 `kratos-admin` 二进制，不再安装 Goose CLI 或复制外部迁移目录。

Goose CLI 和 `make migrate` 继续保留，作为人工排障、查看状态和受控运维入口；正常部署不依赖独立迁移镜像。

## 4. RabbitMQ 配置与拓扑

### 4.1 配置契约

`DataConfig` 新增 RabbitMQ 配置：

- `url`：AMQP URL，通过 `KRATOS_ADMIN_RABBITMQ_URL` 注入。
- `prefetch`：单消费者未确认消息上限，默认 `10`。
- `concurrency`：每类任务的最大并发协程数，默认 `10`。

本地宿主机默认 URL 为 `amqp://kratos:kratos@127.0.0.1:5672/kratos_admin`；Compose 使用主机名 `rabbitmq`。生产凭据只能通过 Secret 或环境变量提供，不写入 YAML。

### 4.2 稳定拓扑

应用声明以下持久化拓扑，名称不开放为部署配置：

| 类型 | 名称 | Routing key |
| --- | --- | --- |
| Direct exchange | `kratos.admin.tasks.v1` | - |
| 审计队列 | `kratos.admin.audit.v1` | `audit.publish.v1` |
| 日志导出队列 | `kratos.admin.log_export.v1` | `log.export.v1` |
| 文件清理队列 | `kratos.admin.file_cleanup.v1` | `file.cleanup.v1` |
| Dead-letter exchange | `kratos.admin.tasks.dlx.v1` | 原 routing key |
| 统一死信队列 | `kratos.admin.dead.v1` | 三个 routing key |

交换机和队列均为 durable、非 exclusive、非 auto-delete。应用不硬编码 classic 或 quorum 类型；本地使用 RabbitMQ vhost 默认队列类型，生产在首次声明队列前将目标 vhost 的默认队列类型统一设置为 quorum，避免应用实例声明参数不一致。

### 4.3 消息格式

消息使用 JSON，公共属性固定为：

- `message_id`：任务幂等键，例如 `audit:<event_id>`。
- `type`：routing key。
- `content_type`：`application/json`。
- `delivery_mode`：persistent。
- body：版本化任务载荷，只保存定位业务记录所需的 ID 和文件清理必要快照。

三类载荷沿用现有字段：审计为 `event_id`，日志导出为 `export_id`，文件清理为 `tenant_id`、`file_id`、`provider_name`、`object_key`，并统一增加 `version: 1`。

## 5. MySQL 任务真相与状态机

RabbitMQ 消息采用至少一次投递，MySQL 记录决定任务是否需要发布、是否可以重试以及是否已经完成。

新增迁移文件补充以下字段及中文 COMMENT：

- `audit_outbox`：增加 `dispatched_at`、`last_error`；状态扩展为 `1待处理、2已完成、3等待重试、4最终失败`。
- `log_exports`：增加 `dispatched_at`，沿用现有 `retry_count`、`next_retry_at`、`failure_reason` 和四种业务状态。
- `files`：增加 `cleanup_dispatched_at`、`cleanup_retry_count`、`cleanup_next_retry_at`、`cleanup_failure_reason`；沿用 `5待清理、4清理失败` 状态。

相关待投递索引同步包含状态、下次重试时间、投递时间和创建/更新时间。生成模型和 GORM Gen 查询代码同步更新。

### 5.1 发布状态

发布扫描器每 2 秒分批读取最多 100 条到期且 `dispatched_at IS NULL` 的记录。发布时启用 `mandatory` 与 publisher confirms：

- Broker 确认消息已路由并接管后，更新对应记录的 `dispatched_at`。
- 未路由、收到 NACK、超时或连接断开时，不更新 `dispatched_at`，后续扫描继续补投。
- 若消息已确认但进程在更新 MySQL 前退出，允许产生重复消息；消费者必须幂等。

多个 Admin 实例允许同时扫描。重复发布不影响业务正确性，不以跨 MySQL/RabbitMQ 分布式事务为目标。

### 5.2 消费状态

消费者使用 manual ack 和有限 prefetch：

- 业务处理成功或发现任务已经完成：ACK。
- 可重试业务错误：在 MySQL 原子增加重试次数、写入失败原因和指数退避后的 `next_retry_at`、清空 `dispatched_at`，保存成功后 ACK；发布扫描器到期后重新投递。
- 达到 10 次重试：写入既有 `failed_tasks` 并将业务记录置为最终失败，保存成功后 ACK。
- 重试或最终失败状态无法写入 MySQL：NACK 并 requeue，避免 RabbitMQ 删除唯一可用消息。
- 载荷 JSON 非法、版本不支持或必填 ID 缺失：Reject 且不 requeue，由 DLX 进入统一死信队列。

重试延迟沿用指数退避，首轮 1 分钟，最大 64 分钟。失败原因最长保存 1024 字符。

### 5.3 崩溃恢复与幂等

- 审计发布继续通过 Outbox 行锁和 `audit_logs.event_id` 唯一键幂等。
- 日志导出继续通过行锁原子领取；后台扫描器将超过 10 分钟仍为处理中状态的任务恢复为待处理并清空 `dispatched_at`，避免进程崩溃后永久卡住。
- 文件清理按文件状态条件更新；对象删除按“目标不存在等同已删除”处理，重复消息不会重复改变终态。
- RabbitMQ 连接或 channel 断开时，未 ACK 消息由 Broker 重新投递；连接 supervisor 使用有上限的指数退避重新建连、声明拓扑、恢复发布和消费。

## 6. 周期维护与进程生命周期

日志保留清理迁入 Admin 后台运行时，仍按小时检查。多实例通过短时 MySQL 命名锁 `kratos_admin_log_maintenance` 保证同一周期只有一个实例执行；获取不到锁时跳过本轮，不阻塞服务。

RabbitMQ 不可用不影响 Admin readiness。健康接口继续反映 HTTP 服务与核心 MySQL/Redis 依赖；RabbitMQ 连接状态通过中文结构化日志暴露，记录连接成功、断开、重连、发布失败、消费失败和死信原因，不记录 AMQP URL 中的密码或完整任务敏感载荷。

优雅停机顺序如下：

1. Kratos 停止接收新 HTTP/gRPC 请求。
2. 停止发布扫描器和注册新消费任务。
3. 等待在途任务至统一退出超时。
4. 未完成且未 ACK 的消息交由 RabbitMQ 重新投递。
5. 关闭 AMQP、Redis 和 MySQL 连接。

## 7. Compose、Kubernetes 与开发命令

Compose 新增 `rabbitmq:4.3.2-management`、持久卷、AMQP 端口 `5672`、管理端口 `15672` 和 `rabbitmq-diagnostics ping` 健康检查。`.env.example` 提供本地用户、密码、vhost、端口和 Admin AMQP URL。

Compose 删除 `migrate` 与 `worker` 服务，启动顺序改为：

```text
MySQL/Redis/Mailpit/RabbitMQ
  -> Admin 启动时加锁迁移并开始监听
  -> init-admin 等待 Admin 健康后幂等初始化
  -> Frontend 等待 init-admin 成功
```

RabbitMQ 不作为 Admin 的 `service_healthy` 启动前置条件，以验证和保留降级启动能力。`compose-deps-up` 增加 RabbitMQ。

Kubernetes 只需要 Admin Deployment，不再部署 Worker Deployment 或迁移 Job。每个 Pod 都执行相同的启动迁移检查，RabbitMQ 自动在多个 Pod 的消费者之间分发任务。生产发布仍需确保迁移向前兼容，并监控迁移失败、RabbitMQ 未确认发布、队列堆积、死信和 MySQL 最终失败任务。

## 8. 测试与验收

### 8.1 单元测试

- 迁移器：获得锁、锁超时、锁返回异常、Goose 失败、释放锁，以及失败时不启动 Admin。
- 配置：RabbitMQ URL、prefetch、concurrency 默认值与环境变量覆盖。
- 发布器：confirm 后标记投递，NACK/return/断线不标记，重复扫描保持安全。
- 消费器：成功 ACK、重试入库后 ACK、重试入库失败 NACK、非法载荷进入死信、达到 10 次进入最终失败。
- Supervisor：连接失败退避、断线重建拓扑、停止时不再创建新连接。
- 三类处理器：保留原 Worker 测试，并补充重复消息、过期处理中日志导出和对象已不存在的文件清理。
- 命令与架构：`worker` 子命令不存在，Admin 不再导入 `app/worker` 或 Asynq。

### 8.2 集成与部署验收

- 全新 MySQL 并发启动两个 Admin，Goose 版本全部应用且两个实例最终都能启动。
- RabbitMQ 停止时 Admin 健康；创建的任务留在 MySQL。RabbitMQ 恢复后，三类任务自动补投并完成。
- 发布确认后主动中断进程，重启后可能收到重复消息，但审计、导出和文件清理最终结果唯一。
- 消费中断开 AMQP 连接，未 ACK 消息由其他实例或重连后的实例继续处理。
- `docker compose config --quiet`、定向 Go 测试、`go test ./...`、`go vet ./...` 和后端构建通过。
- README、部署、验收和架构文档不再把 Worker、Asynq 或独立 migrate 服务描述为现行入口。

## 9. 约束与明确取舍

- RabbitMQ 使用官方 `github.com/rabbitmq/amqp091-go` 客户端，不引入额外自动重连封装依赖。
- 使用至少一次投递，不承诺 exactly-once；正确性由 publisher confirms、manual ack、MySQL 状态和业务幂等共同保证。
- RabbitMQ 故障不会扩大为 API 全站不可用，但异步任务会积压并产生可观测日志。
- 直接删除旧 Worker 入口，不兼容旧部署脚本。
- 不新建通用异步任务表；复用现有三类业务真相表和 `failed_tasks`。
- 不在本次调整中移除 Redis、改变前端 API 契约或重构无关业务模块。

## 10. 参考资料

- RabbitMQ Reliability Guide：publisher confirms、consumer acknowledgements 与重复投递边界。<https://www.rabbitmq.com/docs/reliability>
- RabbitMQ Consumer Acknowledgements and Publisher Confirms：manual ack、prefetch 和 confirm 语义。<https://www.rabbitmq.com/docs/confirms>
- RabbitMQ Quorum Queues：生产高可用队列、publisher confirms 与 manual ack 建议。<https://www.rabbitmq.com/docs/quorum-queues>
- RabbitMQ Release Information：4.3 为当前完全支持系列。<https://www.rabbitmq.com/release-information>
