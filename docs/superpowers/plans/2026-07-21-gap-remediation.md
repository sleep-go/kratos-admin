# 平台/租户底座差距补缺实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `2026-07-21-gap-remediation-design.md` 盘点出的 P0 阻塞项与 P1 补强项，使平台/租户底座真正达到「设计即实现」。P2 对齐项作为独立 Task 收尾。

**Architecture:** 沿用现有 `app/admin/internal/{biz,data,service,server,conf}` 分层与 `ManagementService` 统一资源网关。安全收紧集中在 `management_repository.Allowed`、`auth_repository.ListNavigation`、`biz/auth/impersonate`；页面可用性集中在 `managementResources` 注册表与前端 `resourceDefinitions`。

**Tech Stack:** Go 1.26、go-kratos v2.9.2、Casbin、GORM Gen、Goose、Vue 3 + Element Plus。

## Global Constraints

- 保持 Go module `github.com/sleep-go/kratos-admin` 不变。
- 不改变 API 契约主路径、JWT 结构、三类账号模型。
- 开发阶段可直接修改 `migrations/00001_init_schema.sql` 与种子数据，不写增量迁移。
- 所有导出 Go 标识符保留或补充中文 GoDoc；Git Commit Message 使用中文。
- 每个 Task 完成后运行对应测试与 `go vet`，不主动跑全量构建（除非用户要求）。
- 涉及「待确认决策」(D1–D5) 的 Task，实施前需用户拍板；本计划默认按 design 的推荐方案推进。

---

### Task 1: 代维 super admin 校验（G5）

**Files:**
- Modify: `app/admin/internal/biz/auth/impersonate.go`
- Modify: `app/admin/internal/biz/auth/impersonate_test.go`
- Modify: `app/admin/internal/service/platform_auth.go`（前置守卫）

**Interfaces:**
- Consumes: `PlatformAdminRepository.FindByID` 已返回 `IsSuperAdmin`
- Produces: 非 super admin 调用 `Impersonate` 返回 `ErrImpersonateSuperAdminOnly`

- [ ] **Step 1: 写失败测试**

新增测试：普通平台管理员（`is_super_admin=0`）调用 `Impersonate` 返回 `ErrImpersonateSuperAdminOnly`；super admin 正常签发代维令牌。

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth -run TestImpersonate -count=1`

- [ ] **Step 3: 实现 super admin 校验**

在 `impersonate.go` 的 `admins.FindByID` 之后增加 `if !admin.IsSuperAdmin { return ImpersonateResult{}, ErrImpersonateSuperAdminOnly }`；在 `service/platform_auth.go` 的 `Impersonate` 入口加前置守卫（从 claims 反查 admin）。新增错误变量 `ErrImpersonateSuperAdminOnly`。

- [ ] **Step 4: 验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth ./app/admin/internal/service -count=1`

---

### Task 2: 平台 RBAC 收紧（G3 + G4）

**Files:**
- Modify: `app/admin/internal/biz/management/contracts.go`（`Scope` 加 `IsSuperAdmin`）
- Modify: `app/admin/internal/service/management.go`（`managementScope` 透传、`authorize` 收紧）
- Modify: `app/admin/internal/data/management_repository.go`（`Allowed` 平台分支走 Casbin）
- Modify: `app/admin/internal/data/auth_repository.go`（`ListNavigation` 平台分支按 Casbin 过滤 + 祖先补齐）
- Modify: `app/admin/internal/service/platform_auth.go`（`Navigation` 透传 `isSuperAdmin`）
- Modify: 对应 `_test.go`

**Interfaces:**
- Consumes: Casbin `g(v0=0,v1=admin_id)→p(v0=0,v2=code)` 查询能力（`updatePlatformRoleAuthorization` 已具备）
- Produces: 普通 platform admin 受 Casbin 约束的 API 与菜单

- [ ] **Step 1: 写失败测试**

  - API 层：普通平台管理员无角色时调用 `ManagementService.List`（`platform-admins` 资源）返回 PermissionDenied
  - 菜单层：普通平台管理员 `ListNavigation` 仅返回授权资源；super admin 返回全部 `scope_mask&1`
  - super admin 全部放行

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/data ./app/admin/internal/service -run 'Allowed|Navigation' -count=1`

- [ ] **Step 3: `Scope` 增加 `IsSuperAdmin` 并透传**

`contracts.go` 的 `Scope` 加 `IsSuperAdmin bool`；`service/management.go` 的 `managementScope` 在 `IsPlatformContext` 分支调 `platformAdmins.FindByID(claims.UserID)` 填充。

- [ ] **Step 4: `Allowed` 与 `authorize` 收紧**

`management_repository.go` 的 `Allowed`：`scope.PlatformAdmin` 时 super admin 放行，否则走 Casbin `v0=0, v1=scope.UserID`。`service/management.go` 的 `authorize` 同步：移除「`scope.PlatformAdmin` 直接 return nil」，改为调 `Allowed`。

- [ ] **Step 5: `ListNavigation` 平台分支按 Casbin 过滤**

`auth_repository.go` 的 `ListNavigation` 平台分支：super admin 返回全部 `scope_mask&1 && visible=1`；非 super admin JOIN Casbin g/p，并复用 `tenantResourceIDsWithAncestors` 同构逻辑补齐祖先目录。`service/platform_auth.go` 的 `Navigation` 调用透传 `isSuperAdmin`。

- [ ] **Step 6: 验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/... -count=1`

Run: `GOCACHE=/tmp/go-build go vet ./app/admin/internal/...`

---

### Task 3: 平台日志页面可用性（G1 + G2 + 种子）

**前置依赖：D1 决策（默认方案 A：独立 platform-* code）**

**Files:**
- Modify: `migrations/00001_init_schema.sql`（新增 4 个 `platform-*` 日志资源种子）
- Modify: `app/admin/internal/data/management_repository.go`（注册 `platform-*` 资源）
- Modify: `app/admin/internal/data/management_gen_read.go`（`isPlatformLogScope` 改为 code 显式区分）
- Modify: `app/frontend/src/features/management/resourceDefinitions.ts`（补 4 个 `platform-*` definition）
- Modify: 对应 `_test.go`

**Interfaces:**
- Consumes: `login_logs`/`audit_logs`/`api_access_logs`/`log_exports` 表（与租户侧共用）
- Produces: 平台管理员访问 `/platform/logs/*` 正常列表；租户管理员访问 `platform-*` 资源 403

- [ ] **Step 1: 写失败测试**

  - 后端：`platform-login-logs` 等 4 个 code 在 `managementResources` 注册；`scopedManagementRead` 强制 `tenant_id=0`；租户管理员访问返回 PermissionDenied
  - 前端：`resourceDefinitions['platform-login-logs']` 等存在且 `scopeTabs` 固定平台

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/data -run ManagementResource -count=1`

Run: `pnpm --filter frontend test -- resourceDefinitions`

- [ ] **Step 3: 新增 platform-* 日志资源种子**

在 `00001_init_schema.sql` 的 resources 种子段插入 4 行 `platform-login-logs`/`platform-audit-logs`/`platform-api-logs`/`platform-log-exports`，`scope_mask=1, type=2, visible=1`，挂在 `menu.platform.logs` 下。若 `menu.platform.logs` 下已存在同 code 租户侧节点需先清理冲突。

- [ ] **Step 4: 后端注册 platform-* 资源**

`management_repository.go` 的 `managementResources` 新增 4 项，`tenantScoped=false`，`table` 与租户侧相同。`management_gen_read.go` 的 `isPlatformLogScope` 由「上下文判断」改为「code ∈ platform-* 集合」，`scopedManagementRead` 对 `platform-*` 强制 `tenant_id=0` 过滤；租户管理员访问 `platform-*` 在 `isPlatformResource` 列表补充拦截。

- [ ] **Step 5: 前端补 platform-* definition**

`resourceDefinitions.ts` 复制租户侧 4 个 definition，key 改为 `platform-*`，`scopeTabs` 固定 `platform`，字段定义与租户侧一致。

- [ ] **Step 6: 重新生成 GORM Gen 与验证**

Run: `go run ./app/admin/cmd/tools gorm-gen`

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/data -count=1`

Run: `pnpm --filter frontend test && pnpm --filter frontend typecheck`

---

### Task 4: super admin 删除保护与表单角色绑定（G6 + G7）

**前置依赖：D2 决策（默认入口 A：platform-admins 表单虚拟字段）**

**Files:**
- Modify: `app/admin/internal/data/management_repository.go`（`Delete` 计数保护；`platform-admins` writeFields 加 `role_ids` 虚拟字段后置写 Casbin g）
- Modify: `app/admin/internal/data/management_gen_operations.go`（`role_ids` 后置步骤）
- Modify: `app/frontend/src/features/management/resourceDefinitions.ts`（`platform-admins` 表单加角色多选）
- Modify: 对应 `_test.go`

**Interfaces:**
- Consumes: `platform-roles` 列表（前端多选数据源）、Casbin `g(v0=0,v1=admin_id,v2=role_id)`
- Produces: 最后一个 super admin 无法删除；平台管理员表单可勾选角色并保存

- [ ] **Step 1: 写失败测试**

  - 删除最后一个 super admin 返回错误；删除非 super admin 或存在多个 super admin 时正常
  - `platform-admins` Create/Update 携带 `role_ids` 后，Casbin `g` 规则按「先删后建」写入

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/data -run 'Delete|PlatformAdmin' -count=1`

- [ ] **Step 3: 实现删除保护**

`management_repository.go` 的 `Delete` 对 `resource=="platform-admins"`：事务内先 `SELECT is_super_admin FROM platform_admins WHERE id=? AND deleted_at IS NULL`，若为 super admin 再 `COUNT(*) WHERE is_super_admin=1 AND status=1 AND deleted_at IS NULL`，计数 ≤1 则返回 `ErrLastSuperAdmin`。

- [ ] **Step 4: 表单角色绑定虚拟字段**

`platform-admins` 资源 `writeFields` 增加 `role_ids`（虚拟，不落表）；`management_gen_operations.go` 的 Create/Update 后置步骤：先 `DELETE FROM casbin_rules WHERE ptype='g' AND v0='0' AND v1=admin_id`，再批量 `INSERT`。复用 `updatePlatformRoleAuthorization` 的 g 写入逻辑（抽公共方法）。

- [ ] **Step 5: 前端表单加角色多选**

`resourceDefinitions.ts` 的 `platform-admins` fields 增加 `role_ids` 多选，`optionsSource` 指向 `platform-roles` 列表，`valueType: 'array'`。

- [ ] **Step 6: 验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/data -count=1`

Run: `pnpm --filter frontend test -- platform-admins`

---

### Task 5: 审计与导出 realm 校验（G8 + G9 + G17）

**前置依赖：D4 决策（默认是：日志表增加 realm 字段）**

**Files:**
- Modify: `migrations/00001_init_schema.sql`（`audit_logs`/`login_logs`/`api_access_logs`/`log_exports` 加 `realm`）
- Modify: `app/admin/internal/biz/audit/processor.go`（`Entry` 加 `Realm`，`Process` 校验）
- Modify: `app/admin/internal/biz/audit/log.go`（`AccessLogRecord`/`LoginLogRecord` 加 `Realm`）
- Modify: `app/admin/internal/biz/logexport/export.go`（`Access` 加 `Realm`，`validFilters` 收紧）
- Modify: `app/admin/internal/data/audit_repository.go`、`log_repository.go`、`log_export_repository.go`（写入 realm；`ReadRows` 按 realm 过滤）
- Modify: `app/admin/internal/data/management_repository.go`（`writeAuditOutboxGen` 透传 `scope.Realm`）
- Modify: GORM Gen 重新生成
- Modify: 对应 `_test.go`

**Interfaces:**
- Consumes: `scope.Realm`（平台/租户上下文）
- Produces: 审计与导出记录带 realm；平台导出强制时间范围；租户导出强制 tenant_id 过滤

- [ ] **Step 1: 写失败测试**

  - `processor.Process` 对 `realm=platform && tenant_id>0` 返回错误；`realm=tenant && tenant_id=0` 返回错误
  - `log_export_repository.ReadRows` 对 `realm=tenant` 强制 `tenant_id` 过滤；`realm=platform` 缺时间范围时拒绝

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/audit ./app/admin/internal/biz/logexport ./app/admin/internal/data -count=1`

- [ ] **Step 3: 数据库加 realm 字段**

`00001_init_schema.sql` 的四张日志表各加 `realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '域：platform平台，tenant租户，app应用'`，索引 `idx_*_realm_tenant (realm, tenant_id)`。

- [ ] **Step 4: biz 层加 Realm 字段与校验**

`audit/processor.go` 的 `Entry` 与 `audit/log.go` 的 Record 结构加 `Realm string`；`Process` 校验 realm 与 tenant_id 一致性。`logexport/export.go` 的 `Access` 加 `Realm`，`validFilters` 对 `realm=platform` 强制 `created_at` 范围。

- [ ] **Step 5: data 层写入与过滤**

`audit_repository.go`/`log_repository.go` 写入时填 `realm`；`log_export_repository.go` 的 `ReadRows` 按 `realm` 分支：`platform` 允许跨租户但强制时间范围，`tenant` 强制 `tenant_id=scope.TenantID`。`management_repository.go` 的 `writeAuditOutboxGen` 从 `scope.Realm` 透传。

- [ ] **Step 6: 重新生成 GORM Gen 与验证**

Run: `go run ./app/admin/cmd/tools gorm-gen`

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/... -count=1`

---

### Task 6: 租户开户默认初始化（G10）

**前置依赖：D3 决策（默认仅默认角色，管理员手动创建）**

**Files:**
- Create: `app/admin/internal/biz/setup/tenant.go`
- Create: `app/admin/internal/biz/setup/tenant_test.go`
- Modify: `app/admin/internal/data/management_repository.go`（`Create` 对 `tenants` 后置调用）
- Modify: `app/admin/internal/data/setup_repository.go`（提供 `EnsureDefaultRole`）

**Interfaces:**
- Consumes: `roles` 表、Casbin `p(v0=tenant_id)`
- Produces: 新建租户自动获得内置 `tenant-admin` 角色

- [ ] **Step 1: 写失败测试**

新建租户后，`roles` 表存在 `tenant_id=新租户, code='tenant-admin', is_builtin=1` 记录；重复创建幂等。

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/setup -count=1`

- [ ] **Step 3: 实现 TenantProvisioner**

`biz/setup/tenant.go` 的 `TenantProvisioner.EnsureDefaults(ctx, tenantID)`：幂等创建内置角色 `tenant-admin`（`is_builtin=1, data_scope=1, status=1`），绑定 Casbin `p(v0=tenant_id, v1=role_id, v2='*', v3='*')`。若 D3 选择「仅默认角色」，则不创建默认管理员。

- [ ] **Step 4: Create 后置调用**

`management_repository.go` 的 `Create` 对 `resource=="tenants"`：事务提交后调用 `TenantProvisioner.EnsureDefaults(ctx, newTenantID)`。失败时记录审计但不回滚租户创建（默认角色可手动补建）。

- [ ] **Step 5: 验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/setup ./app/admin/internal/data -count=1`

---

### Task 7: 登录顺序与代维全菜单与前端守卫（G11 + G12 + G13）

**Files:**
- Modify: `app/admin/internal/biz/auth/login.go`（先 `FindTenant` 后校验密码）
- Modify: `app/admin/internal/biz/auth/login_test.go`
- Modify: `app/admin/internal/data/auth_repository.go`（`ListNavigation` 代维特权分支）
- Modify: `app/admin/internal/data/auth_repository_test.go`（若存在）
- Modify: `app/frontend/src/router/index.ts`（移除 `resolved.length > 0` 条件）
- Modify: `app/frontend/src/router/index.test.ts`

**Interfaces:**
- Consumes: `FindTenant`、`impersonatorID`
- Produces: 冻结租户登录在密码校验前被拒；超管代维看全部租户菜单；前端空菜单时拦截

- [ ] **Step 1: 写失败测试**

  - 登录：冻结租户的账号调用 `Login`，密码未校验即返回 `ErrTenantFrozen`
  - 菜单：`impersonatorID>0` 的代维会话 `ListNavigation` 返回全部 `scope_mask&2` 资源
  - 前端：`resolved` 为空时访问 `/console/logs/audit` 被重定向

- [ ] **Step 2: 运行测试确认失败**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth ./app/admin/internal/data -count=1`

Run: `pnpm --filter frontend test -- router`

- [ ] **Step 3: 调整登录顺序**

`login.go` 的 `Login`：`FindByIdentifier` 命中后，先 `FindTenant` 校验租户状态，冻结则返回 `ErrTenantFrozen`；再 `hasher.Verify` 密码。

- [ ] **Step 4: 代维全菜单特权分支**

`auth_repository.go` 的 `ListNavigation` 租户分支前：`if realm==RealmTenant && impersonatorID>0` 返回全部 `scope_mask&2 && visible=1`，跳过 `tenant_resources` join。

- [ ] **Step 5: 前端守卫收紧**

`router/index.ts` 移除 `resolved.length > 0 &&`；空菜单且目标不在白名单时重定向 `defaultHome`。

- [ ] **Step 6: 验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth ./app/admin/internal/data -count=1`

Run: `pnpm --filter frontend test -- router`

---

### Task 8: Wire ProviderSet 对齐（G14）

**Files:**
- Modify: `app/admin/internal/biz/auth/*.go`、`biz/audit/*.go`、`biz/file/*.go`、`biz/logexport/*.go`、`biz/management/*.go`、`biz/permission/*.go`、`biz/setting/*.go`、`biz/setup/*.go`、`biz/storage/*.go`（各子包补 `ProviderSet`）
- Modify: `app/admin/cmd/server/wire.go`（注入 `biz.ProviderSet`）
- Modify: `app/admin/cmd/server/wire_gen.go`（重新生成）
- Modify: `app/admin/cmd/server/services.go`（逐步移除手工 `provideServices`）

**Interfaces:**
- Consumes: 现有 biz 用例构造函数
- Produces: `wireApp` 通过 `biz.ProviderSet` 装配，与 kratos-layout 风格一致

- [ ] **Step 1: 各 biz 子包补 ProviderSet**

每个 biz 子包新增 `var ProviderSet = wire.NewSet(NewXxxUsecase, ...)`，聚合到 `biz/provider.go` 的 `biz.ProviderSet`。

- [ ] **Step 2: wire.go 注入并重新生成**

`wire.go` 的 `wire.Build` 增加 `biz.ProviderSet`；`services.go` 的 `provideServices` 逐步替换为 ProviderSet 注入（保留 `Services` 聚合结构）。

Run: `GOCACHE=/tmp/go-build go tool wire ./app/admin/cmd/server`

- [ ] **Step 3: 验证**

Run: `GOCACHE=/tmp/go-build go build ./app/admin/cmd/server`

Run: `GOCACHE=/tmp/go-build go test ./app/admin/... -count=1`

---

### Task 9: 文档与种子清理（G15 + G16）

**前置依赖：D5 决策（默认否：保持 platform-casbin-rules 隐藏）**

**Files:**
- Modify: `docs/superpowers/specs/2026-07-21-platform-menu-rbac-design.md`（追加 G15 决策记录）
- Modify: `migrations/00001_init_schema.sql`（清理 `menu.platform` 旧根节点，若 D5/G16 同意）

**Interfaces:**
- Produces: 设计文档与实现一致；种子无冗余节点

- [ ] **Step 1: 追加决策记录**

在 `platform-menu-rbac-design.md` 的「已确认决策」表追加：`platform-casbin-rules` 保持 `type=3 visible=0`，作为 RolePermissionView 内部 API 资源，不恢复独立菜单入口。

- [ ] **Step 2: 清理 menu.platform 旧根节点**

`00001_init_schema.sql` 删除 `menu.platform` 种子行（`visible=0` 的冗余根节点），确认无引用。

- [ ] **Step 3: 验证**

Run: `GOCACHE=/tmp/go-build go test ./app/admin/... -count=1`

Run: `docker compose config --quiet`

---

## 验收

- P0（Task 1/2/3）全部完成：普通平台管理员无角色时无法访问平台治理 API 与菜单；代维仅 super admin；平台日志四类页面可用
- P1（Task 4/5/6/7）全部完成：最后 super admin 不可删；审计与导出带 realm；新租户有默认角色；冻结租户登录前置拒绝；代维全菜单；前端空菜单拦截
- P2（Task 8/9）收尾：Wire ProviderSet 对齐；文档与种子清理
- `GOCACHE=/tmp/go-build go test ./app/admin/... -count=1` 通过
- `GOCACHE=/tmp/go-build go vet ./app/admin/...` 通过
- `pnpm --filter frontend test && pnpm --filter frontend typecheck` 通过
- `make config && make wire && make gorm-gen && git diff --exit-code` 无生成差异
- `docker compose config --quiet` 通过

## 提交策略

每个 Task 独立提交，commit message 中文，按「重构/修复/补齐」前缀分类。P0 三个 Task 可合并为一个 PR 优先 review，P1 四个 Task 为第二个 PR，P2 两个 Task 为第三个 PR。
