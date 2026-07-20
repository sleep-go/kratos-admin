# Kratos Layout 官方规范对齐设计

## 背景与结论

当前仓库已经具备 Kratos 的 API Proto、Wire 和 `biz/data/service/server` 分层，但运行配置仍由手写 `internal/conf/config.go` 直接读取环境变量，缺少官方 `kratos-layout` 的 `configs/*.yaml`、`internal/conf/conf.proto`、配置生成目标和 `--conf` 启动语义；根级 Makefile 也只有项目定制目标。

本次改造以 `go-kratos/kratos-layout` v2 习惯为基准，不升级现有 `github.com/go-kratos/kratos/v2 v2.9.2`，避免把目录规范对齐扩大为 Kratos v3 迁移。用户已确认的多应用大仓、Cobra、Wire、Goose、GORM Gen 和前后端部署边界保持不变。

## 目标目录

```text
api/admin/v1/*.proto
app/admin/cmd/kratos-admin
app/admin/internal/{service,server}
app/worker/internal/{service,server}
configs/{admin.yaml,worker.yaml}
internal/conf/{conf.proto,conf.pb.go,config.go}
internal/{biz,data,provider}
buf.gen.config.yaml
Makefile
```

`api/admin/v1` 继续作为业务接口真相源；`internal/conf/conf.proto` 只描述进程 Bootstrap 配置，两者不混用。

## 配置模型与加载顺序

`internal/conf/conf.proto` 定义 `Bootstrap`，包含 `environment`、`server`、`data`、`auth`、`storage` 和 `messaging`。HTTP、gRPC、数据库、Redis、认证、对象存储、SMTP 与阿里云短信均有显式字段；时间值使用 `google.protobuf.Duration`。

`conf.Load(path)` 使用 Kratos `config`、`config/file` 和 `config/env`：

1. 从 `--conf` 指定的 YAML 文件加载非敏感默认配置。
2. YAML 通过 `${KRATOS_ADMIN_*:default}` 引用环境变量，使现有 Docker 和生产环境变量继续覆盖。
3. Scan 到生成的 `Bootstrap`，再转换为现有运行时 `conf.Config`，把目录对齐与业务构造器变更解耦。
4. 生产环境继续要求 Secret Key 与 JWT 私钥；配置文件不存在、字段类型错误或关键配置为空时启动失败并返回中文上下文错误。

后续按用户确认收敛为单一 `kratos-admin` Cobra 入口，提供 `server`、`worker`、`init-admin` 与 `gorm-gen` 子命令。Server 默认 `--conf ./configs/admin.yaml`，Worker 默认 `--conf ./configs/worker.yaml`，init-admin 默认复用 Admin 配置。

## Makefile

Makefile 以官方目标为主体：

- `init`：安装 Wire、Buf 和项目所需生成器。
- `config`：使用 `buf.gen.config.yaml` 生成配置 Proto。
- `api`：lint 并生成业务 API/OpenAPI。
- `generate`：执行 Go Generate、Wire、GORM Gen，并检查模块依赖。
- `build`：输出单一 `bin/kratos-admin` 命令。
- `all`：依次执行 API、配置和代码生成。
- `help`：作为默认目标展示注释生成的帮助。

现有 `wire`、`gorm-gen`、`migrate`、`test`、前端和 Compose 目标作为项目扩展保留；旧的 `backend-*` 目标保留为兼容别名，文档改用标准目标。

## 部署与兼容性

Docker 镜像复制 `configs/` 并通过 `kratos-admin server|worker|init-admin` 及对应 `--conf` 启动。Compose 继续通过 `.env` 提供敏感值和容器地址，YAML 不保存真实密钥。原有 `KRATOS_ADMIN_*` 环境变量仍可用。

数据库表、Goose SQL、GORM Gen 查询、业务 API、前端 DTO、RBAC 和认证规则不变。

## 验收

- `make config` 生成 `internal/conf/conf.pb.go`，重复生成无差异。
- 配置测试覆盖 YAML 加载、环境变量覆盖、缺失文件和生产密钥校验。
- 根命令测试覆盖四个子命令，以及 `--conf` 和工具参数。
- `make all`、`make build`、`make test`、前端测试和 Docker 镜像构建通过。
- `go run ./app/admin/cmd/kratos-admin server --conf ./configs/admin.yaml` 与 Worker 子命令可启动。
