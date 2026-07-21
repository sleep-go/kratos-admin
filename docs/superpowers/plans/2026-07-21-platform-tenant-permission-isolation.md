# 平台与租户权限隔离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将平台管理员能力限定在平台上下文，建立资源适用范围，并提供平台域租户初始化入口，使租户菜单、操作权限和数据查询始终服从可信租户边界。

**Architecture:** 保留全局用户与租户成员模型，以 `claims.platform_admin && claims.tenant_id == 0` 作为唯一平台特权判定。资源表使用 `scope_mask` 明确平台、租户或共用范围；通用管理 API 增加只对平台上下文开放的 `target_tenant_id`，复用现有仓储完成租户初始化，不让平台管理员获得租户会话。

**Tech Stack:** Go 1.24、Kratos v2、GORM Gen、MySQL 8/Goose、Protocol Buffers、Vue 3 `<script setup>`、TypeScript、Pinia、Vitest。

## Global Constraints

- 创建租户时不创建用户、成员、角色或管理员关系，也不自动授权租户功能。
- 平台特权只在 `claims.platform_admin = true AND claims.tenant_id = 0` 时生效。
- 平台管理员进入具体租户后必须按租户成员身份授权。
- 租户允许没有管理员，平台管理员通过平台域初始化入口维护。
- `scope_mask` 取值：`1` 仅平台、`2` 仅租户、`3` 平台与租户共用。
- `tenant_resources` 禁止关联 `scope_mask & 2 = 0` 的平台专属资源。
- 普通租户接口不信任客户端传入的租户 ID；只有平台上下文可使用 `target_tenant_id`。
- 不新增第三方依赖，不批量删除或改写现有租户成员关系。
- 保留工作区已有未提交修改；每次提交只暂存当前任务列出的文件。

---

### Task 1: 为权限资源增加平台与租户适用范围

**Files:**
- Create: `migrations/00007_resource_scope.sql`
- Regenerate: `app/admin/internal/data/model/resources.gen.go`
- Regenerate: `app/admin/internal/data/query/resources.gen.go`
- Test: `app/admin/internal/data/model/schema_test.go`
- Test: `app/admin/internal/data/management_repository_test.go`

**Interfaces:**
- Produces: 数据库字段 `resources.scope_mask TINYINT UNSIGNED NOT NULL DEFAULT 3`。
- Produces: Go 字段 `model.Resource.ScopeMask uint8` 和 `query.Resource.ScopeMask field.Uint8`。
- Produces: 内置资源的明确范围映射，后续菜单和租户功能授权直接查询该字段。

- [ ] **Step 1: 写迁移失败测试**

在 `schema_test.go` 增加断言，要求 `model.Resource` 存在 `ScopeMask`；在 MySQL 仓储测试中查询 `resources.scope_mask` 并断言：

```go
if (model.Resource{}).ScopeMask != 0 {
	t.Fatal("Resource.ScopeMask 零值应为0")
}

var rows []struct {
	Code      string
	ScopeMask uint8
}
if err := tx.Table("resources").Select("code, scope_mask").
	Where("code IN ?", []string{"users", "members", "audit-logs"}).Scan(&rows).Error; err != nil {
	t.Fatal(err)
}
```

测试的目标映射为：`users=1`、`members=2`、`audit-logs=3`。

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/data/model ./app/admin/internal/data -run 'Test.*Resource.*Scope' -count=1
```

Expected: FAIL，提示 `ScopeMask undefined` 或数据库不存在 `scope_mask`。

- [ ] **Step 3: 添加可执行 Goose 迁移**

创建 `00007_resource_scope.sql`：

```sql
-- +goose Up
ALTER TABLE resources
    ADD COLUMN scope_mask TINYINT UNSIGNED NOT NULL DEFAULT 3
        COMMENT '资源适用范围位标记：1仅平台，2仅租户，3平台与租户共用'
        AFTER type;

UPDATE resources SET scope_mask = 1
WHERE code IN ('menu.platform', 'users', 'tenants', 'resources', 'tenant-resources');

UPDATE resources SET scope_mask = 2
WHERE code IN ('menu.organization', 'members', 'departments', 'positions', 'roles', 'casbin-rules');

UPDATE resources SET scope_mask = 3
WHERE code IN (
    'menu.permission', 'menu.logs', 'menu.storage', 'menu.settings',
    'login-logs', 'audit-logs', 'api-logs', 'log-exports', 'files',
    'settings', 'providers', 'dictionary-types', 'dictionary-items'
);

INSERT INTO resources (
    parent_id, type, scope_mask, code, name, route_path, component_key,
    icon, sort_order, visible, status
)
SELECT id, 3, 1, 'tenant-setup', '租户初始化', '', '', 'Tools', 13, 0, 1
FROM resources WHERE code = 'menu.platform'
ON DUPLICATE KEY UPDATE scope_mask = 1, name = '租户初始化', visible = 0, status = 1;

-- +goose Down
DELETE FROM resources WHERE code = 'tenant-setup';
ALTER TABLE resources DROP COLUMN scope_mask;
```

- [ ] **Step 4: 重新生成 GORM Model 与 Query**

Run:

```bash
GOCACHE=/tmp/go-build make gorm-gen
```

Expected: `resources.gen.go` 出现 `ScopeMask uint8`，生成命令不访问配置中的业务数据库。

- [ ] **Step 5: 完成迁移测试并确认 GREEN**

把 `rows` 转换为 `map[string]uint8`，逐项断言 `users=1`、`members=2`、`audit-logs=3`，避免依赖数据库返回顺序。

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/data/model ./app/admin/internal/data -run 'Test.*Resource.*Scope' -count=1
```

Expected: PASS。

- [ ] **Step 6: 提交本任务文件**

```bash
git add migrations/00007_resource_scope.sql app/admin/internal/data/model/resources.gen.go app/admin/internal/data/query/resources.gen.go app/admin/internal/data/model/schema_test.go app/admin/internal/data/management_repository_test.go
git commit -m "权限：增加资源平台与租户范围"
```

---

### Task 2: 统一平台上下文与租户上下文授权判定

**Files:**
- Modify: `app/admin/internal/biz/auth/session.go`
- Modify: `app/admin/internal/data/auth_repository.go`
- Modify: `app/admin/internal/service/management.go`
- Modify: `app/admin/internal/service/log.go`
- Modify: `app/admin/internal/biz/logexport/export.go`
- Test: `app/admin/internal/biz/auth/session_test.go`
- Test: `app/admin/internal/service/management_test.go`
- Test: `app/admin/internal/service/log_test.go`
- Test: `app/admin/internal/data/auth_repository_integration_test.go`

**Interfaces:**
- Produces: `func IsPlatformContext(claims *TokenClaims) bool`。
- Produces: `management.Scope.PlatformAdmin` 表示当前请求的有效平台上下文，而不是用户的全局能力。
- Consumes: Task 1 的 `resources.scope_mask`。

- [ ] **Step 1: 写平台管理员进入租户后不能绕过权限的失败测试**

在 `management_test.go` 将旧的“租户上下文仍可跨租户治理”断言改为拒绝：

```go
func TestPlatformAdministratorInTenantContextDoesNotBypassPermission(t *testing.T) {
	checker := &fakePermissionChecker{allowed: false}
	service := NewManagementService(&fakeManagementRepository{}, checker)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{
		UserID: 5, TenantID: 8, MemberID: 9, PlatformAdmin: true,
	})
	_, err := service.ListResources(ctx, &v1.ListResourcesRequest{Resource: "members"})
	if kratoserrors.Reason(err) != "PERMISSION_DENIED" {
		t.Fatalf("reason = %q, want PERMISSION_DENIED", kratoserrors.Reason(err))
	}
}
```

在 `log_test.go` 增加同等断言，确保租户上下文会调用权限检查器；在 `session_test.go` 保留并扩展 `TestNavigationScopesPlatformAdministratorToSelectedTenant`。

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth ./app/admin/internal/service -run 'TestPlatformAdministratorInTenantContext|TestNavigationScopes' -count=1
```

Expected: 至少一个测试 FAIL，显示平台标记仍绕过检查。

- [ ] **Step 3: 增加唯一的平台上下文判定函数**

在 `session.go` 添加：

```go
// IsPlatformContext 判断令牌是否处于可信平台治理上下文。
func IsPlatformContext(claims *TokenClaims) bool {
	return claims != nil && claims.PlatformAdmin && claims.TenantID == 0
}
```

`Navigation` 继续把平台参数收敛为：

```go
return u.repository.ListNavigation(ctx, tenantID, memberID, platformAdmin && tenantID == 0)
```

- [ ] **Step 4: 替换服务层的全局平台标记绕过**

`managementScope` 和日志导出访问对象必须使用 `IsPlatformContext`：

```go
platformContext := bizauth.IsPlatformContext(claims)
return managementbiz.Scope{
	TenantID: claims.TenantID,
	UserID: claims.UserID,
	MemberID: claims.MemberID,
	PlatformAdmin: platformContext,
}, nil
```

平台专属资源 `users`、`tenants`、`resources`、`tenant-resources`、`tenant-setup` 只有 `platformContext` 为真时可访问。`LogService.access` 同样只把有效平台上下文写入 `logexport.Access.PlatformAdmin`。

- [ ] **Step 5: 按资源范围重写菜单查询**

`ListNavigation` 的基础过滤增加：

```go
if platformAdmin && tenantID == 0 {
	query = query.Where("res.scope_mask & 1 = 1")
} else {
	query = query.Where("res.scope_mask & 2 = 2")
	query = query.Joins(
		"JOIN tenant_resources AS tr ON tr.resource_id = res.id AND tr.tenant_id = ?", tenantID,
	)
	// 租户管理员跳过角色连接，普通成员继续连接 p/g 策略。
}
```

删除平台菜单固定代码名单。仓储集成测试分别断言平台上下文不返回 `members`、租户上下文不返回 `users`，共用的 `audit-logs` 可按上下文返回。

- [ ] **Step 6: 运行授权与菜单测试确认 GREEN**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth ./app/admin/internal/service ./app/admin/internal/data -run 'TestPlatform|TestNavigation|Test.*TenantContext' -count=1
```

Expected: PASS。

- [ ] **Step 7: 提交本任务文件**

```bash
git add app/admin/internal/biz/auth/session.go app/admin/internal/data/auth_repository.go app/admin/internal/service/management.go app/admin/internal/service/log.go app/admin/internal/biz/logexport/export.go app/admin/internal/biz/auth/session_test.go app/admin/internal/service/management_test.go app/admin/internal/service/log_test.go app/admin/internal/data/auth_repository_integration_test.go
git commit -m "权限：隔离平台与租户授权上下文"
```

---

### Task 3: 创建租户时不再自动创建管理员成员

**Files:**
- Modify: `app/admin/internal/data/management_repository.go`
- Modify: `app/frontend/src/features/management/resourceDefinitions.ts`
- Test: `app/admin/internal/data/management_repository_test.go`
- Test: `app/frontend/src/features/management/resourceDefinitions.test.ts`

**Interfaces:**
- Produces: 创建 `tenants` 只写租户记录。
- Removes: 前端字段 `admin_user_id` 及“当前平台管理员”默认语义。

- [ ] **Step 1: 写租户创建不产生成员的失败测试**

将仓储测试改为：

```go
id, err := repository.Create(ctx, managementbiz.Scope{UserID: admin.ID, PlatformAdmin: true}, "tenants", map[string]any{
	"code": "empty-tenant", "name": "待初始化租户", "status": 1,
})
if err != nil {
	t.Fatal(err)
}
var memberCount int64
if err := tx.Table("tenant_members").Where("tenant_id = ?", id).Count(&memberCount).Error; err != nil {
	t.Fatal(err)
}
if memberCount != 0 {
	t.Fatalf("member count = %d, want 0", memberCount)
}
```

前端测试断言租户字段集合不包含 `admin_user_id`。

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/data -run TestCreateTenant -count=1
cd app/frontend && pnpm test:run src/features/management/resourceDefinitions.test.ts
```

Expected: 后端发现自动成员数量为 1，前端仍存在管理员字段。

- [ ] **Step 3: 删除租户创建事务中的自动成员逻辑**

从 `ManagementRepository.Create` 删除 `resource == "tenants"` 时读取 `admin_user_id`、回退 `scope.UserID` 和插入 `tenant_members` 的代码。保留租户主记录、审计 Outbox 和其余资源逻辑。

从租户前端定义中删除：

```ts
{
  key: 'admin_user_id',
  label: '租户管理员',
  type: 'relation',
  // ...
}
```

- [ ] **Step 4: 运行定向测试确认 GREEN**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/data -run TestCreateTenant -count=1
cd app/frontend && pnpm test:run src/features/management/resourceDefinitions.test.ts
```

Expected: PASS。

- [ ] **Step 5: 提交本任务文件**

```bash
git add app/admin/internal/data/management_repository.go app/admin/internal/data/management_repository_test.go app/frontend/src/features/management/resourceDefinitions.ts app/frontend/src/features/management/resourceDefinitions.test.ts
git commit -m "租户：创建后再初始化角色和用户"
```

---

### Task 4: 为平台域提供显式目标租户初始化 API

**Files:**
- Modify: `api/admin/v1/management.proto`
- Regenerate: `api/admin/v1/management.pb.go`
- Regenerate: `api/admin/v1/management_grpc.pb.go`
- Regenerate: `api/admin/v1/management_http.pb.go`
- Regenerate: `api/openapi.yaml`
- Regenerate: `app/frontend/src/api/generated/*`
- Modify: `app/admin/internal/biz/management/contracts.go`
- Modify: `app/admin/internal/service/management.go`
- Modify: `app/admin/internal/data/management_repository.go`
- Modify: `app/frontend/src/api/management.ts`
- Test: `app/admin/internal/service/management_test.go`
- Test: `app/admin/internal/data/management_repository_test.go`
- Test: `app/frontend/src/api/management.test.ts`

**Interfaces:**
- Produces: `target_tenant_id` on list/create/update/delete resource requests and role authorization requests。
- Produces: 平台上下文可对允许的租户初始化资源建立可信 `management.Scope{TenantID: target, PlatformAdmin: true}`。
- Consumes: Task 2 的 `IsPlatformContext`。

- [ ] **Step 1: 写目标租户越权失败测试**

服务测试覆盖三种情况：

```go
// 平台上下文 + target_tenant_id=8 + resource=roles：允许，仓储 scope.TenantID 必须为8。
// 租户上下文 + target_tenant_id=8：返回 PLATFORM_ADMIN_REQUIRED。
// 平台上下文 + target_tenant_id=8 + resource=users：返回 TENANT_SETUP_RESOURCE_INVALID。
```

仓储测试断言平台初始化创建的角色和成员都写入目标租户，不能通过 payload 中的 `tenant_id` 覆盖。

- [ ] **Step 2: 运行服务测试确认 RED**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/service ./app/admin/internal/data -run 'TestPlatformTenantSetup|TestTargetTenant' -count=1
```

Expected: FAIL，Proto 请求缺少 `TargetTenantId`。

- [ ] **Step 3: 扩展 Proto 契约**

在以下请求末尾增加字段，保持已有字段编号不变：

```proto
message ListResourcesRequest {
  // existing fields 1..6
  uint64 target_tenant_id = 7;
}

message CreateResourceRequest {
  string resource = 1;
  google.protobuf.Struct data = 2;
  uint64 target_tenant_id = 3;
}

message UpdateResourceRequest {
  string resource = 1;
  uint64 id = 2;
  google.protobuf.Struct data = 3;
  uint64 target_tenant_id = 4;
}

message DeleteResourceRequest {
  string resource = 1;
  uint64 id = 2;
  uint64 target_tenant_id = 3;
}

message UpdateRoleAuthorizationRequest {
  // existing fields 1..4
  uint64 target_tenant_id = 5;
}
```

Run:

```bash
make api
cd app/frontend && pnpm generate:api
```

Expected: Go、OpenAPI 和前端类型同时生成，字段名为 `targetTenantId`。

- [ ] **Step 4: 实现平台初始化白名单和可信 Scope**

在 `management.go` 定义：

```go
var tenantSetupResources = map[string]struct{}{
	"members": {}, "departments": {}, "positions": {}, "roles": {},
	"casbin-rules": {}, "role-scope-departments": {},
}
```

将作用域解析改为：

```go
func managementScope(ctx context.Context, resource string, targetTenantID uint64) (managementbiz.Scope, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok {
		return managementbiz.Scope{}, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	platformContext := bizauth.IsPlatformContext(claims)
	if targetTenantID != 0 {
		if !platformContext {
			return managementbiz.Scope{}, kratoserrors.Forbidden("PLATFORM_ADMIN_REQUIRED", "租户初始化仅限平台管理员")
		}
		if _, allowed := tenantSetupResources[resource]; !allowed {
			return managementbiz.Scope{}, kratoserrors.BadRequest("TENANT_SETUP_RESOURCE_INVALID", "该资源不能通过租户初始化入口维护")
		}
		return managementbiz.Scope{TenantID: targetTenantID, UserID: claims.UserID, PlatformAdmin: true}, nil
	}
	return managementbiz.Scope{
		TenantID: claims.TenantID, UserID: claims.UserID, MemberID: claims.MemberID,
		PlatformAdmin: platformContext,
	}, nil
}
```

所有通用 CRUD 和角色授权调用传入请求的 `GetTargetTenantId()`。平台初始化成员时仍通过 `users` 平台资源先创建全局用户，再用 `members` + `target_tenant_id` 建立成员关系。

- [ ] **Step 5: 阻止 payload 覆盖目标租户**

仓储继续统一使用：

```go
if definition.tenantScoped {
	values[definition.tenantColumn] = scope.TenantID
}
```

移除初始化资源通过 `data["target_tenant_id"]` 决定租户的路径。`targetTenantID` 不写入资源表，也不进入 `sanitizeResourceWrite`。

- [ ] **Step 6: 扩展前端管理 API 参数**

使用统一 options：

```ts
export interface ManagementScopeOptions {
  targetTenantId?: string
}

function targetTenantParams(options?: ManagementScopeOptions) {
  return options?.targetTenantId ? { target_tenant_id: options.targetTenantId } : undefined
}
```

`listResources` 合并查询参数；`createResource`、`updateResource`、`deleteResource` 通过 Axios `params` 发送目标租户；`updateRoleAuthorization` 把生成类型中的 `targetTenantId` 放入请求体。

- [ ] **Step 7: 运行 API、服务与仓储测试确认 GREEN**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/service ./app/admin/internal/data -run 'TestPlatformTenantSetup|TestTargetTenant' -count=1
cd app/frontend && pnpm test:run src/api/management.test.ts
```

Expected: PASS。

- [ ] **Step 8: 提交本任务文件**

```bash
git add api/admin/v1/management.proto api/admin/v1/management.pb.go api/admin/v1/management_grpc.pb.go api/admin/v1/management_http.pb.go api/openapi.yaml app/frontend/src/api/generated app/admin/internal/biz/management/contracts.go app/admin/internal/service/management.go app/admin/internal/service/management_test.go app/admin/internal/data/management_repository.go app/admin/internal/data/management_repository_test.go app/frontend/src/api/management.ts app/frontend/src/api/management.test.ts
git commit -m "租户：增加平台域初始化接口"
```

---

### Task 5: 限制租户功能范围并保证权限版本即时失效

**Files:**
- Modify: `app/admin/internal/data/management_repository.go`
- Test: `app/admin/internal/data/management_repository_test.go`
- Test: `app/admin/internal/data/auth_repository_integration_test.go`

**Interfaces:**
- Consumes: Task 1 的 `scope_mask`。
- Produces: `UpdateTenantFeatures` 只接受租户适用资源，并自动补齐已选资源的祖先目录。
- Produces: 成员管理员状态、角色关系和成员状态变化递增租户权限版本。

- [ ] **Step 1: 写平台资源不能授权给租户的失败测试**

```go
err := repository.UpdateTenantFeatures(ctx, platformScope, tenant.ID, []uint64{platformOnly.ID})
if err == nil || !strings.Contains(err.Error(), "平台专属资源") {
	t.Fatalf("error = %v", err)
}
```

再创建一个租户菜单及其目录父节点，只提交菜单 ID，断言 `tenant_resources` 同时保存菜单和父目录。

- [ ] **Step 2: 写权限版本变化失败测试**

分别更新：

```go
// members.is_tenant_admin: false -> true
// members.status: 1 -> 2
// casbin-rules 的 g 成员角色关系
```

每次操作前后查询 `tenants.permission_version`，断言增加 1。

- [ ] **Step 3: 运行测试确认 RED**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/data -run 'TestUpdateTenantFeaturesRejectsPlatformResource|TestPermissionVersion' -count=1
```

Expected: FAIL，当前实现接受平台资源或未递增版本。

- [ ] **Step 4: 校验范围并补齐父资源**

`UpdateTenantFeatures` 查询资源时必须满足：

```sql
id IN (?) AND status = 1 AND deleted_at IS NULL AND (scope_mask & 2) = 2
```

若有效数量与提交数量不一致，返回 `租户功能集合包含平台专属资源或无效资源`。循环查询 `parent_id` 补齐祖先，祖先同样必须适用于租户，最终对去重后的完整集合执行替换。

- [ ] **Step 5: 扩展权限版本触发条件**

在现有 `incrementPermissionVersionGen` 路径中确保以下资源与字段触发目标租户版本递增：

```text
members: create/delete/status/is_tenant_admin
roles: create/update/delete
casbin-rules: create/update/delete
role-scope-departments: create/update/delete
tenant-resources: replace
```

平台初始化使用 `scope.TenantID` 作为目标租户，不能把版本更新到平台域 `0`。

- [ ] **Step 6: 运行仓储与令牌陈旧测试确认 GREEN**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/data ./app/admin/internal/biz/auth -run 'TestUpdateTenantFeatures|TestPermissionVersion|TestValidateAccess' -count=1
```

Expected: PASS，旧会话因权限版本不一致返回 `ErrPermissionVersionChanged`。

- [ ] **Step 7: 提交本任务文件**

```bash
git add app/admin/internal/data/management_repository.go app/admin/internal/data/management_repository_test.go app/admin/internal/data/auth_repository_integration_test.go
git commit -m "权限：限制租户资源并即时刷新版本"
```

---

### Task 6: 实现平台租户初始化页面和上下文安全的前端权限

**Files:**
- Create: `app/frontend/src/views/platform/TenantSetupView.vue`
- Create: `app/frontend/src/views/platform/TenantSetupView.test.ts`
- Modify: `app/frontend/src/views/management/ResourceListView.vue`
- Modify: `app/frontend/src/views/management/ResourceListView.test.ts`
- Modify: `app/frontend/src/views/permission/RolePermissionView.vue`
- Modify: `app/frontend/src/router/index.ts`
- Modify: `app/frontend/src/router/index.test.ts`
- Modify: `app/frontend/src/directives/permission.ts`
- Modify: `app/frontend/src/directives/permission.test.ts`
- Modify: `app/frontend/src/features/management/resourceDefinitions.ts`
- Modify: `app/frontend/src/features/navigation/registry.ts`
- Modify: `app/frontend/src/features/navigation/registry.test.ts`

**Interfaces:**
- Consumes: Task 4 的 `targetTenantId` 前端 API 参数。
- Produces: `/platform/tenants/:tenantId/setup` 平台初始化页面。
- Produces: `ResourceListView` 可选 `targetTenantId`，普通页面不传该属性。

- [ ] **Step 1: 写平台管理员在租户上下文不能绕过按钮权限的失败测试**

将旧测试替换为：

```ts
it('平台管理员在租户上下文不绕过当前权限', () => {
  expect(hasPermission([], 'api-logs:export', false)).toBe(false)
})
```

挂载指令时设置 `currentUser.platformAdmin=true`、`currentTenant.id='8'`，断言无权限按钮隐藏；设置 `currentTenant.id='0'` 后断言平台按钮可见。

- [ ] **Step 2: 写租户初始化页面失败测试**

`TenantSetupView.test.ts` 挂载路由参数 `tenantId=8`，mock 租户查询结果，断言：

```ts
expect(wrapper.text()).toContain('待初始化租户')
expect(wrapper.text()).toContain('目标租户 ID 8')
expect(wrapper.findAll('[data-testid="setup-tab"]')).toHaveLength(4)
```

四个页签为“功能授权、角色管理、用户管理、成员管理”。资源请求必须包含 `targetTenantId: '8'`。

- [ ] **Step 3: 运行前端测试确认 RED**

Run:

```bash
cd app/frontend && pnpm test:run src/directives/permission.test.ts src/views/platform/TenantSetupView.test.ts src/router/index.test.ts
```

Expected: FAIL，页面和路由不存在，权限指令仍按全局平台标记放行。

- [ ] **Step 4: 修正权限指令的有效平台上下文**

`applyPermission` 使用：

```ts
const platformContext = Boolean(
  authStore.currentUser?.platformAdmin && String(authStore.currentTenant?.id ?? '0') === '0'
)
element.hidden = !hasPermission(authStore.currentUser?.permissions, binding.value, platformContext)
```

`platformAdmin` 只保留给租户切换器中的“平台管理”入口，不再作为租户内通用操作放行条件。

- [ ] **Step 5: 让通用资源页支持明确目标租户**

扩展 props：

```ts
const props = defineProps<{ resourceKey: string; targetTenantId?: string }>()
const managementOptions = computed(() =>
  props.targetTenantId ? { targetTenantId: props.targetTenantId } : undefined
)
```

所有 list/create/update/delete 和表单租户资源 lookup 传入 `managementOptions.value`。全局用户 lookup 不传目标租户，因为它属于平台资源。

- [ ] **Step 6: 新增租户初始化页面**

页面从路由读取租户 ID，先通过平台租户列表确认目标租户存在，然后提供：

```ts
const tabs = [
  { key: 'features', label: '功能授权' },
  { key: 'roles', label: '角色管理' },
  { key: 'users', label: '用户管理' },
  { key: 'members', label: '成员管理' }
] as const
```

- “功能授权”复用 `TenantFeatureView` 并固定目标租户。
- “角色管理”复用 `RolePermissionView` 并传 `targetTenantId`。
- “用户管理”使用 `ResourceListView resource-key="users"`，创建全局账号。
- “成员管理”使用 `ResourceListView resource-key="members" :target-tenant-id="tenantId"`，关联用户、设置租户管理员并维护组织字段。

页面标题始终显示目标租户名称和 ID，返回按钮回到 `/platform/tenants`。

- [ ] **Step 7: 注册仅平台菜单可进入的动态路由**

增加路由：

```ts
{
  path: 'platform/tenants/:tenantId/setup',
  name: 'tenant-setup',
  component: () => import('@/views/platform/TenantSetupView.vue'),
  meta: { platformOnly: true }
}
```

路由守卫要求当前为平台上下文；动态初始化页面不依赖导航 routePath 精确匹配，而是要求服务端菜单包含 `tenants` 和用户处于平台上下文。

租户列表为每行增加“初始化”动作，链接到对应租户 ID。不要在通用资源目录中生成动态菜单地址。

- [ ] **Step 8: 运行 Vue 测试、类型检查和 ESLint**

Run:

```bash
cd app/frontend && pnpm test:run src/directives/permission.test.ts src/views/platform/TenantSetupView.test.ts src/views/management/ResourceListView.test.ts src/router/index.test.ts src/features/navigation/registry.test.ts
cd app/frontend && pnpm exec eslint src/directives/permission.ts src/views/platform/TenantSetupView.vue src/views/management/ResourceListView.vue src/router/index.ts
cd app/frontend && pnpm typecheck
```

Expected: 全部 PASS，无 ESLint 和 vue-tsc 错误。

- [ ] **Step 9: 提交本任务文件**

```bash
git add app/frontend/src/views/platform/TenantSetupView.vue app/frontend/src/views/platform/TenantSetupView.test.ts app/frontend/src/views/management/ResourceListView.vue app/frontend/src/views/management/ResourceListView.test.ts app/frontend/src/views/permission/RolePermissionView.vue app/frontend/src/router/index.ts app/frontend/src/router/index.test.ts app/frontend/src/directives/permission.ts app/frontend/src/directives/permission.test.ts app/frontend/src/features/management/resourceDefinitions.ts app/frontend/src/features/navigation/registry.ts app/frontend/src/features/navigation/registry.test.ts
git commit -m "前端：增加平台租户初始化入口"
```

---

### Task 7: 完成端到端权限回归与文档同步

**Files:**
- Modify: `app/frontend/src/stores/auth.test.ts`
- Modify: `app/frontend/src/layouts/AppShell.test.ts`
- Modify: `app/admin/internal/service/access_test.go`
- Modify: `docs/superpowers/specs/2026-07-21-platform-tenant-permission-isolation-design.md`（仅在实现确认设计需要修正时同步）

**Interfaces:**
- Verifies: 平台登录、租户切换、菜单刷新、权限版本失效和租户初始化完整链路。

- [ ] **Step 1: 增加跨上下文回归场景**

测试必须覆盖：

```text
平台管理员登录 -> tenant_id=0 -> 只返回平台/共用菜单
平台域创建空租户 -> 无 tenant_members
平台域初始化功能、角色、用户和成员 -> 写入目标 tenant_id
新用户登录 -> 只出现自己的租户
租户管理员登录 -> 返回租户/共用菜单，不出现平台菜单
平台管理员作为普通成员进入租户 -> 不因全局标记获得额外按钮和数据
成员权限变化 -> 旧令牌失效 -> 刷新后菜单更新
```

- [ ] **Step 2: 运行必要后端测试**

Run:

```bash
GOCACHE=/tmp/go-build go test ./app/admin/internal/biz/auth ./app/admin/internal/service ./app/admin/internal/data -count=1
```

Expected: PASS。若 MySQL 集成环境不可用，必须单独记录环境错误，不能把未执行说成通过。

- [ ] **Step 3: 运行必要前端测试**

Run:

```bash
cd app/frontend && pnpm test:run src/stores/auth.test.ts src/layouts/AppShell.test.ts src/directives/permission.test.ts src/router/index.test.ts src/features/navigation/registry.test.ts src/views/platform/TenantSetupView.test.ts src/views/management/ResourceListView.test.ts
cd app/frontend && pnpm typecheck
```

Expected: PASS。

- [ ] **Step 4: 校验生成物和差异质量**

Run:

```bash
git diff --check
GOCACHE=/tmp/go-build make gorm-gen-check
git status --short
```

Expected: `git diff --check` 和生成物检查通过；`git status` 中原有用户修改仍被保留。

- [ ] **Step 5: 提交回归测试或确认无需额外提交**

```bash
git add app/frontend/src/stores/auth.test.ts app/frontend/src/layouts/AppShell.test.ts app/admin/internal/service/access_test.go
git commit -m "测试：覆盖平台与租户权限隔离链路"
```

如果三个文件没有新增差异，则跳过提交，不创建空 commit。
