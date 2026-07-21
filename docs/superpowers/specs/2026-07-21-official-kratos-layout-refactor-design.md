# Kratos Admin 官方 Layout 对齐设计

## 背景

当前仓库已经采用大仓模式，但 Admin 的运行入口、Wire 注入器和业务分层分别散落在 `app/admin` 与根级 `internal`。同时，后台服务与数据库迁移、管理员初始化、GORM Gen 共用一个 Cobra 可执行程序。这与 go-kratos `kratos-layout` v2.9.2 的 `cmd/server + internal/{biz,data,service,server}` 结构和标准 `main.go` 启动方式不一致。

本次重构只调整目录、包路径、启动入口和构建部署命令，不改变 HTTP/gRPC API、数据库结构、RabbitMQ 后台任务行为以及 YAML 配置字段。

## 目标目录

```text
api/admin/v1/                         # Protobuf 契约和生成代码
app/admin/cmd/server/                 # 标准 Kratos 服务入口和 Wire 注入器
app/admin/cmd/tools/                  # 独立 Cobra 运维工具入口
app/admin/internal/biz/               # 实体、用例、仓储接口和业务规则
app/admin/internal/conf/              # conf.proto、生成代码和配置适配
app/admin/internal/data/              # 仓储实现、GORM 模型、查询和迁移
app/admin/internal/data/provider/     # 邮件、短信、存储和密钥等外部适配
app/admin/internal/server/            # HTTP、gRPC 和 Kratos App 组装
app/admin/internal/server/task/       # 实现 Kratos Server 生命周期的后台任务
app/admin/internal/service/           # Protobuf 服务实现
app/frontend/                         # Vue 前端应用
configs/                              # YAML 运行配置
migrations/                           # Goose SQL 与嵌入入口
third_party/                          # Protobuf 第三方契约
```

根级 `internal` 在迁移完成后删除。`provider` 属于基础设施实现，归入 `data/provider`；RabbitMQ 消费及定时任务实现 Kratos `transport.Server` 生命周期，归入 `server/task`。

## Admin 服务入口

`app/admin/cmd/server/main.go` 按 Kratos v2.9.2 官方模板实现：

- 使用标准库 `flag` 提供 `-conf`，不依赖 Cobra。
- 定义可由 `-ldflags` 注入的 `Name` 和 `Version`，实例 ID 默认使用主机名。
- 使用 Kratos `config.New` 和 file source 加载 YAML，并扫描为 `conf.Bootstrap`。
- 创建带 service/trace 字段的 Kratos Logger。
- 通过同目录 `wireApp` 构造 `*kratos.App`，最后调用 `app.Run()`。
- 保留当前启动前 Goose 迁移语义以及 HTTP、gRPC、RabbitMQ 后台任务共同生命周期。

`app/admin` 根目录不再保留 `app.go`、`logger.go`、`wire.go` 和 `wire_gen.go`。

## Tools 入口

`app/admin/cmd/tools` 构建为独立 `kratos-admin-tools` 可执行程序，使用 Cobra 提供：

- `migrate -conf <yaml>`：执行 Goose 迁移。
- `init-admin -conf <yaml>`：幂等初始化平台超级管理员。
- `gorm-gen`：生成 `app/admin/internal/data/query`。

Tools 位于 `app/admin` 的目录边界内，因此可以合法导入 `app/admin/internal`，无需暴露仅供命令使用的公共包。服务入口不包含任何 Cobra 命令。

## 构建与部署

- `make build` 同时生成 `bin/kratos-admin` 和 `bin/kratos-admin-tools`。
- `make wire` 生成 `app/admin/cmd/server/wire_gen.go`。
- `make run-admin` 使用 `go run ./app/admin/cmd/server -conf ./configs/config.yaml`。
- `make migrate`、`make init-admin`、`make gorm-gen` 使用 `go run ./app/admin/cmd/tools ...`。
- 后端 Dockerfile 分别构建服务和工具二进制；Compose API 使用服务二进制，初始化服务使用工具二进制。
- README、部署、验收和仍被引用的设计文档更新为新路径，历史计划保留历史说明而不伪造已实施状态。

## 验收标准

- 仓库不存在根级 `internal` 和旧 `app/admin/cmd/kratos-admin`。
- Admin `main.go` 的启动结构与官方 Kratos v2.9.2 模板一致，且不导入 Cobra。
- `app/admin/cmd/tools` 只负责运维命令。
- `go test ./app/admin/...`、`go vet ./app/admin/...`、两个可执行程序构建、生成一致性检查和 `docker compose config --quiet` 通过。
- 前端契约和运行行为不因本次目录调整发生变化。
