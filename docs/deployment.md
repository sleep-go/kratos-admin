# 部署与运维

## 首次启动

1. 复制配置：`cp .env.example .env`。
2. 至少修改 `KRATOS_ADMIN_SECRET_KEY`（精确 32 字节）和 `KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD`（不少于 12 位）。生产环境还必须配置持久化 Ed25519 私钥 `KRATOS_ADMIN_JWT_PRIVATE_KEY`。
3. 执行 `make compose-config` 检查 Compose 配置，再执行 `make compose-up`。
4. 访问 `http://127.0.0.1:8080`；API 健康检查为 `http://127.0.0.1:8000/api/v1/health`，Mailpit 为 `http://127.0.0.1:8025`。

启动顺序固定为 MySQL 健康检查、Goose 迁移、幂等超级管理员初始化、API/Worker、Frontend。API 和 Worker 不执行 `AutoMigrate`。

## PolarDB 与 OSS

- 将 `KRATOS_ADMIN_MYSQL_DSN` 改为 PolarDB MySQL 8 内网地址；账号至少需要目标库 DDL/DML 权限。先在维护窗口独立运行 `migrate` 服务，再滚动启动 API 和 Worker。
- 将存储 Provider 改为 `aliyun-oss` 并配置 Region、Endpoint、Bucket 和凭据。Bucket 保持私有，浏览器只使用短期直传凭证与短期下载签名。
- 生产环境不得使用示例密钥、默认数据库密码或临时生成的 JWT 私钥。

## 备份与恢复

升级前同时备份 MySQL、Redis AOF 和对象存储清单。MySQL 建议使用 PolarDB 备份或 `mysqldump --single-transaction --routines --triggers`；恢复时先恢复数据库，再恢复对象文件，最后启动 Redis、API 与 Worker。Redis 只承载缓存、验证码和队列，恢复失败时可重建，但尚未消费的异步任务需要从业务 outbox 补投。

文件元数据与对象必须成对核对。不要只恢复数据库后直接清理“孤儿对象”，应先运行只读清单比对。

## 升级与回滚

1. 备份并记录当前镜像标签、Goose 版本和环境变量摘要。
2. 使用新镜像单独执行 `migrate`，确认完成后再更新 API、Worker 和 Frontend。
3. 检查健康接口、登录、租户切换、权限、日志和异步任务。
4. 应用回滚不自动回退数据库。只有迁移文件明确提供安全的 `Down` 且已验证数据兼容性时，才允许执行 Goose 回退。

## 故障排查

- API 启动失败：检查 DSN、MySQL 字符集、Redis 地址、32 字节配置密钥及生产 JWT 私钥。
- 登录后立即失效：确认所有 API 实例使用同一 Ed25519 私钥，租户 `permission_version` 未被异常重复递增。
- 菜单缺失：确认平台已授权租户功能，角色已保存资源动作，并重新刷新令牌。
- 日志导出停在等待：检查 Worker、Redis/Asynq 和 `async_tasks`；任务具有幂等键，可安全重试。
- 文件停在等待清理：检查 Worker、Provider 配置和对象权限；失败原因保存在文件状态中，不要直接删除元数据。
- Goose 迁移失败：停止新版本 API/Worker，保留失败现场并检查 `goose_db_version`，不要手工伪造成功版本。

## 验收命令

```bash
make api
GOCACHE=/tmp/go-build go test -race ./backend/...
GOCACHE=/tmp/go-build go vet ./backend/...
cd frontend && pnpm lint && pnpm typecheck && pnpm test:run && pnpm build
cd frontend && E2E_ADMIN_PASSWORD='你的初始化密码' E2E_REDIS_PORT=6379 pnpm e2e
make compose-config
docker compose --env-file .env -f deploy/docker-compose.yml build
```
