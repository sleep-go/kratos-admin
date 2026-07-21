# 平台菜单树与平台 RBAC 设计

## 背景

平台侧导航长期存在以下问题：

1. 前端 `resolveTopNavigation` 硬编码「平台治理 / 平台日志」分组，与 `resources` 表不一致
2. `resolveNavigation` 丢弃 `type=1` 目录节点，无法还原 DB 菜单树
3. 菜单项职责混乱（如 `providers` 挂在 `menu.settings` 却出现在平台顶栏）
4. 与「开通配置」功能重复的历史入口已隐藏，但整树仍未重新规划
5. 平台管理员全员 `*:*`，缺少平台域角色体系

## 目标

1. **菜单单一数据源**：平台顶栏完全由 `resources.parent_id` 树驱动（方案 C）
2. **结构清晰**：4 个一级目录，职责边界明确
3. **平台 RBAC（方案 A）**：bootstrap 超级管理员保留 `*:*`；其余平台管理员必须绑定平台角色
4. **开通配置不入菜单**：仍从「租户管理 → 开通配置」进入

## 非目标

- 租户控制台菜单重组（租户侧仅展示 `scope_mask & 2` 的业务功能，另行配置）
- 参数字典 / 系统设置等平台页面的路由统一（若尚无 `/platform` 路由则不在本次菜单中展示）
- 代维会话权限模型变更（代维逻辑保持现状）

---

## 一、平台菜单树（`resources` 迁移目标）

所有节点 `scope_mask = 1`（仅平台），`parent_id = 0` 的为顶栏一级目录。

```
租户运营              menu.platform.tenants      sort=10  visible=1  type=1
  └─ 租户管理         tenants                    sort=11  visible=1  type=2  → /platform/tenants

身份与账号            menu.platform.identity     sort=20  visible=1  type=1
  ├─ 全局用户         users                      sort=21  visible=1  type=2  → /platform/users
  ├─ 平台管理员       platform-admins            sort=22  visible=1  type=2  → /platform/admins
  ├─ 平台角色         platform-roles             sort=23  visible=1  type=2  → /platform/permission/roles
  └─ 按钮与 API 授权  platform-casbin-rules      sort=24  visible=1  type=2  → /platform/permission/policies

系统配置              menu.platform.system       sort=30  visible=1  type=1
  ├─ 菜单与权限资源   resources                  sort=31  visible=1  type=2  → /platform/resources
  └─ 渠道配置         providers                  sort=32  visible=1  type=2  → /platform/settings/providers

日志审计              menu.platform.logs         sort=40  visible=1  type=1
  ├─ 登录日志         platform-login-logs        sort=41  visible=1  type=2  → /platform/logs/login
  ├─ 操作审计         platform-audit-logs        sort=42  visible=1  type=2  → /platform/logs/audit
  ├─ API 日志         platform-api-logs            sort=43  visible=1  type=2  → /platform/logs/api
  └─ 日志导出         platform-log-exports       sort=44  visible=1  type=2  → /platform/logs/exports
```

### 隐藏节点（`visible = 0`，不占导航）

| code | 用途 |
|------|------|
| `menu.platform` | 旧根目录，迁移后隐藏 |
| `menu.platform-logs` | 合并进 `menu.platform.logs` 后删除或隐藏 |
| `tenant-setup` | 开通配置 API / 页面权限，从租户列表进入 |
| `tenant-resources` | 功能授权 API，在开通配置 Tab 内使用 |

### 迁移动作摘要

- 新建 4 个一级目录（若已存在则 UPDATE）
- 将现有平台菜单项 `parent_id` 挂到对应目录下
- `providers` 从 `menu.settings` 迁到 `menu.platform.system`
- `resources` 从 `menu.platform` 直下迁到 `menu.platform.system`
- 合并 `menu.platform-logs` → `menu.platform.logs`
- 新增 `platform-roles`、`platform-casbin-rules` 菜单资源
- 旧 `menu.platform` 设 `visible=0`

---

## 二、前端导航渲染（DB 树驱动）

### 数据流

```
GET /platform/auth/navigation  →  flat NavigationItem[]（含 parent_id、type）
        ↓
buildNavigationTree(items)     →  递归组树，保留 type=1 目录
        ↓
attachRoutes(tree)             →  仅 type=2 叶子校验 componentRouteRegistry
        ↓
TopNavigation                  →  一级目录 = 顶栏下拉；二级 = 子菜单链接
```

### 代码变更

| 文件 | 变更 |
|------|------|
| `features/navigation/registry.ts` | 新增 `buildNavigationTree`；删除 `resolveTopNavigation` 硬编码分组；扩展 `componentRouteRegistry` |
| `layouts/PlatformShell.vue` | 使用树形导航，不再调用 platform 专用分组 |
| `layouts/ConsoleShell.vue` | 同样改为 DB 树（租户侧） |
| `router/index.ts` | 新增 `/platform/permission/roles`、`/platform/permission/policies` |

### 路由注册表新增

```typescript
'platform-roles': '/platform/permission/roles',
'platform-casbin-rules': '/platform/permission/policies',
```

### 顶栏展示规则（C1：两层）

- **顶栏项**：`parent_id = 0` 且 `visible = 1` 的目录（type=1）
- **下拉项**：该目录下 `visible = 1` 的菜单（type=2）；若子节点仍为目录则展平一层（目录不可点击，仅作分组标题）或合并为带前缀的标签——**默认：仅允许「目录 → 菜单」两层，迁移保证结构**

---

## 三、平台 RBAC（方案 A）

### 模型

| 概念 | 存储 | 说明 |
|------|------|------|
| 平台域 | Casbin `v0 = "0"` | 与租户域 `v0 = tenant_id` 隔离 |
| 平台角色 | `roles.tenant_id = 0` |  schema 已支持 |
| 平台策略 | `casbin_rules` ptype=p, v0=0 | 资源 code 限 `scope_mask & 1` |
| 管理员绑角色 | `casbin_rules` ptype=g, v0=0, v1=platform_admin_id, v2=role_id | 复用 Casbin，不新增表 |
| 超级管理员 | `platform_admins.is_super = 1` 或等价标记 | 始终 `*:*`，绕过 Casbin |

### 超级管理员判定

**推荐**：`platform_admins` 增加 `is_super_admin TINYINT(1) DEFAULT 0`。

- 种子数据 / 迁移：将首个管理员或现有全部管理员初始设为 `is_super_admin=1`（保证升级不锁死）
- 新建普通平台管理员默认 `is_super_admin=0`，必须分配角色

**备选**（不新增字段）：约定 `id=1` 为 bootstrap——不推荐，隐式规则难维护。

### 权限加载（`ListPermissions`）

```
realm=platform && tenant_id=0:
  if admin.is_super_admin → ["*:*"]
  else → Casbin g + p，资源限 scope_mask&1，返回 code:action 列表
```

### 权限校验（`Allowed`）

```
scope.PlatformAdmin && scope.TenantID==0:
  if super admin → true
  else → Casbin 查 v0=0, v1=admin_id（注意：平台上下文 MemberID 为 0，需用 UserID 作主体）
```

**注意**：当前 `managementScope` 中 `PlatformAdmin = (realm==platform)`，需在 Scope 或 Claims 中区分 super admin 与普通平台管理员。

### 平台角色管理页

- 路由：`/platform/permission/roles`
- 组件：复用 `RolePermissionView`，平台模式不传 `targetTenantId`，后端 `tenant_id=0`
- 可授权资源集合：所有 `scope_mask & 1` 的平台资源（不含 hidden API 资源若需要可单独过滤）

### 平台管理员页增强

- 在 `platform-admins` 编辑表单中增加「角色」多选
- 保存时写入 Casbin `g` 规则（v0=0, v1=admin_id, v2=role_id）
- 超级管理员不展示角色绑定或展示为「超级管理员（全部权限）」

### 数据范围

平台角色 **不使用** 租户 `data_scope` / 部门范围；平台数据默认为全平台可见。`roles.data_scope` 对 `tenant_id=0` 固定为「全部」或忽略。

---

## 四、后端 API 变更摘要

| 区域 | 变更 |
|------|------|
| `auth_repository.ListPermissions` | 平台非 super admin 走 Casbin |
| `management_repository.Allowed` | 平台非 super admin 走 Casbin（v0=0） |
| `managementScope` / List roles | 平台上下文允许 `roles`/`casbin-rules` 且 `tenant_id=0` |
| `ListNavigation` | 非 super admin 按 Casbin 过滤可见菜单（与租户侧一致） |
| `platform_admins` CRUD | 支持角色绑定；禁止删除最后一个 super admin |

---

## 五、分阶段交付

### 阶段 1：菜单树驱动（可独立上线）

- [ ] migration 重组 `resources` 树
- [ ] 前端 `buildNavigationTree`，移除硬编码分组
- [ ] 路由注册表补全
- [ ] `/platform/tenant-features` 保持重定向

### 阶段 2：平台 RBAC

- [ ] `platform_admins.is_super_admin` 迁移
- [ ] 平台角色 / 策略菜单与路由
- [ ] `RolePermissionView` 平台模式
- [ ] `ListPermissions` / `Allowed` / `ListNavigation` 收紧
- [ ] 平台管理员角色绑定 UI
- [ ] 集成测试：普通平台管理员无角色时无法访问菜单

---

## 六、测试要点

1. 超级管理员可见 4 个顶栏目录及全部子菜单
2. 普通平台管理员仅可见其角色授权的子菜单
3. 修改 `resources.sort_order` 后导航顺序变化（验证 DB 驱动）
4. `providers` 出现在「系统配置」下，不再受 `menu.settings` 影响
5. 开通配置仍从租户列表进入，导航无入口
6. 代维与普通租户登录行为不受影响

---

## 七、已确认决策

| 项 | 决策 |
|----|------|
| 菜单组织 | C：完全 DB 树驱动 |
| 顶栏层级 | C1：两层（目录 → 菜单） |
| 菜单与权限资源 | 归入「系统配置」 |
| 平台角色 | 需要，放在「身份与账号」 |
| 平台权限模型 | A：super admin `*:*` + 普通管理员绑角色 |
| 开通配置 | 不入平台顶栏菜单 |
