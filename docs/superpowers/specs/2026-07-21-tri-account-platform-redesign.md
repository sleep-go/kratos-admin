# 三类独立账号 + 平台菜单 + 单文件迁移 设计

## 背景

开发阶段重新规划：

1. 系统用户分为 **三类且完全独立**：平台管理员、租户管理员、App 用户
2. 平台菜单由 `resources` 树驱动（方案 C）
3. 平台 RBAC 方案 A（super admin `*:*` + 普通平台管理员绑角色）
4. **合并迁移脚本**为单一 `00001_init_schema.sql`，开发库可 drop 重建

---

## 一、三类账号模型（完全独立）

### 原则

| 原则 | 说明 |
|------|------|
| 三表三域 | 各自独立表、独立凭证、独立登录入口，**无共享 user_id** |
| 不交叉登录 | 平台管理员不能走 App 登录；App 用户不能进平台/租户 Admin |
| 代维例外 | 仅平台 super admin 可代维进入租户视角（现有机制保留） |

### 账号定义

```
┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
│ platform_admins │   │  tenant_admins  │   │    app_users    │
│  平台管理员      │   │  租户管理员      │   │   App 用户       │
├─────────────────┤   ├─────────────────┤   ├─────────────────┤
│ 管 SaaS 平台     │   │ 管单个租户后台   │   │ 使用业务 App     │
│ /platform/login │   │ /tenant/login   │   │ App API 登录     │
│ realm=platform  │   │ realm=tenant    │   │ realm=app       │
└─────────────────┘   └─────────────────┘   └─────────────────┘
        │                       │                       │
        ▼                       ▼                       ▼
   tenant_id=0            tenant_id>0              tenant_id>0
   平台菜单 /platform      租户菜单 /console         无 Admin 菜单
```

### 与现模型对比（废弃）

| 现表/概念 | 处置 |
|-----------|------|
| `users` | **删除**，拆为 `tenant_admins` + `app_users` |
| `tenant_members.user_id → users` | **删除** `tenant_members` 作为登录主体；组织关系另定（见下） |
| `tenant_members.is_tenant_admin` | **删除**，租户管理员即 `tenant_admins` 表 |
| 「全局用户」菜单 | 拆为 **App 用户**（平台代管）+ **租户管理员**（开通配置） |

### 表结构要点

#### `platform_admins`（保留并增强）

- 增加 `is_super_admin TINYINT(1) DEFAULT 0`
- 平台角色通过 Casbin `v0=0` 绑定

#### `tenant_admins`（新建）

```sql
tenant_admins (
  id, tenant_id NOT NULL,          -- 所属租户（见「待确认」是否允许多租户）
  username, email, phone,          -- 租户内唯一（uk: tenant_id + username）
  password_hash, display_name, ...
  status, mfa_*, failed_login_*, ...
  created_at, updated_at, deleted_at
)
```

- 登录：`POST /api/v1/tenant/auth/login`（或保留 `/auth/login` 但只查 `tenant_admins`）
- JWT：`realm=tenant`, `tenant_id`, **无 member_id**（或 `admin_id` 字段替代）

#### `app_users`（新建，替代 users）

```sql
app_users (
  id,
  username, email, phone,          -- 全局唯一（App 账号体系）
  password_hash, display_name, ...
  status, email_verified_at, ...
  created_at, updated_at, deleted_at
)
```

- 登录：**独立 App API**（`POST /api/v1/app/auth/login`），不在 Admin 前端
- 业务数据：`tenant_id + app_user_id` 隔离

#### 组织表（departments / positions）

**阶段 1 建议**：保留表结构，但关联改为 **App 用户侧组织**（可选）：

- `tenant_members` 重命名为 `app_user_memberships` 或保留逻辑：`tenant_id + app_user_id + department_id`
- **租户管理员不进组织表**

若 App 阶段 1 不需要部门/岗位，可 **整组组织表延后**，开通配置去掉成员/部门 Tab。

---

## 二、认证与会话

### `auth_sessions` 调整

```sql
realm ENUM('platform', 'tenant', 'app')  -- 或 VARCHAR
user_id     -- 指向对应表的 id
tenant_id   -- platform=0, tenant/app>0
member_id   -- 废弃改 admin_id，或 tenant 域用 user_id 即可
```

### 登录入口

| 类型 | 前端路径 | API |
|------|----------|-----|
| 平台管理员 | `/platform/login` | `POST /api/v1/platform/auth/login` |
| 租户管理员 | `/tenant/login` 或 `/login` | `POST /api/v1/tenant/auth/login` |
| App 用户 | （Admin 无入口） | `POST /api/v1/app/auth/login` |

### 权限

| 域 | 主体 | 授权 |
|----|------|------|
| platform | platform_admins | super admin `*:*` 或平台角色 Casbin v0=0 |
| tenant | tenant_admins | 租户角色 Casbin v0=tenant_id |
| app | app_users | App 侧 RBAC / 业务权限（后续迭代，Admin 不管菜单） |

---

## 三、平台菜单树（resources 种子，parent_id 驱动）

```
租户运营              menu.platform.tenants
  └─ 租户管理         tenants

身份与账号            menu.platform.identity
  ├─ App 用户         app-users              → /platform/app-users
  ├─ 平台管理员       platform-admins
  ├─ 平台角色         platform-roles
  └─ 按钮与 API 授权  platform-casbin-rules

系统配置              menu.platform.system
  ├─ 菜单与权限资源   resources
  └─ 渠道配置         providers

日志审计              menu.platform.logs
  └─ （四类平台日志）
```

**开通配置**（`/platform/tenants/:id/setup`）Tab：

1. 功能授权（tenant-resources API）
2. 租户角色（roles, tenant_id=目标租户）
3. **租户管理员**（tenant_admins CRUD，不再创建 users/members）
4. ~~全局用户~~ / ~~成员管理~~ 移除

**隐藏 visible=0**：`tenant-setup`, `tenant-resources`, 旧 `menu.platform`

---

## 四、迁移策略（开发阶段）

### 做法

1. **删除** `migrations/00008`～`00012` 及所有增量文件
2. **重写** `migrations/00001_init_schema.sql` 为终态：
   - 三账号表 + 调整后的 sessions
   - 最终 resources 菜单树
   - `platform_admins.is_super_admin`
   - 种子：super admin 账号、默认平台菜单
3. 开发环境：`DROP DATABASE` → `goose up`
4. **不保留**旧 `users` / `tenant_members` 数据迁移（开发阶段）

### 代码同步（实现阶段）

- GORM gen 重新生成
- 删除 `users` / `UserRepository` 租户登录路径，新增 `tenant_admins` / `app_users` 仓储
- Admin 前端：`/login` 改为租户管理员登录；新增 `/platform/app-users`
- 管理 API：`members` 资源或移除或改为 `app-users` / `tenant-admins`

---

## 六、已确认

| 项 | 决策 |
|----|------|
| 账号分类 | 平台管理员 / 租户管理员 / App 用户，**完全独立** |
| 废弃 users 统一身份 | 是 |
| 迁移 | 单文件 00001，开发库可重建 |
| 平台菜单 | DB 树驱动，4 一级目录 |
| 平台 RBAC | 方案 A（super admin + 角色） |
| 租户管理员范围 | **A：一个账号绑定一个 tenant_id** |
| App 用户创建 | **C：App 自助注册 + Admin 可管理/禁用** |
| 组织（部门/岗位） | **A：阶段 1 不做**，不建或暂不使用相关表 |

---

## 七、阶段 1 范围摘要

### 数据库（单文件 00001）

**保留/新建**

- `platform_admins`（+ `is_super_admin`）
- `tenant_admins`（`tenant_id` NOT NULL，租户内唯一 username）
- `app_users`（全局唯一 username/email/phone）
- `tenants`, `roles`, `resources`, `casbin_rules`, `tenant_resources`, …
- `auth_sessions`（realm: platform / tenant / app）

**阶段 1 移除或暂不启用**

- ~~`users`~~
- ~~`tenant_members`~~、~~`member_departments`~~
- ~~`departments`~~、~~`positions`~~（组织整组延后）

### API

- `POST /api/v1/app/auth/register` — App 自助注册
- `POST /api/v1/app/auth/login`
- `POST /api/v1/tenant/auth/login` — 租户管理员（原 `/auth/login` 语义变更）
- 平台侧 `app-users` CRUD（列表、禁用、重置状态，不含代登录）

### 开通配置 Tab

1. 功能授权
2. 租户角色
3. 租户管理员（创建 `tenant_admins`，不再 users/members）

### Admin 前端登录页

- `/platform/login` → 平台管理员
- `/tenant/login`（或保留 `/login`）→ 租户管理员
- App 无 Admin 页面
