# Unified Admin RabbitMQ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将独立 Worker 与 Compose migrate 服务收敛进单一 Admin 进程，由 Admin 启动时通过 MySQL 命名锁执行 Goose 迁移，并在进程内使用 RabbitMQ 可靠发布和消费三类异步任务。

**Architecture:** MySQL 保存异步任务真相、投递和重试状态，RabbitMQ 使用三个 durable queue 提供至少一次分发。Admin 的 Kratos 生命周期同时托管 HTTP/gRPC、AMQP 重连 supervisor、发布扫描器、消费者和日志维护；RabbitMQ 故障只造成任务积压，不阻止 API 启动。

**Tech Stack:** Go 1.26、go-kratos v2.9.2、Goose v3.26.0、MySQL 8、RabbitMQ 4.3.2、`github.com/rabbitmq/amqp091-go` v1.10.0、GORM/GORM Gen、Google Wire。

## Global Constraints

- 仅在 `/Volumes/sleepCard/workspace/sleep-go/kratos-admin/.worktrees/unified-admin-rabbitmq` 和分支 `codex/unified-admin-rabbitmq` 实施。
- 所有生产代码严格遵循 RED → GREEN → REFACTOR；每个行为先看到测试因缺少该行为而失败。
- Go 导出标识必须有以名称开头的中文 GoDoc；运行日志、错误和关键业务注释使用中文。
- 不改变现有 HTTP/gRPC/前端契约，不移除 Redis，不重构无关业务。
- 不保留 `kratos-admin worker`、Worker Wire 图、Worker 镜像或独立 migrate 镜像兼容入口。
- RabbitMQ 故障不得阻止 Admin HTTP/gRPC 启动；MySQL 迁移失败必须阻止启动。
- 消息语义固定为至少一次，使用 publisher confirms、mandatory publish、manual ack 和业务幂等，不宣称 exactly-once。
- 正常任务最多重试 10 次，退避从 1 分钟开始并封顶 64 分钟；非法载荷直接进入死信队列。
- 不主动执行前端全量测试；只运行后端、配置、Compose 和文档相关验证。

---

### Task 1: RabbitMQ 配置契约与固定依赖

**Files:**
- Modify: `internal/conf/conf.proto`
- Modify: `internal/conf/config.go`
- Modify: `internal/conf/config_test.go`
- Modify: `configs/admin.yaml`
- Modify: `.env.example`
- Regenerate: `internal/conf/conf.pb.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `conf.Data.RabbitMQURL string`、`conf.Data.RabbitMQPrefetch int`、`conf.Data.RabbitMQConcurrency int`。
- Produces: 环境变量 `KRATOS_ADMIN_RABBITMQ_URL`、`KRATOS_ADMIN_RABBITMQ_PREFETCH`、`KRATOS_ADMIN_RABBITMQ_CONCURRENCY`。

- [ ] **Step 1: 写配置失败测试**

在 `internal/conf/config_test.go` 的最小 YAML 中加入 RabbitMQ，并断言显式值和默认值：

```go
func TestLoadRabbitMQConfig(t *testing.T) {
	path := writeConfig(t, `
data:
  database:
    source: user:password@tcp(mysql:3306)/admin
  redis:
    addr: redis:6379
  rabbitmq:
    url: amqp://user:password@rabbitmq:5672/admin
    prefetch: 20
    concurrency: 8
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.RabbitMQURL != "amqp://user:password@rabbitmq:5672/admin" || cfg.Data.RabbitMQPrefetch != 20 || cfg.Data.RabbitMQConcurrency != 8 {
		t.Fatalf("rabbitmq config = %+v", cfg.Data)
	}
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `GOCACHE=/tmp/go-build go test ./internal/conf -run RabbitMQ -count=1`

Expected: FAIL，`conf.Data` 尚无 RabbitMQ 字段。

- [ ] **Step 3: 扩展 Proto、运行配置与 YAML**

在 `DataConfig` 中加入：

```proto
message RabbitMQ {
  string url = 1;
  int32 prefetch = 2;
  int32 concurrency = 3;
}
RabbitMQ rabbitmq = 3;
```

在 `conf.Data` 中加入三个字段，默认 URL 为 `amqp://kratos:kratos@127.0.0.1:5672/kratos_admin`，prefetch/concurrency 默认均为 `10`。`configs/admin.yaml` 使用 `${KRATOS_ADMIN_RABBITMQ_*:...}` 占位符；`.env.example` 使用 Compose 主机名 `rabbitmq`。

- [ ] **Step 4: 生成配置代码并固定依赖**

Run:

```bash
make config
go get github.com/pressly/goose/v3@v3.26.0
go get github.com/rabbitmq/amqp091-go@v1.10.0
go mod tidy
```

Expected: `conf.pb.go` 包含 `DataConfig_RabbitMQ`；Goose 和 AMQP 客户端成为直接依赖。

- [ ] **Step 5: 运行配置回归测试**

Run: `GOCACHE=/tmp/go-build go test ./internal/conf -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/conf configs/admin.yaml .env.example go.mod go.sum
git commit -m "配置：新增RabbitMQ连接参数"
```

---

### Task 2: 使用 MySQL 命名锁执行内嵌 Goose 迁移

**Files:**
- Create: `migrations/embed.go`
- Create: `internal/data/migration.go`
- Create: `internal/data/migration_test.go`
- Modify: `app/admin/cmd/kratos-admin/run.go`
- Test: `app/admin/cmd/kratos-admin/run_test.go`

**Interfaces:**
- Produces: `migrations.Files embed.FS`。
- Produces: `data.Migrate(ctx context.Context, dsn string) error`。
- `runServer` 在 `adminapp.NewApplication` 前调用 `data.Migrate`。

- [ ] **Step 1: 为锁状态机写失败测试**

将数据库操作收敛为可替换的小接口：

```go
type lockRow interface { Scan(...any) error }
type lockConnection interface {
	QueryRowContext(context.Context, string, ...any) lockRow
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}
```

测试覆盖返回 `1` 获锁、返回 `0` 超时、Scan 错误、Goose 错误和释放锁错误；测试中的 fake 记录调用顺序必须为 `GET_LOCK -> up -> RELEASE_LOCK`。

- [ ] **Step 2: 运行测试并确认 RED**

Run: `GOCACHE=/tmp/go-build go test ./internal/data -run Migration -count=1`

Expected: FAIL，迁移 API 尚不存在。

- [ ] **Step 3: 实现最小迁移器**

`migrations/embed.go`：

```go
// Package migrations 提供编译进 Admin 二进制的 Goose SQL 迁移文件。
package migrations

import "embed"

// Files 保存所有版本化 SQL 迁移，供服务启动和测试使用。
//go:embed *.sql
var Files embed.FS
```

`data.Migrate` 使用 `database/sql` 与 MySQL driver 打开独立连接池，在专用 `*sql.Conn` 上获取 `kratos_admin_schema_migration` 锁，调用 `goose.SetBaseFS(migrations.Files)` 和 `goose.UpContext(ctx, db, ".")`。使用 `defer` 释放锁；任一步失败都返回中文 `%w` 错误，锁结果不是 `1` 时返回明确超时错误。

- [ ] **Step 4: 验证迁移测试 GREEN**

Run: `GOCACHE=/tmp/go-build go test ./internal/data -run Migration -count=1`

Expected: PASS。

- [ ] **Step 5: 写 runServer 启动顺序失败测试**

在 runner 依赖中注入 `migrate` 与 `newAdmin` 函数，断言迁移失败时不会调用 `newAdmin`，成功时顺序为 `migrate,newAdmin,run`。

- [ ] **Step 6: 接入 Admin 启动并验证**

在 `runServer` 加载配置后立即调用 `data.Migrate(ctx, cfg.Data.MySQLDSN)`；保留 `OpenMySQL` 不执行 AutoMigrate 的语义。运行：

`GOCACHE=/tmp/go-build go test ./app/admin/cmd/kratos-admin ./internal/data -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add migrations internal/data/migration.go internal/data/migration_test.go app/admin/cmd/kratos-admin
git commit -m "数据库：启动时加锁执行迁移"
```

---

### Task 3: 为 RabbitMQ 投递和重试补充数据库状态

**Files:**
- Create: `migrations/00006_rabbitmq_task_state.sql`
- Modify: `internal/data/model/schema.go`
- Modify: `internal/data/model/schema_test.go`
- Regenerate: `internal/data/query/audit_outbox.gen.go`
- Regenerate: `internal/data/query/files.gen.go`
- Regenerate: `internal/data/query/log_exports.gen.go`

**Interfaces:**
- `AuditOutbox` 新增 `DispatchedAt *time.Time`、`LastError string`。
- `LogExport` 新增 `DispatchedAt *time.Time`。
- `File` 新增 `CleanupDispatchedAt *time.Time`、`CleanupRetryCount uint32`、`CleanupNextRetryAt *time.Time`、`CleanupFailureReason string`。

- [ ] **Step 1: 写模型字段失败测试**

在 `schema_test.go` 使用反射断言上述字段存在且表名保持不变，并断言文件状态常量仍包含 `StatusDeletionPending=5`。

- [ ] **Step 2: 运行并确认 RED**

Run: `GOCACHE=/tmp/go-build go test ./internal/data/model -run RabbitMQ -count=1`

Expected: FAIL，字段不存在。

- [ ] **Step 3: 编写可执行迁移 SQL**

`Up` 使用 `ALTER TABLE` 增加字段、中文 COMMENT 和组合索引；同时更新状态字段 COMMENT：

```sql
ALTER TABLE audit_outbox
  ADD COLUMN dispatched_at DATETIME(3) NULL COMMENT '最近一次RabbitMQ确认投递时间' AFTER next_retry_at,
  ADD COLUMN last_error VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '最后一次处理失败原因' AFTER dispatched_at;

ALTER TABLE log_exports
  ADD COLUMN dispatched_at DATETIME(3) NULL COMMENT '最近一次RabbitMQ确认投递时间' AFTER next_retry_at;

ALTER TABLE files
  ADD COLUMN cleanup_dispatched_at DATETIME(3) NULL COMMENT '文件清理任务最近一次RabbitMQ确认投递时间' AFTER status,
  ADD COLUMN cleanup_retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件清理失败重试次数' AFTER cleanup_dispatched_at,
  ADD COLUMN cleanup_next_retry_at DATETIME(3) NULL COMMENT '文件清理下次重试时间' AFTER cleanup_retry_count,
  ADD COLUMN cleanup_failure_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '文件清理最后失败原因' AFTER cleanup_next_retry_at;
```

同时将 `audit_outbox.status` COMMENT 改为 `1待处理，2已完成，3等待重试，4最终失败`，将 `files.status` COMMENT 中的 `5等待Worker清理` 改为 `5等待后台清理`。索引固定为：

```sql
KEY idx_audit_outbox_dispatch (status, next_retry_at, dispatched_at, created_at)
KEY idx_log_exports_pending (status, next_retry_at, dispatched_at, created_at)
KEY idx_files_cleanup (status, cleanup_next_retry_at, cleanup_dispatched_at, updated_at)
```

`Down` 删除 `idx_files_cleanup` 和新增字段，恢复前两个旧索引及旧状态 COMMENT。不要改写旧迁移。

- [ ] **Step 4: 更新模型并重新生成查询**

Run: `make gorm-gen`

Expected: 三个生成查询包含新增字段。

- [ ] **Step 5: 验证模型和 SQL 静态约束**

Run:

```bash
GOCACHE=/tmp/go-build go test ./internal/data/model -count=1
rg -n "COMMENT|dispatched_at|cleanup_retry_count" migrations/00006_rabbitmq_task_state.sql
```

Expected: PASS，所有新增字段均有中文 COMMENT。

- [ ] **Step 6: 提交**

```bash
git add migrations/00006_rabbitmq_task_state.sql internal/data/model internal/data/query
git commit -m "数据库：记录异步任务投递与重试状态"
```

---

### Task 4: 定义稳定消息协议与 RabbitMQ Broker

**Files:**
- Create: `app/admin/internal/task/message.go`
- Create: `app/admin/internal/task/message_test.go`
- Create: `app/admin/internal/task/broker.go`
- Create: `app/admin/internal/task/broker_test.go`

**Interfaces:**
- Produces constants: `ExchangeTasks`、`ExchangeDead`、三个 queue 与 routing key、`QueueDead`。
- Produces `type Message struct { ID, Type string; Body []byte }`。
- Produces `type Delivery struct { Message Message; Ack func() error; Nack func(bool) error; Reject func(bool) error }`，bool 参数表示 requeue。
- Produces `type Broker interface { Publish(context.Context, Message) error; Consume(context.Context, string) (<-chan Delivery, error); Done() <-chan error; Close() error }`。
- Produces `type Connector interface { Connect(context.Context, conf.Data) (Broker, error) }`。

- [ ] **Step 1: 写消息协议失败测试**

测试三个构造函数生成版本 `1`、固定 message ID、routing key 和最小 JSON 字段；空 ID 必须报中文错误。

- [ ] **Step 2: 运行并确认 RED**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/task -run Message -count=1`

Expected: FAIL，包和构造函数尚不存在。

- [ ] **Step 3: 实现消息协议**

定义载荷：

```go
type auditPayload struct { Version uint16 `json:"version"`; EventID string `json:"event_id"` }
type logExportPayload struct { Version uint16 `json:"version"`; ExportID string `json:"export_id"` }
type fileCleanupPayload struct {
	Version uint16 `json:"version"`
	TenantID uint64 `json:"tenant_id"`
	FileID string `json:"file_id"`
	ProviderName string `json:"provider_name"`
	ObjectKey string `json:"object_key"`
}
```

统一由构造函数 JSON 编码，调用方不能自行拼 routing key。

- [ ] **Step 4: 为 AMQP publish 语义写失败测试**

使用窄接口包裹 `amqp091.Channel`，fake channel 断言 `mandatory=true`、`DeliveryMode=Persistent`、`ContentType=application/json`、`MessageId` 和 `Type`；Return、NACK、context 超时均返回错误。

- [ ] **Step 5: 实现 AMQP 0-9-1 Broker**

连接后声明两个 durable direct exchange、三个 durable 主队列、统一死信队列及绑定；主队列设置 `x-dead-letter-exchange=kratos.admin.tasks.dlx.v1`。发布 channel 开启 confirm，等待对应 confirm，并监听 mandatory return。消费 channel 设置 QoS，使用 `autoAck=false`，将 AMQP delivery 适配为内部 `Delivery`。

- [ ] **Step 6: 运行 task 包测试**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/task -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add app/admin/internal/task
git commit -m "异步任务：建立RabbitMQ消息协议与Broker"
```

---

### Task 5: 实现 MySQL 待办发布仓储

**Files:**
- Create: `internal/data/task_repository.go`
- Create: `internal/data/task_repository_test.go`
- Modify: `internal/data/audit_repository.go`
- Modify: `internal/data/log_export_repository.go`
- Modify: `internal/data/file_repository.go`

**Interfaces:**
- Produces `data.PendingTask`，字段为 `Kind`、`ID`、`TenantID`、`ProviderName`、`ObjectKey`。
- Produces `TaskRepository.Pending(ctx, limit, now) ([]PendingTask, error)`。
- Produces `TaskRepository.MarkDispatched(ctx, task, at) error`。
- Produces `TaskRepository.RecordFailure(ctx, task, cause, now) (final bool, error)`。
- Produces `TaskRepository.RecoverStaleLogExports(ctx, before) (int64, error)`。

- [ ] **Step 1: 写待办选择与确认失败测试**

在现有 GORM dry-run 测试风格下验证：只选择到期、未完成且 `dispatched_at IS NULL` 的记录；MarkDispatched 必须带当前状态条件，不能覆盖已完成记录。

- [ ] **Step 2: 运行并确认 RED**

Run: `GOCACHE=/tmp/go-build go test ./internal/data -run TaskRepository -count=1`

Expected: FAIL，仓储不存在。

- [ ] **Step 3: 实现三表待办合并**

每类最多读取 `limit`，合并后按时间排序并截断到 `limit`。固定 Kind 为 `audit`、`log_export`、`file_cleanup`。文件只选择状态 `5`，日志只选择状态 `1`，审计选择状态 `1/3`。

- [ ] **Step 4: 写重试状态失败测试**

断言失败次数从 0 增至 1、`next_retry_at=now+1m`、清空 dispatched_at；第 10 次写入 `failed_tasks` 并进入各自最终失败状态。错误字符串截断至 1024 字符。

- [ ] **Step 5: 实现统一失败记录与卡死恢复**

在单表事务和行锁中更新对应业务记录；最终失败时使用 `failed_tasks.idempotency_key` 唯一键幂等写入。日志导出 `status=2` 且 `started_at < now-10m` 时恢复为 pending，清空 started/dispatched 时间并记录原因。

- [ ] **Step 6: 运行数据层测试**

Run: `GOCACHE=/tmp/go-build go test ./internal/data -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/data
git commit -m "异步任务：实现MySQL待办与重试状态"
```

---

### Task 6: 迁移三类处理器并实现 ACK 状态机

**Files:**
- Create: `app/admin/internal/task/service.go`
- Create: `app/admin/internal/task/service_test.go`
- Create: `app/admin/internal/task/consumer.go`
- Create: `app/admin/internal/task/consumer_test.go`
- Modify: `internal/biz/logexport/export.go`
- Modify: `internal/biz/logexport/export_test.go`
- Modify: `internal/provider/storage/local.go`
- Delete after port: `app/worker/internal/service/tasks.go`
- Delete after port: `app/worker/internal/service/tasks_test.go`

**Interfaces:**
- Produces `Service.Handle(ctx context.Context, message Message) error`。
- Produces sentinel `ErrInvalidMessage` 和 `ErrPermanentTask`。
- Produces `Consumer.Run(ctx context.Context, queue string) error`。

- [ ] **Step 1: 把现有 Worker 处理测试移到 Admin task 包并确认 RED**

保留三类非法载荷、审计处理、日志导出和文件 Provider 不匹配场景；测试直接传内部 `Message`，不再构造 Asynq Task。

- [ ] **Step 2: 实现最小处理适配**

复用 `audit.Processor`、`logexport.Processor`、`FileRepository` 和 storage Provider。日志导出 Processor 不再自行写 Retry；它只返回处理错误，由 Consumer 统一调用 `TaskRepository.RecordFailure`。文件对象不存在视为删除成功。

- [ ] **Step 3: 写 ACK/NACK/Reject 失败测试**

覆盖：

```text
Handle成功                     -> Ack
任务已完成                     -> Ack
Handle失败且RecordFailure成功   -> Ack
RecordFailure失败              -> Nack(requeue)
ErrInvalidMessage              -> Reject(no requeue)
ErrPermanentTask且终态写入成功  -> Ack
```

fake Delivery 必须记录且只允许调用一次终结动作。

- [ ] **Step 4: 实现 Consumer**

Consumer 从 Broker channel 读取，使用有界 worker semaphore 控制并发；每条消息从接收 context 派生任务 context。非法消息 Reject，其他错误按 MySQL 状态机处理。所有日志包含 task type 与 message ID，不输出完整载荷。

- [ ] **Step 5: 运行处理与领域测试**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/task -count=1
GOCACHE=/tmp/go-build go test ./internal/biz/logexport ./internal/provider/storage -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add app/admin/internal/task internal/biz/logexport internal/provider/storage app/worker/internal/service
git commit -m "异步任务：迁移处理器并实现可靠确认"
```

---

### Task 7: 发布扫描、连接恢复与日志维护生命周期

**Files:**
- Create: `app/admin/internal/task/publisher.go`
- Create: `app/admin/internal/task/publisher_test.go`
- Create: `app/admin/internal/task/server.go`
- Create: `app/admin/internal/task/server_test.go`
- Modify: `internal/data/log_maintenance.go`
- Modify: `internal/data/log_maintenance_test.go`

**Interfaces:**
- Produces `Publisher.Dispatch(ctx context.Context, now time.Time) error`。
- Produces `NewServer(cfg conf.Config, resources *data.Data, providers *provider.AdminSet, logger log.Logger) *Server`，复用 Admin 已创建的 Storage Provider。
- `Server` 实现 Kratos `transport.Server` 的 `Start(context.Context) error` 与 `Stop(context.Context) error`。

- [ ] **Step 1: 写 publisher confirm 状态测试**

fake repository 返回三类 PendingTask，fake Broker 记录 Message。断言 Publish 成功才调用 MarkDispatched；Publish 失败时停止本批并保留待办；MarkDispatched 失败返回错误且允许后续重复投递。

- [ ] **Step 2: 实现 Publisher**

`Dispatch` 每批最多 100 条，使用 Task 4 构造函数生成固定消息；不并发共享同一个 AMQP channel。周期 loop 为 2 秒，错误只记录并等待下一轮。

- [ ] **Step 3: 写 Server 降级启动和重连测试**

fake Connector 前两次失败、第三次成功；在 goroutine 中调用 `Start`，断言 RabbitMQ 连接失败不会让 `Start` 返回，重连按可注入 timer 触发，连接成功后启动发布与三个 consumer；连接关闭后重新 Connect；调用 `Stop` 后 `Start` 返回 nil 且不再重连。另用阻塞 handler 验证 Stop 等待在途任务；Stop context 到期时关闭 Broker，使未 ACK delivery 可由 RabbitMQ 重投。

- [ ] **Step 4: 实现 supervisor 与退避**

重连间隔为 1s、2s、4s、8s、16s、最大 30s；连接成功后重置。每次重连重新声明拓扑并重启 publisher/consumer 子 context。Server 使用 WaitGroup 跟踪 publisher 与在途消费协程，Stop 先取消注册和扫描，再等待 WaitGroup 或 context deadline，最后关闭 Broker。RabbitMQ URL 日志只输出 host/vhost，不输出 userinfo。

- [ ] **Step 5: 为日志维护 MySQL 锁写失败测试并实现**

每小时执行前调用 `GET_LOCK('kratos_admin_log_maintenance', 0)`；未获锁跳过本轮，获锁后执行现有 Cleanup 并释放。复用 Task 2 的命名锁适配器，不复制 SQL 解析逻辑。

- [ ] **Step 6: 运行生命周期测试**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/task ./internal/data -count=1`

Expected: PASS 且无 goroutine 泄漏告警。

- [ ] **Step 7: 提交**

```bash
git add app/admin/internal/task internal/data/log_maintenance.go internal/data/log_maintenance_test.go
git commit -m "异步任务：接入发布消费与重连生命周期"
```

---

### Task 8: 接入 Admin Wire 并硬删除独立 Worker

**Files:**
- Modify: `app/admin/wire.go`
- Regenerate: `app/admin/wire_gen.go`
- Modify: `app/admin/internal/server/app.go`
- Modify: `app/admin/internal/server/app_test.go`
- Modify: `app/admin/cmd/kratos-admin/main.go`
- Modify: `app/admin/cmd/kratos-admin/root.go`
- Modify: `app/admin/cmd/kratos-admin/root_test.go`
- Modify: `internal/data/data.go`
- Modify: `internal/provider/factory.go`
- Modify: `internal/provider/factory_test.go`
- Modify: `internal/architecture/dependency_test.go`
- Delete: `app/worker/**`

**Interfaces:**
- `adminserver.NewApp` 接收 HTTP、gRPC 和 task Server。
- `Data` 只保留 DB、Query、Redis；删除 AsynqClient。
- Cobra 只保留 `server`、`init-admin`、`gorm-gen`。

- [ ] **Step 1: 修改命令和架构测试并确认 RED**

根命令测试期望子命令集合不包含 `worker`；架构测试新增全仓禁止 `github.com/hibiken/asynq` 与 `/app/worker` import 的断言。运行后应因旧代码存在失败。

- [ ] **Step 2: 将 task Server 加入 Admin App 和 Wire**

Wire ProviderSet 加入 task Service、Publisher、Connector、Server；`kratos.New` 同时注册 HTTP、gRPC 和 task Server。删除 `Data.AsynqClient` 的创建与关闭；删除 `WorkerSet`/`NewWorkerSet`，task Server 复用 `AdminSet.Storage`。

- [ ] **Step 3: 删除 Worker 入口与目录**

删除 `runWorker`、runner 字段、Cobra command、Worker logger/Wire/App/Server/Service。此次删除已由设计明确批准，不保留废弃命令。

- [ ] **Step 4: 重新生成 Wire 并整理依赖**

Run:

```bash
make wire
go mod tidy
```

Expected: 只生成 Admin Wire 图，`go.mod` 不再包含 Asynq。

- [ ] **Step 5: 运行命令、架构和 Admin 测试**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/... ./internal/architecture ./internal/data -count=1
rg -n "asynq|app/worker|runWorker|NewWorker" --glob '*.go' .
```

Expected: 测试 PASS；`rg` 无现行 Go 代码命中。

- [ ] **Step 6: 提交**

```bash
git add app internal go.mod go.sum
git commit -m "架构：收敛为单一Admin后台进程"
```

---

### Task 9: 收敛 Docker Compose、镜像和开发命令

**Files:**
- Modify: `deploy/docker-compose.yml`
- Modify: `deploy/Dockerfile.backend`
- Modify: `Makefile`
- Modify: `.env.example`

**Interfaces:**
- Compose 增加 `rabbitmq` 服务、`rabbitmq-data` volume、5672/15672 端口。
- Compose 删除 `migrate`、`worker` 服务。
- `compose-deps-up` 启动 mysql、redis、rabbitmq、mailpit。
- 保留宿主机 `make migrate`，删除 `run-worker` 和 Worker 构建目标。

- [ ] **Step 1: 先修改 Compose 验收断言**

使用 shell 检查脚本或 Make 目标断言最终 config 包含 rabbitmq 且不包含 migrate/worker service；在修改 Compose 前运行并确认失败。

- [ ] **Step 2: 更新 Compose**

使用 `rabbitmq:4.3.2-management`，设置独立用户、密码和 vhost，挂载 `/var/lib/rabbitmq`，健康检查使用 `rabbitmq-diagnostics -q ping`。API 只依赖 MySQL/Redis 健康，不依赖 RabbitMQ 健康。`init-admin` 改为等待 API healthy，Frontend 等待 init-admin 成功。

- [ ] **Step 3: 收敛 Dockerfile 与 Makefile**

删除 `tools` 和 `worker` stage；runtime 二进制已经内嵌 SQL。删除 `run-worker`；`compose-up/down/config` 语义保持不变。

- [ ] **Step 4: 验证 Compose 和镜像配置**

Run:

```bash
docker compose --env-file .env.example -f deploy/docker-compose.yml config --quiet
docker compose --env-file .env.example -f deploy/docker-compose.yml config --services
```

Expected services: `mysql redis rabbitmq mailpit api init-admin frontend`，无 `migrate`、`worker`。

- [ ] **Step 5: 提交**

```bash
git add deploy Makefile .env.example
git commit -m "部署：接入RabbitMQ并移除Worker服务"
```

---

### Task 10: MySQL 与 RabbitMQ 集成恢复验证

**Files:**
- Modify: `internal/data/migration_test.go`
- Create: `app/admin/internal/task/broker_integration_test.go`
- Create: `app/admin/internal/task/recovery_integration_test.go`

**Interfaces:**
- 使用 `KRATOS_ADMIN_TEST_MYSQL_DSN` 启用真实 MySQL 8 迁移锁测试。
- 使用 `KRATOS_ADMIN_TEST_RABBITMQ_URL` 启用真实 RabbitMQ publish/consume/ack 测试。
- 未配置环境变量时使用中文原因 `t.Skip`，保持普通 `go test ./...` 可运行。

- [ ] **Step 1: 写 MySQL 并发迁移集成测试**

两个 goroutine 同时调用 `data.Migrate`，使用 barrier 同时开始；断言两者均成功，随后用 Goose status API 断言版本 `00001` 至 `00006` 已应用。测试只操作专用测试库，不创建或删除生产数据库。

- [ ] **Step 2: 写 RabbitMQ Broker 集成测试**

连接测试 vhost，声明正式拓扑，向审计 routing key 发布唯一 message ID，消费后验证 body/type/message ID 并 manual ACK。另向不存在的 routing key mandatory publish，断言返回不可路由错误。

- [ ] **Step 3: 写 RabbitMQ 恢复集成测试**

使用 fake TaskRepository 加真实 Broker：首次发布确认后消费但不 ACK，主动关闭 consumer connection，等待 supervisor 重连后断言同一 message ID 再次送达；第二次 ACK 后断言不再投递。测试总超时 30 秒。

- [ ] **Step 4: 启动本地依赖并执行集成测试**

Run:

```bash
docker compose --env-file .env.example -f deploy/docker-compose.yml up -d mysql redis rabbitmq mailpit
KRATOS_ADMIN_TEST_MYSQL_DSN='kratos:kratos@tcp(127.0.0.1:3306)/kratos_admin?charset=utf8mb4&parseTime=True&loc=Local' KRATOS_ADMIN_TEST_RABBITMQ_URL='amqp://kratos:kratos@127.0.0.1:5672/kratos_admin' GOCACHE=/tmp/go-build go test ./internal/data ./app/admin/internal/task -run 'Migration|RabbitMQ|Recovery' -count=1 -timeout=60s
```

Expected: PASS；RabbitMQ management 中无未确认测试消息。

- [ ] **Step 5: 提交**

```bash
git add internal/data/migration_test.go app/admin/internal/task/broker_integration_test.go app/admin/internal/task/recovery_integration_test.go
git commit -m "测试：验证迁移锁与RabbitMQ故障恢复"
```

---

### Task 11: 文档、全链路验证与最终审查

**Files:**
- Modify: `README.md`
- Modify: `docs/deployment.md`
- Modify: `docs/acceptance.md`
- Modify: relevant current architecture docs under `docs/`

**Interfaces:**
- 文档统一描述单 Admin 进程、启动迁移、RabbitMQ 降级和管理端口。
- 明确 K8s 仅部署 Admin Deployment，不再部署 Worker 或 migrate Job。

- [ ] **Step 1: 更新用户与运维文档**

README 启动顺序改为 MySQL/Redis/RabbitMQ/Mailpit → Admin 自动迁移 → init-admin → Frontend。部署文档说明 RabbitMQ 不可用时 API 可用但异步积压、恢复后补投；升级文档说明所有 Admin Pod 使用 MySQL 锁串行检查迁移且 SQL 必须向前兼容。

- [ ] **Step 2: 更新验收命令**

移除独立 Goose/Worker 镜像验收；增加 RabbitMQ 停机/恢复、死信、队列堆积和多 Admin 迁移锁场景。保留 `make migrate` 作为人工运维命令。

- [ ] **Step 3: 运行格式化和定向验证**

Run:

```bash
gofmt -w migrations/embed.go internal/data/migration.go internal/data/migration_test.go internal/data/task_repository.go internal/data/task_repository_test.go app/admin/internal/task/*.go app/admin/cmd/kratos-admin/*.go internal/biz/logexport/*.go internal/provider/storage/local.go internal/conf/config.go internal/conf/config_test.go
GOCACHE=/tmp/go-build go test ./app/admin/... ./internal/conf ./internal/data/... ./internal/biz/logexport ./internal/provider/storage -count=1
```

Expected: PASS，无格式 diff。

- [ ] **Step 4: 运行后端回归验证**

Run:

```bash
GOCACHE=/tmp/go-build go test ./...
GOCACHE=/tmp/go-build go vet ./...
GOCACHE=/tmp/go-build make build
docker compose --env-file .env.example -f deploy/docker-compose.yml config --quiet
```

Expected: 全部退出码 0。

- [ ] **Step 5: 执行静态残留检查**

Run:

```bash
rg -n "Asynq|asynq|app/worker|kratos-admin worker|service_completed_successfully.*migrate" README.md docs app internal deploy Makefile go.mod
git diff --check
git status --short
```

Expected: 只有历史 `docs/superpowers/specs|plans` 可保留旧架构描述；现行代码和部署文档无旧入口。`git diff --check` 无输出，状态只包含本任务预期文档改动。

- [ ] **Step 6: 提交文档与最终修正**

```bash
git add README.md docs .env.example
git commit -m "文档：同步单进程与RabbitMQ部署方式"
```

- [ ] **Step 7: 最终审查**

核对设计中的每一项：MySQL 锁迁移、单进程、RabbitMQ 三队列、MySQL 重试、Broker 降级、多实例幂等、优雅停机、Compose/K8s 文档均有代码与测试证据。不要提交与本任务无关的文件。
