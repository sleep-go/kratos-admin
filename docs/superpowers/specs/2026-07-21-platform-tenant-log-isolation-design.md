# 平台与租户日志隔离设计

## 背景

系统已在数据层用 `tenant_id = 0` 表示平台域（`audit_logs`、`login_logs`、`api_access_logs`），但产品层仍将日志菜单全部挂在租户控制台 `/console/logs/*`，且资源 `scope_mask = 3`（平台与租户共用）。

平台顶栏只展示 `/platform/*` 路由，导致平台管理员无法访问日志。更关键的是，平台管理员查询日志时后端未强制 `tenant_id = 0` 过滤，存在越权查看所有租户日志的风险。

平台操作（租户管理、全局用户、资源授权等）与租户操作（成员、部门、角色等）职责不同，应使用独立入口与严格的数据边界。

## 目标

- 平台域与租户域日志在菜单、路由、查询上完全隔离。
- 平台操作审计仅记录平台域直接操作（`tenant_id = 0`）。
- 代维期间的操作记入租户日志，带 `impersonator_id`，不出现在平台审计中。
- 不新建分表，复用现有日志表与 management API 资源名。

## 非目标

- 不在平台侧提供代维/跨租户操作汇总视图。
- 不调整日志保留策略（仍使用 `system_settings` 的 `log.*_retention_days`）。
- 不拆分 `audit_logs` / `login_logs` / `api_access_logs` 物理表。

## 数据边界

| 域 | `tenant_id` | 记录内容 | 查询入口 |
|----|-------------|----------|----------|
| 平台 | `0` | 平台登录、租户 CRUD、全局用户、平台管理员、资源授权等平台管理操作 | `/platform/logs/*` |
| 租户 | `> 0` | 成员、部门、角色、文件等租户内操作；代维操作带 `impersonator_id` | `/console/logs/*` |

### 平台操作审计范围（已确认：方案 A）

仅平台域直接操作进入平台审计：

- 平台管理员登录（`login_logs.tenant_id = 0`）
- 平台上下文下的管理写操作（`audit_logs.tenant_id = 0`）
- 平台 API 访问（`api_access_logs.tenant_id = 0`）

代维期间的操作：

- `audit_logs.tenant_id = 当前租户`
- `impersonator_id = 平台管理员 ID`
- 仅在租户控制台 `/console/logs/*` 可见，平台审计不可见

## 后端查询隔离

在 `scopedManagementRead` 对日志类资源（`audit-logs`、`login-logs`、`api-logs`、`log-exports`）强制域过滤：

```text
平台上下文（realm=platform 且 tenant_id=0 且非代维）
  → WHERE tenant_id = 0
  → 跳过 applyDataScopeGen（平台管理员无 member_id）

租户上下文（realm=tenant 或代维）
  → WHERE tenant_id = 当前租户
  → 继续 applyDataScopeGen 数据权限
```

修复现有漏洞：平台 `tenant_id = 0` 时不得因 `tenantScoped` 条件缺失而返回全量租户日志。

平台上下文访问日志资源时，服务端校验调用者为平台管理员（`IsPlatformContext`）。

## 审计写入

现有 `writeAuditOutboxGen` 使用 `scope.TenantID` 写入 Outbox，平台管理操作已自然为 `tenant_id = 0`，写入链路无需改表结构。

代维写入需补齐 `impersonator_id`：

- `writeAuditOutboxGen` 在 `scope.Impersonating` 时将 `impersonator_id` 写入 Outbox payload
- `audit.Entry` 与 `AuditRepository.Publish` 持久化 `impersonator_id`

## 菜单与权限资源

通过 Goose 迁移 `00011_platform_tenant_log_isolation.sql` 调整种子数据（已有环境 UPDATE，新环境同步改 `00001` 可选，以迁移为准）。

### 租户侧（`scope_mask` 从 3 改为 2）

- `menu.logs`
- `login-logs`、`audit-logs`、`api-logs`、`log-exports`

路由保持 `/console/logs/*` 不变。

### 平台侧（新增，`scope_mask = 1`）

挂在 `menu.platform` 下：

| code | 名称 | route_path | component_key |
|------|------|------------|-----------------|
| `menu.platform-logs` | 平台日志 | （目录） | |
| `platform-login-logs` | 登录日志 | `/platform/logs/login` | `platform-login-logs` |
| `platform-audit-logs` | 操作审计 | `/platform/logs/audit` | `platform-audit-logs` |
| `platform-api-logs` | API 日志 | `/platform/logs/api` | `platform-api-logs` |
| `platform-log-exports` | 日志导出 | `/platform/logs/exports` | `platform-log-exports` |

`resources.code` 全局唯一；`component_key` 与 `code` 一致。前端路由 `resourceKey` 仍映射到 management API 资源名（`login-logs` 等），与菜单 code 解耦。

平台管理员在平台上下文拥有 `*:*`，无需为平台日志单独配置 Casbin 规则。

## 前端

### 路由

在 `platformManagementRoutes` 增加：

- `logs/login` → `platform-login-logs` → API `login-logs`
- `logs/audit` → `platform-audit-logs` → API `audit-logs`
- `logs/api` → `platform-api-logs` → API `api-logs`
- `logs/exports` → `platform-log-exports` → API `log-exports`

页面复用 `ResourceListView`，`meta.realm = 'platform'`（继承 PlatformShell）。

### 导航

`componentRouteRegistry` 增加 `platform-*` 路径映射。

`resolveTopNavigation` 在平台上下文将 `/platform/logs/` 前缀归入「平台日志」分组（与「平台治理」并列或作为其子组，以实现时顶栏结构清晰为准）。

租户上下文 `menu.logs` 不再对平台可见（`scope_mask = 2`）。

## 测试要求

### 后端

- 平台上下文列表 `audit-logs` 仅返回 `tenant_id = 0`
- 租户上下文列表 `audit-logs` 仅返回当前 `tenant_id`
- 平台上下文不能通过列表 API 看到租户日志
- 代维写入的审计含 `impersonator_id`，且 `tenant_id > 0`

### 前端

- 平台导航含 `/platform/logs/*`，不含 `/console/logs/*`
- 租户导航含 `/console/logs/*`，不含 `/platform/logs/*`
- `registry` 与路由测试覆盖平台日志路径

## 风险与兼容

- 已有 `scope_mask = 3` 的日志资源在迁移后变为租户专用；平台需重新下发平台日志菜单（迁移 INSERT）。
- 历史平台操作若已写入 `tenant_id = 0`，迁移后可在平台日志页看到；误写入租户 ID 的历史数据不在本次修复范围。

## 验收标准

1. 平台管理员登录后，顶栏可进入平台日志（登录/审计/API/导出）。
2. 平台日志页数据均为 `tenant_id = 0`。
3. 租户管理员只能看到本租户日志，且菜单仅在租户控制台出现。
4. 代维操作出现在租户审计，平台审计不可见。
5. 平台管理员无法通过 API 越权列举租户日志。
