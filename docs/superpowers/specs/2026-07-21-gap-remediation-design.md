# 平台/租户底座差距盘点与补缺设计

## 背景

项目已按 `kratos-layout` v2.9.2 完成目录与启动入口对齐，并落地了三类独立账号、平台菜单树、租户菜单授权、租户开户配置等主体能力。本次在对全仓库代码与既有设计文档逐项盘点后，发现一批「设计已定但实现未落地」或「实现与设计偏离」的差距，集中在平台 RBAC 收紧、平台日志页面可用性、代维安全、租户开户初始化等链路。

本设计给出分级补缺方案，供 review 后转入实施计划。开发阶段不考虑兼容，可直接调整 `00001_init_schema.sql` 与种子数据。

## 目标

- 关闭平台 RBAC 方案 A 的 API 层与菜单层缺口，使普通平台管理员受 Casbin 约束
- 让平台日志四类页面真正可用（前端 definition + 后端资源注册 + 隔离查询）
- 收紧代维与删除操作的安全边界
- 补齐租户开户的默认初始化与登录顺序校验
- 对齐 `kratos-layout` 的 Wire ProviderSet 风格

## 非目标

- 不改变三类账号模型、JWT 结构、API 契约主路径
- 不重构 `ManagementService` 统一资源网关架构
- 不引入 App 用户侧 RBAC（App 域权限后续迭代）
- 不重组织/部门/岗位表（阶段 1 延后）

---

## 差距总表

| # | 优先级 | 差距 | 证据 |
|---|--------|------|------|
| G1 | P0 阻塞 | 平台日志页面前端「未找到资源定义」 | `app/frontend/src/features/management/resourceDefinitions.ts` 无 `platform-*` key；`ResourceListView.vue:756` 兜底报错 |
| G2 | P0 阻塞 | 后端未注册 `platform-*` 日志资源 | `app/admin/internal/data/management_repository.go:82-85` 仅注册租户侧 4 个 code |
| G3 | P0 阻塞 | 平台 API 鉴权对非 super admin 完全失效 | `management_repository.go:151-179` `Allowed` 对 `PlatformAdmin` 无条件 `return true`；`service/management.go:197-209` `authorize` 同样跳过 |
| G4 | P0 阻塞 | 平台菜单对非 super admin 未按 Casbin 过滤 | `auth_repository.go:285-302` 平台分支仅 `scope_mask&1`，未按 admin_id 过滤 |
| G5 | P0 阻塞 | 代维未校验 `IsSuperAdmin` | `biz/auth/impersonate.go:55-66` 仅校验 realm 与账号状态 |
| G6 | P1 补强 | 无「最后一个 super admin」删除保护 | `management_repository.go:818-855` `Delete` 无计数校验 |
| G7 | P1 补强 | 平台管理员编辑表单无法绑定角色 | `management_repository.go:73` `writeFields` 无 `role_ids`；`resourceDefinitions.ts:199-238` 表单无角色字段 |
| G8 | P1 补强 | 审计 Processor 无 realm/scope_mask 校验 | `biz/audit/processor.go:52-65` 盲信 payload `tenant_id` |
| G9 | P1 补强 | `log_exports` 无 realm 字段，平台导出跨租户读日志缺二次校验 | `biz/logexport/export.go:42-47` `Access` 无 realm；`data/log_export_repository.go:153,174,195` 仅按 `tenant_id==0` 放行 |
| G10 | P1 补强 | 租户开户无默认初始化（默认角色/默认管理员） | `biz/setup/` 仅有 `admin.go`；`ManagementRepository.Create` 对 `tenants` 无后置步骤 |
| G11 | P1 补强 | 登录顺序：密码先校验后查租户状态 | `biz/auth/login.go:148-198` 先 `FindByIdentifier`+`Verify` 再 `FindTenant` |
| G12 | P1 补强 | 超管代维无法看全部租户菜单（排障受限） | `auth_repository.go:289-296` 代维走租户分支，无 `impersonatorID>0` 特权 |
| G13 | P1 补强 | 前端守卫在 `resolved.length===0` 时不拦截 | `router/index.ts:203-212` 条件含 `resolved.length > 0` |
| G14 | P2 对齐 | `biz` 包无 `ProviderSet`，依赖 `cmd/server/services.go` 手工装配 | `wire.go:21-27` 无 `biz.ProviderSet` |
| G15 | P2 对齐 | `platform-casbin-rules` 被降级为隐藏 API，与设计文档菜单入口冲突 | `migrations/00002_hide_platform_casbin_menu.sql:3`；`router/index.ts:96-99` redirect |
| G16 | P2 对齐 | `menu.platform` 旧根节点仍保留 `visible=0` | `migrations/00001_init_schema.sql:419` |
| G17 | P2 对齐 | 四张日志表缺 `realm` 字段（当前用 `tenant_id=0` 区分平台域） | `migrations/00001_init_schema.sql:190-265` |

---

## P0 阻塞项设计

### G1+G2 平台日志页面可用性

**问题**：前端 `platformManagementRoutes` 把 `/platform/logs/login` 等路由的 `resourceKey` 设为 `platform-login-logs` 等，但 `resourceDefinitions.ts` 与后端 `managementResources` 均无这些 key，平台管理员进入页面直接看到「未找到资源定义」。

**方案 A（推荐）**：注册独立的 `platform-*` 资源，与租户侧 `login-logs` 等彻底分离。

- 后端 `management_repository.go` 新增 4 个资源定义：`platform-login-logs`、`platform-audit-logs`、`platform-api-logs`、`platform-log-exports`，`tenantScoped=false`，读写表与租户侧相同，但 `scopedManagementRead` 强制 `tenant_id=0` 过滤
- `isPlatformLogScope` 由「上下文判断」改为「资源 code 显式区分」，租户管理员访问 `platform-*` 直接 403
- 前端 `resourceDefinitions.ts` 补 4 个 `platform-*` definition，字段与租户侧一致，`scopeTabs` 固定平台
- `biz/audit/processor.go` 与 `data/log_repository.go` 写入时：平台域事件（`realm=platform` 或 `tenant_id=0 && impersonator==0`）落 `tenant_id=0`，与平台资源查询对齐

**方案 B（备选）**：复用租户侧 4 个 code，靠 `scope.PlatformAdmin` 切换 `tenant_id=0` 过滤。问题：租户管理员与平台管理员共用同一 resourceKey，权限边界靠运行时判断，易回归，不推荐。

**待确认**：是否接受方案 A 的「8 个日志资源 code 并存」（4 平台 + 4 租户）。

### G3 平台 API 鉴权收紧

**问题**：`Allowed` 与 `authorize` 对 `scope.PlatformAdmin` 一律放行，方案 A 在 API 层未生效。

**方案**：

1. `managementbiz.Scope` 增加字段 `IsSuperAdmin bool`，由 `service/management.go` 的 `managementScope` 从 `platformAdmins.FindByID` 结果透传
2. `ManagementRepository.Allowed` 改为：
   ```go
   if scope.PlatformAdmin {
       if scope.IsSuperAdmin { return true, nil }
       // 走 Casbin v0=0, v1=admin_id
       return r.casbinAllowed(ctx, 0, scope.UserID, resource, action)
   }
   ```
3. `service/management.go` 的 `authorize` 同步：非 super admin 平台管理员走 Casbin
4. 平台资源 `scope_mask&1` 校验在 Casbin 策略写入时已保证（`updatePlatformRoleAuthorization` 已校验），运行时不再重复

**注意**：`scope.UserID` 在平台上下文为 `platform_admin_id`，需确认 `managementScope` 正确填充（当前 `contracts.go` Scope 有 `UserID` 字段）。

### G4 平台菜单按 Casbin 过滤

**问题**：`ListNavigation` 平台分支仅 `scope_mask&1`，普通平台管理员能看到全部 4 一级目录。

**方案**：

- `data/auth_repository.go` 的 `ListNavigation` 平台分支：
  - super admin → 返回全部 `scope_mask&1 && visible=1` 资源
  - 非 super admin → JOIN Casbin `g(v0=0,v1=admin_id)→p(v0=0,v2=resource.code)`，仅返回授权资源；同时自动补齐祖先目录节点（与租户侧 `tenantResourceIDsWithAncestors` 同构），保证菜单树完整
- `service/platform_auth.go` 的 `Navigation` 调用需透传 `isSuperAdmin`（当前未传）

### G5 代维校验 super admin

**问题**：`impersonate.go` 仅校验 realm 与账号状态，任意启用的平台管理员可代维。

**方案**：在 `biz/auth/impersonate.go:62-66` 的 `admins.FindByID` 之后增加：

```go
if !admin.IsSuperAdmin {
    return ImpersonateResult{}, ErrImpersonateSuperAdminOnly
}
```

`PlatformAdminRepository.FindByID` 已返回 `IsSuperAdmin`（`platform_admin_repository.go:121`），无需新增查询。同步在 `service/platform_auth.go` 的 `Impersonate` 入口加前置守卫，避免业务层遗漏。

---

## P1 补强项设计

### G6 最后一个 super admin 删除保护

`management_repository.go` 的 `Delete` 对 `resource=="platform-admins"` 增加前置校验：事务内 `COUNT(*) WHERE is_super_admin=1 AND status=1 AND deleted_at IS NULL`，若目标管理员是 super admin 且计数 ≤1 则拒绝。同样适用于 `Update` 把 super admin 降级为普通管理员的场景（`is_super_admin` 当前不在 writeFields，本项可不处理 Update 路径，仅保护 Delete）。

### G7 平台管理员编辑表单角色绑定

两个入口选择：

- **入口 A（推荐）**：在 `platform-admins` 资源 `writeFields` 增加 `role_ids []uint64` 虚拟字段，`Create`/`Update` 后置步骤写 Casbin `g(v0=0,v1=admin_id,v2=role_id)`（先删后建，与 `updatePlatformRoleAuthorization` 同构）。前端 `resourceDefinitions.ts` 增加多选角色字段，数据源为 `platform-roles` 列表。
- **入口 B**：保持现状，仅从 `RolePermissionView` 反向勾选，但在平台管理员详情页增加「已绑角色」只读展示。

**待确认**：倾向入口 A（双向可编辑），但需确认是否接受 `platform-admins` 资源新增虚拟字段带来的 `management_gen` 适配工作量。

### G8 审计 Processor realm 校验

`biz/audit/processor.go` 的 `Entry` 增加 `Realm string` 字段，`Process` 在 `Publish` 前校验：

- `realm=platform` 时 `tenant_id` 必须为 0
- `realm=tenant` 时 `tenant_id` 必须 >0
- `realm=app` 暂不写入 `audit_logs`（App 域审计后续迭代）

写入端 `management_repository.go` 的 `writeAuditOutboxGen` 透传当前 `scope.Realm`，保证 payload 与上下文一致。

### G9 log_exports realm 校验

`biz/logexport/export.go` 的 `Access` 增加 `Realm string`。`data/log_export_repository.go` 的 `ReadRows`：

- `realm=platform` 允许跨租户读取（平台导出语义），但 `validFilters` 强制必须显式指定时间范围，禁止无条件全量导出
- `realm=tenant` 强制 `tenant_id=scope.TenantID` 过滤

### G10 租户开户默认初始化

新增 `biz/setup/tenant.go` 提供 `TenantProvisioner.EnsureDefaults(ctx, tenantID)`：

1. 创建内置租户角色 `tenant-admin`（`is_builtin=1, data_scope=1`），绑定 Casbin `p(v0=tenant_id, v1=role_id, v2=*, v3=*)` 或按 `tenant_resources` 授权集合
2. （可选）创建默认租户管理员，密码由平台管理员在开通配置 Tab 手动设置——若不自动创建，则在 `TenantSetupView` 的「租户管理员」Tab 显著提示「请尽快创建首个租户管理员」

`ManagementRepository.Create` 对 `resource=="tenants"` 在事务提交后调用 `TenantProvisioner.EnsureDefaults`。

**待确认**：是否自动创建默认租户管理员（需要密码输入），还是仅创建默认角色、管理员留给平台管理员手动在开通配置创建。

### G11 登录顺序调整

`biz/auth/login.go` 的 `Login`：`FindByIdentifier` 命中后，先 `FindTenant`（按 `tenant_id`）校验租户状态，再 `hasher.Verify` 密码。冻结租户直接返回 `ErrTenantFrozen`，避免密码试探。

### G12 超管代维全菜单特权

`data/auth_repository.go` 的 `ListNavigation` 租户分支前增加：

```go
if realm == RealmTenant && impersonatorID > 0 {
    // 代维超管：返回全部 scope_mask&2 资源，跳过 tenant_resources join
    return query.Where(resource.ScopeMask.BitAnd(2).Eq(2))...
}
```

仅 super admin 能进入代维（G5 已保证），故此分支安全。

### G13 前端守卫空菜单拦截

`router/index.ts:207` 移除 `resolved.length > 0 &&` 条件。当 `resolved` 为空且目标路径不在白名单（`/console`、`/console/account`、`/platform/account`）时，重定向到 `defaultHome` 或新增「无权限」提示页。

---

## P2 对齐项建议

| # | 建议 |
|---|------|
| G14 | `biz` 各子包补 `ProviderSet`，`wire.go` 注入 `biz.ProviderSet`，逐步替换 `cmd/server/services.go` 的手工 `provideServices`。可独立 PR，不阻塞 P0/P1 |
| G15 | 保持 `platform-casbin-rules` 为隐藏 API（合并到平台角色页），在 `platform-menu-rbac-design.md` 追加决策记录，不恢复独立菜单入口 |
| G16 | 清理 `menu.platform` 旧根节点（`DELETE` 或保留 `visible=0` 均可，功能无影响） |
| G17 | 四张日志表增加 `realm VARCHAR(16)` 字段，与 `auth_sessions.realm` 对齐，作为平台/租户域的显式标记。`tenant_id=0` 仍可作为兜底过滤，但新写入以 `realm` 为准 |

---

## 数据库变更摘要

开发阶段直接修改 `migrations/00001_init_schema.sql`（如需保留 00002 则合并后删除）：

1. `audit_logs`、`login_logs`、`api_access_logs`、`log_exports` 增加 `realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '域：platform平台，tenant租户，app应用'`（G8/G9/G17）
2. `resources` 种子：新增 4 个 `platform-*` 日志资源（G1/G2），清理 `menu.platform` 旧根节点（G16）
3. `tenant_resources` 是否增加 `enabled` 字段：当前行存在语义功能等价，**本设计不新增**，保持删-建语义

**SQL 样例（platform-* 日志资源种子）**：

```sql
INSERT INTO resources (parent_id, scope_mask, type, code, name, route_path, component_key, icon, sort_order, visible, status) VALUES
((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p),
 1, 2, 'platform-login-logs', '登录日志', '/platform/logs/login', 'platform-login-logs', 'Document', 41, 1, 1),
((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p),
 1, 2, 'platform-audit-logs', '操作审计', '/platform/logs/audit', 'platform-audit-logs', 'List', 42, 1, 1),
((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p),
 1, 2, 'platform-api-logs', 'API日志', '/platform/logs/api', 'platform-api-logs', 'Connection', 43, 1, 1),
((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p),
 1, 2, 'platform-log-exports', '日志导出', '/platform/logs/exports', 'platform-log-exports', 'Download', 44, 1, 1);
```

---

## 待确认决策

| # | 决策点 | 推荐 |
|---|--------|------|
| D1 | 平台日志资源：独立 `platform-*` code（方案 A） vs 复用租户 code + scope 切换（方案 B） | 方案 A |
| D2 | 平台管理员角色绑定入口：`platform-admins` 表单虚拟字段（入口 A） vs 仅 RolePermissionView 反向勾选（入口 B） | 入口 A |
| D3 | 租户开户默认初始化：自动创建默认角色+默认管理员 vs 仅默认角色、管理员手动创建 | 仅默认角色 |
| D4 | 四张日志表是否增加 `realm` 字段 | 是（G8/G9 依赖） |
| D5 | `platform-casbin-rules` 是否恢复为独立菜单入口 | 否（保持隐藏，更新文档） |

---

## 验收标准

- P0 全部修复：普通平台管理员无角色时无法访问任何平台治理 API 与菜单；代维仅 super admin 可用；平台日志四类页面可正常列表/过滤
- P1 全部修复：最后一个 super admin 无法删除；审计与导出带 realm 校验；新开租户自动有默认角色；冻结租户登录在密码校验前被拒
- P2 按独立 PR 推进，不阻塞 P0/P1
- `go test ./app/admin/...`、`go vet ./app/admin/...`、前端 `pnpm test`、`pnpm typecheck`、`docker compose config --quiet` 全部通过
- 集成测试覆盖：普通平台管理员越权场景、代维非 super admin 场景、平台日志跨租户隔离场景
