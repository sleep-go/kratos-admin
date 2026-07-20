# 大仓结构验收

## 目录与依赖

- Admin 应用位于 `app/admin`，Worker 应用位于 `app/worker`，Vue 前端位于 `app/frontend`。
- 共享领域、配置、仓储和 Provider 位于根级 `internal`。
- Admin 与 Worker 配置位于 `configs`，配置契约位于 `internal/conf/conf.proto`。
- Goose 迁移位于 `migrations`，服务与 Worker 不执行 `AutoMigrate`。
- `internal/data` 不导入应用 Service/Server，`internal/biz` 不导入 Data 或应用内部包。
- 仓库不存在实际 `backend/` 目录或运行命令兼容入口。

统一可执行程序及其子命令为：

```text
app/admin/cmd/kratos-admin
├── server
├── worker
├── init-admin
└── gorm-gen
```

## 后端验收

```bash
make wire
make gorm-gen
make test
make vet
make build
GOCACHE=/tmp/go-build go test ./app/admin/... ./app/worker/... ./internal/...
GOCACHE=/tmp/go-build go vet ./app/admin/... ./app/worker/... ./internal/...
```

## 生成一致性

依次执行 `make config`、`make wire`、`make gorm-gen`、`make api` 和 `cd app/frontend && pnpm api:generate`，随后确认对应生成目录没有 Git 差异。

## 数据库与部署

在全新 MySQL 8 测试库执行：

```bash
goose -dir migrations mysql "$KRATOS_ADMIN_TEST_MYSQL_DSN" up
goose -dir migrations mysql "$KRATOS_ADMIN_TEST_MYSQL_DSN" status
```

迁移版本 `00001` 至 `00005` 应全部应用，再次执行 `up` 应无待执行迁移。部署配置检查和镜像构建命令为：

```bash
make compose-config
docker compose --env-file .env build migrate init-admin api worker frontend
```

## 前端验收

```bash
cd app/frontend
pnpm lint
pnpm typecheck
pnpm test:run
pnpm build
pnpm e2e
```
