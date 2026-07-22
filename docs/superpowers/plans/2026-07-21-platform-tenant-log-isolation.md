# 平台与租户日志隔离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将平台日志与租户日志在菜单、路由与 API 查询上完全隔离，平台审计仅展示 `tenant_id = 0` 的记录，并修复平台管理员可越权查看全量租户日志的后端漏洞。

**Architecture:** 复用现有 `audit_logs` / `login_logs` / `api_access_logs` / `log_exports` 表，以 `tenant_id` 区分域。Goose 迁移拆分菜单资源（租户 `scope_mask=2`，平台新增 `platform-*` 菜单）；`scopedManagementRead` 对日志类资源按认证域强制过滤；前端增加 `/platform/logs/*` 路由与导航映射，页面复用 `ResourceListView`。

**Tech Stack:** Go 1.24、Kratos v2、GORM Gen、MySQL 8/Goose、Vue 3 `<script setup>`、TypeScript、Vitest。

## Global Constraints

- 平台操作审计范围：仅 `tenant_id = 0` 的平台域直接操作；代维操作记入租户日志且带 `impersonator_id`。
- `scope_mask` 取值：`1` 仅平台、`2` 仅租户；日志资源不再使用 `3`（平台与租户共用）。
- 平台特权判定：`realm = platform AND tenant_id = 0 AND impersonator_id = 0`（`bizauth.IsPlatformContext`）。
- 菜单 `code` 全局唯一；management API 资源名保持 `login-logs` / `audit-logs` / `api-logs` / `log-exports` 不变。
- 代码注释与 git 提交消息使用简体中文。
- 不新增第三方依赖；每次提交只暂存当前任务列出的文件。

---

### Task 1: 日志域查询隔离（后端）

**Files:**
- Modify: `app/admin/internal/data/management_gen_read.go`
- Modify: `app/admin/internal/data/management_repository_test.go`（或新建 `management_log_scope_test.go`）
- Test: `app/admin/internal/data/management_*_test.go`

**Interfaces:**
- Produces: `applyLogDomainScope(scope managementbiz.Scope, resource string, read managementReadQuery) gen.Dao`（或内联于 `scopedManagementRead`）
- Consumes: `managementbiz.Scope{TenantID, PlatformAdmin, Impersonating}`

- [ ] **Step 1: 写失败测试**

在 `app/admin/internal/data/management_repository_test.go` 增加集成测试（MySQL）：

```go
func TestListAuditLogsPlatformContextOnlyTenantZero(t *testing.T) {
	// 插入 tenant_id=0 与 tenant_id=8 各一条 audit_logs
	// 使用 scope: PlatformAdmin=true, TenantID=0, Impersonating=false
	// 断言 List 只返回 tenant_id=0 的记录
}

func TestListAuditLogsTenantContextOnlyOwnTenant(t *testing.T) {
	// 插入 tenant_id=8 与 tenant_id=9 各一条
	// 使用 scope: TenantID=8, MemberID>0
	// 断言只返回 tenant_id=8
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd app/admin && GOCACHE=/tmp/go-build go test ./internal/data -run 'TestListAuditLogs' -count=1
```

Expected: FAIL（平台上下文返回全量或未过滤）。

- [ ] **Step 3: 实现域过滤**

在 `scopedManagementRead` 中，对 `audit-logs`、`login-logs`、`api-logs`、`log-exports`：

```go
func isLogResource(resource string) bool {
	switch resource {
	case "audit-logs", "login-logs", "api-logs", "log-exports":
		return true
	default:
		return false
	}
}

func isPlatformLogScope(scope managementbiz.Scope) bool {
	return scope.PlatformAdmin && scope.TenantID == 0 && !scope.Impersonating
}

// 在 tenantScoped 分支之前或替代其日志逻辑：
if isLogResource(resource) {
	if isPlatformLogScope(scope) {
		read.dao = read.dao.Where(read.field("tenant_id").Eq(managementSQLValue{uint64(0)}))
	} else {
		read.dao = read.dao.Where(read.field("tenant_id").Eq(managementSQLValue{scope.TenantID}))
	}
}
```

在 `applyDataScopeGen` 开头：

```go
if isLogResource(resource) && isPlatformLogScope(scope) {
	return dao, nil // 平台域跳过租户数据权限
}
```

确保平台 `tenant_id=0` 时不再走原 `tenantScoped` 的「不过滤」分支（删除或调整第 102 行条件，避免日志资源漏过滤）。

- [ ] **Step 4: 运行测试确认 GREEN**

```bash
cd app/admin && GOCACHE=/tmp/go-build go test ./internal/data -run 'TestListAuditLogs' -count=1
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add app/admin/internal/data/management_gen_read.go app/admin/internal/data/management_repository_test.go
git commit -m "fix: 按认证域隔离平台与租户日志查询"
```

---

### Task 2: 代维审计写入 impersonator_id

**Files:**
- Modify: `app/admin/internal/biz/management/contracts.go`（若 Scope 需增加 ImpersonatorID 则在此）
- Modify: `app/admin/internal/service/management.go`（`managementScope` 填充 ImpersonatorID）
- Modify: `app/admin/internal/data/management_repository.go`（`writeAuditOutboxGen`）
- Modify: `app/admin/internal/biz/audit/processor.go`（`Entry` 增加 `ImpersonatorID`）
- Modify: `app/admin/internal/data/audit_repository.go`（`Publish` 写入字段）
- Test: `app/admin/internal/biz/audit/processor_test.go`
- Test: `app/admin/internal/data/audit_repository_test.go`（若无则新建）

**Interfaces:**
- Produces: Outbox payload 字段 `impersonator_id uint64`
- Produces: `audit.Entry.ImpersonatorID`
- Produces: `model.AuditLog.ImpersonatorID` 落库

- [ ] **Step 1: 写失败测试**

```go
func TestWriteAuditOutboxIncludesImpersonatorID(t *testing.T) {
	scope := managementbiz.Scope{TenantID: 8, UserID: 5, MemberID: 9, Impersonating: true /* + ImpersonatorID: 1 */}
	// 调用 writeAuditOutboxGen 后断言 outbox payload 含 impersonator_id: 1
}
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd app/admin && GOCACHE=/tmp/go-build go test ./internal/biz/audit ./internal/data -run 'Impersonator' -count=1
```

- [ ] **Step 3: 实现**

`managementScope` 从 `claims.ImpersonatorID` 写入 `Scope`（新增 `ImpersonatorID uint64` 字段）。

`writeAuditOutboxGen` payload 增加：

```go
"impersonator_id": scope.ImpersonatorID,
```

`audit.Entry` 与 `Publish` 映射 `ImpersonatorID` 到 `model.AuditLog.ImpersonatorID`。

- [ ] **Step 4: 运行测试确认 GREEN**

- [ ] **Step 5: 提交**

```bash
git commit -m "feat: 代维操作审计记录 impersonator_id"
```

---

### Task 3: 菜单资源迁移

**Files:**
- Create: `migrations/00011_platform_tenant_log_isolation.sql`
- Test: `app/admin/internal/data/management_repository_test.go` 或 `schema_test.go`（断言资源 scope 与 code）

**Interfaces:**
- Produces: 租户日志 `scope_mask=2`；平台日志菜单 `scope_mask=1`

- [ ] **Step 1: 写迁移**

```sql
-- +goose Up
-- 租户日志改为仅租户
UPDATE resources SET scope_mask = 2
WHERE code IN ('menu.logs', 'login-logs', 'audit-logs', 'api-logs', 'log-exports');

-- 平台日志目录与菜单
INSERT INTO resources (parent_id, type, scope_mask, code, name, route_path, component_key, icon, sort_order, visible, status)
SELECT id, 1, 1, 'menu.platform-logs', '平台日志', '', '', 'Document', 17, 1, 1
FROM resources WHERE code = 'menu.platform'
AND NOT EXISTS (SELECT 1 FROM resources WHERE code = 'menu.platform-logs');

INSERT INTO resources (parent_id, type, scope_mask, code, name, route_path, component_key, icon, sort_order, visible, status)
SELECT p.id, 2, 1, 'platform-login-logs', '登录日志', '/platform/logs/login', 'platform-login-logs', 'List', 171, 1, 1
FROM resources p WHERE p.code = 'menu.platform-logs'
AND NOT EXISTS (SELECT 1 FROM resources WHERE code = 'platform-login-logs');

-- 同理 platform-audit-logs (172), platform-api-logs (173), platform-log-exports (174)

-- +goose Down
DELETE FROM resources WHERE code IN (
  'platform-login-logs', 'platform-audit-logs', 'platform-api-logs', 'platform-log-exports', 'menu.platform-logs'
);
UPDATE resources SET scope_mask = 3
WHERE code IN ('menu.logs', 'login-logs', 'audit-logs', 'api-logs', 'log-exports');
```

- [ ] **Step 2: 本地执行迁移**

```bash
goose -dir migrations mysql "$DATABASE_DSN" up
```

- [ ] **Step 3: 断言资源数据**

```bash
cd app/admin && GOCACHE=/tmp/go-build go test ./internal/data -run 'TestLogResourceScopeMask' -count=1
```

（测试中查询 `platform-audit-logs` scope_mask=1，`audit-logs` scope_mask=2）

- [ ] **Step 4: 提交**

```bash
git add migrations/00010_platform_tenant_log_isolation.sql app/admin/internal/data/*_test.go
git commit -m "feat: 拆分平台与租户日志菜单资源"
```

---

### Task 4: 前端路由与导航

**Files:**
- Modify: `app/frontend/src/router/index.ts`
- Modify: `app/frontend/src/features/navigation/registry.ts`
- Modify: `app/frontend/src/features/navigation/registry.test.ts`
- Modify: `app/frontend/src/layouts/PlatformShell.test.ts`（如有导航断言）

**Interfaces:**
- Consumes: 后端导航返回 `platform-*` 的 `routePath` / `componentKey`
- Produces: 平台顶栏可见 `/platform/logs/*`

- [ ] **Step 1: 写失败测试**

`registry.test.ts` 增加：

```typescript
it('maps platform log menus to /platform/logs paths', () => {
  const items = resolveNavigation([
    { name: '操作审计', routePath: '/platform/logs/audit', componentKey: 'platform-audit-logs', sortOrder: 172 }
  ])
  expect(items[0].to).toBe('/platform/logs/audit')
})
```

- [ ] **Step 2: 运行测试确认 RED**

```bash
cd app/frontend && npm test -- registry.test.ts
```

- [ ] **Step 3: 实现**

`router/index.ts` — `platformManagementRoutes` 增加：

```typescript
['logs/login', 'platform-login-logs', 'login-logs'],
['logs/audit', 'platform-audit-logs', 'audit-logs'],
['logs/api', 'platform-api-logs', 'api-logs'],
['logs/exports', 'platform-log-exports', 'log-exports'],
```

`registry.ts` — `componentRouteRegistry` 增加：

```typescript
'platform-login-logs': '/platform/logs/login',
'platform-audit-logs': '/platform/logs/audit',
'platform-api-logs': '/platform/logs/api',
'platform-log-exports': '/platform/logs/exports',
```

`resolveTopNavigation` 平台分支增加「平台日志」分组（`prefixes: ['/platform/logs/']`），与「平台治理」并列。

- [ ] **Step 4: 运行测试确认 GREEN**

```bash
cd app/frontend && npm test -- registry.test.ts PlatformShell.test.ts
```

- [ ] **Step 5: 提交**

```bash
git commit -m "feat: 增加平台日志路由与导航映射"
```

---

### Task 5: 端到端验证与文档同步

**Files:**
- Modify: `app/frontend/e2e/admin.spec.ts`（可选：平台日志列表 smoke）
- Verify: `docs/superpowers/specs/2026-07-21-platform-tenant-log-isolation-design.md`

- [ ] **Step 1: 运行全量相关测试**

```bash
cd app/admin && GOCACHE=/tmp/go-build go test ./...
cd app/frontend && npm test
```

- [ ] **Step 2: 手动冒烟**

1. 平台管理员登录 → 顶栏出现「平台日志」→ 进入操作审计 → 仅 `tenant_id=0` 数据
2. 租户管理员登录 → 仅有 `/console/logs/*`，无平台日志菜单
3. 平台代维进入租户 → 操作后租户审计可见且含代维信息

- [ ] **Step 3: 提交（若有 e2e 改动）**

```bash
git commit -m "test: 补充平台日志导航与隔离用例"
```

---

## Spec Coverage Checklist

| 需求 | Task |
|------|------|
| 平台查询 `tenant_id=0` | Task 1 |
| 租户查询当前租户 | Task 1 |
| 平台跳过数据权限 | Task 1 |
| 代维 impersonator_id | Task 2 |
| 租户菜单 scope_mask=2 | Task 3 |
| 平台菜单 `/platform/logs/*` | Task 3, 4 |
| 前端导航分组 | Task 4 |
| 验收与测试 | Task 5 |

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-21-platform-tenant-log-isolation.md`.

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每个 Task 派发独立子代理，任务间做审查，迭代快
2. **Inline Execution** — 本会话按 Task 顺序直接实现，阶段性汇报

请选择一种方式，或告知我直接开始实现。
