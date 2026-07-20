# Kratos Admin 大仓前端与 Compose 目录收口设计

## 背景与目标

当前后端应用已位于 `app/admin` 与 `app/worker`，但 Vue 前端仍位于根级 `frontend`，完整 Docker Compose 文件仍位于 `deploy/docker-compose.yml`。这使大仓应用目录、构建上下文和文档命令存在两套路径习惯。

本次调整将前端作为大仓中的独立应用迁移到 `app/frontend`，并将完整 Compose 编排迁移到根目录 `docker-compose.yml`。迁移只调整工程路径、构建入口和文档，不改变前端业务、后端 API、数据库结构或运行配置语义。

## 目标目录

```text
app/
├── admin/
│   ├── cmd/kratos-admin/
│   └── internal/{server,service}/
├── worker/
│   └── internal/{server,service}/
└── frontend/
    ├── src/
    ├── e2e/
    ├── Dockerfile
    ├── nginx.conf
    ├── package.json
    └── pnpm-lock.yaml
configs/
deploy/
└── Dockerfile.backend
docker-compose.yml
Makefile
README.md
```

迁移完成后不保留根级 `frontend` 或 `deploy/docker-compose.yml` 的副本、软链接及兼容入口，避免形成双真相源。

## Docker Compose 与构建上下文

根目录 `docker-compose.yml` 保留现有全部服务：MySQL、Redis、Mailpit、Goose migration、超级管理员初始化、Admin API、Worker 与 Frontend。

Compose 文件位于仓库根目录后：

- 所有服务的构建上下文统一为 `.`。
- 后端 Dockerfile 使用 `deploy/Dockerfile.backend`。
- 前端 Dockerfile 使用 `app/frontend/Dockerfile`。
- `env_file` 使用根级 `.env`。
- Goose 迁移、健康检查、依赖顺序、端口、数据卷和服务名称保持不变。

前端 Dockerfile 从根构建上下文复制 `app/frontend/package.json`、锁文件、源码和 Nginx 配置。后端 Dockerfile 路径及内容保持不变，除非验证发现根级 Compose 引用必须同步。

## Makefile 与开发命令

Makefile 的前端目标全部进入 `app/frontend` 执行，包括依赖安装、测试、构建和 E2E。Compose 目标直接使用根目录默认文件：

```bash
docker compose --env-file .env config --quiet
docker compose --env-file .env up -d mysql redis mailpit
docker compose --env-file .env up --build
docker compose --env-file .env down
```

保留现有 `compose-config`、`compose-deps-up`、`compose-up`、`compose-down` 目标名称，不增加旧路径兼容目标。

## README 与文档

README 重写为可直接执行的项目入口，依次说明：

1. 项目能力与技术栈。
2. 大仓目录与单一 `kratos-admin` 命令入口。
3. 本地环境要求和首次配置。
4. Compose 完整启动。
5. Compose 仅启动依赖、前后端本地运行。
6. Goose 迁移、管理员初始化及常用 Make 目标。
7. YAML/环境变量配置边界、默认地址和排障入口。

`docs/deployment.md`、`docs/acceptance.md`、`.dockerignore`、`.gitignore`、Playwright 配置及仓库内有效脚本同步为新路径。历史实施计划可保留旧路径作为实施记录，但现行设计与操作文档不得继续引用旧路径。

## 验收标准

- 根目录不存在 `frontend`，前端完整位于 `app/frontend`。
- `deploy/docker-compose.yml` 不存在，根级 `docker-compose.yml` 可成功展开。
- `make all` 重复生成无差异，`make build`、`make test`、`make vet` 通过。
- 在 `app/frontend` 执行 lint、typecheck、组件测试和生产构建通过。
- Compose 的 migrate、init-admin、api、worker、frontend 镜像全部构建通过。
- README 中的完整启动与“只启动依赖”流程使用新路径且可直接复制执行。
- 变更合并到 `main` 并推送 `origin/main`。
