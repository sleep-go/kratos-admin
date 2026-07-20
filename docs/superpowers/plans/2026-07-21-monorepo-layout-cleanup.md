# Kratos Admin 前端与 Compose 目录收口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Vue 前端迁移到 `app/frontend`，将完整 Compose 编排迁移到根目录，并让构建、测试和文档只使用新路径。

**Architecture:** 根目录继续作为 Go module、Docker build context 和运维入口；Admin、Worker、Frontend 都归入 `app/*`。根级 `docker-compose.yml` 保留全部服务，通过 `deploy/Dockerfile.backend` 和 `app/frontend/Dockerfile` 构建应用，不保留旧路径兼容副本。

**Tech Stack:** Go 1.26、go-kratos、Vue 3、Vite、TypeScript、pnpm、Docker Compose、Make。

## Global Constraints

- 不改变 Vue 组件、前端路由、API DTO 或后端业务行为。
- 不保留根级 `frontend` 或 `deploy/docker-compose.yml` 的副本和软链接。
- Compose 保留 MySQL、Redis、Mailpit、migrate、init-admin、api、worker、frontend 全部服务。
- Git Commit Message 使用中文，最终合并到 `main` 并推送 `origin/main`。

---

### Task 1: 建立目录结构契约并迁移文件

**Files:**
- Modify: `internal/architecture/dependency_test.go`
- Move: `frontend` → `app/frontend`
- Move: `deploy/docker-compose.yml` → `docker-compose.yml`

**Interfaces:**
- Produces: `app/frontend/package.json` 与根级 `docker-compose.yml` 两个唯一入口。

- [ ] **Step 1: 写入失败的目录契约测试**

在 `internal/architecture/dependency_test.go` 增加 `TestMonorepoEntrypoints`，断言新入口存在、旧入口不存在。

- [ ] **Step 2: 运行测试并确认失败**

```bash
GOCACHE=/tmp/go-build go test ./internal/architecture -run TestMonorepoEntrypoints -count=1
```

Expected: FAIL，提示 `app/frontend/package.json` 或根级 `docker-compose.yml` 不存在。

- [ ] **Step 3: 物理迁移两个入口**

```bash
mv frontend app/frontend
mv deploy/docker-compose.yml docker-compose.yml
```

- [ ] **Step 4: 运行目录契约测试**

```bash
GOCACHE=/tmp/go-build go test ./internal/architecture -run TestMonorepoEntrypoints -count=1
```

Expected: PASS。

- [ ] **Step 5: 提交目录迁移**

```bash
git add app/frontend frontend docker-compose.yml deploy/docker-compose.yml internal/architecture/dependency_test.go
git commit -m "结构：迁移前端与Compose到大仓入口"
```

### Task 2: 修正构建、测试和 Compose 路径

**Files:**
- Modify: `Makefile`
- Modify: `docker-compose.yml`
- Modify: `app/frontend/Dockerfile`
- Modify: `app/frontend/playwright.config.ts`
- Modify: `.dockerignore`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: 根目录 build context `.`。
- Produces: `make frontend-*` 与 `make compose-*` 新路径命令。

- [ ] **Step 1: 搜索所有有效旧路径引用**

```bash
rg -n 'frontend/|deploy/docker-compose.yml' Makefile docker-compose.yml app/frontend .dockerignore .gitignore docs README.md
```

- [ ] **Step 2: 修改 Makefile 和忽略规则**

前端目标统一执行 `cd app/frontend`；Compose 目标统一执行 `docker compose --env-file .env`。忽略规则改为 `app/frontend/node_modules`、`dist`、coverage、Playwright 结果和 TypeScript build info。

- [ ] **Step 3: 修改 Compose 与 Dockerfile**

根级 Compose 的 build context 使用 `.`；后端 Dockerfile 为 `deploy/Dockerfile.backend`，前端 Dockerfile 为 `app/frontend/Dockerfile`。前端 Dockerfile 从根 context 复制 `app/frontend/*`。

- [ ] **Step 4: 验证工程入口**

```bash
make help
make build
make compose-config
cd app/frontend && pnpm lint && pnpm typecheck && pnpm test:run && pnpm build
```

Expected: 全部退出码 0。

- [ ] **Step 5: 提交构建路径调整**

```bash
git add Makefile docker-compose.yml app/frontend .dockerignore .gitignore
git commit -m "构建：统一大仓应用与Compose路径"
```

### Task 3: 重写 README、同步文档并完成验收

**Files:**
- Modify: `README.md`
- Modify: `docs/deployment.md`
- Modify: `docs/acceptance.md`
- Modify: current specs under `docs/superpowers/specs`

**Interfaces:**
- Produces: 可直接复制执行的全量 Compose 与本地混合启动说明。

- [ ] **Step 1: 重写 README**

按“能力、技术栈、目录、环境要求、全量 Compose、仅依赖 Compose、本地启动、迁移初始化、配置、常用命令、排障”组织，并使用 `app/frontend` 与根级 Compose 路径。

- [ ] **Step 2: 同步部署和验收文档**

移除现行文档中的 `frontend` 与 `deploy/docker-compose.yml` 操作路径；历史计划只保留历史说明。

- [ ] **Step 3: 完整生成和测试**

```bash
make all
make build
make test
make vet
cd app/frontend && pnpm lint && pnpm typecheck && pnpm test:run && pnpm build
docker compose --env-file .env.example config --quiet
docker compose --env-file .env.example build migrate init-admin api worker frontend
git diff --check
```

Expected: 全部退出码 0；仅允许 Buf API body 和 Vite chunk size 既有警告。

- [ ] **Step 4: 提交文档并交付**

```bash
git add README.md docs
git commit -m "文档：重写大仓启动与部署说明"
git switch main
git merge --ff-only codex/monorepo-layout-cleanup
git push origin main
```
