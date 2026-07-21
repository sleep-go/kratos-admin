# SaaS 平台/租户分离设计

## 背景

Kratos Admin 从单租户管理后台演进为 SaaS 多租户底座，需要在同一前端应用中隔离**平台治理**与**租户运营**两套身份上下文，并支持平台管理员代维进入租户视角。

## 核心模型

### 认证域（Realm）

| Realm | 说明 | tenant_id | 登录入口 |
|-------|------|-----------|----------|
| `platform` | 平台管理员 | `0` | `/platform/login` → `POST /api/v1/platform/auth/login` |
| `tenant` | 租户成员 | 目标租户 ID | `/login` → `POST /api/v1/auth/login` |

JWT / Refresh Token 携带 `realm`、`impersonator_id`（代维时非 0）。

### 数据表

- `platform_admins`：平台管理员独立账号，与 `users` 分离
- `auth_sessions.realm` / `impersonator_id`：会话域与代维者
- `audit_logs.impersonator_id`：审计记录代维操作者
- `resources.scope_mask`：菜单可见性（1=平台，2=租户）

## 路由划分

| 区域 | 前缀 | Shell | 说明 |
|------|------|-------|------|
| 平台治理 | `/platform/**` | `PlatformShell` | 租户、全局用户、平台管理员、菜单资源等 |
| 租户控制台 | `/console/**` | `ConsoleShell` | 组织、权限、日志、文件、设置等 |

导航种子数据中的 `route_path` 与前端 `componentRouteRegistry` 一一对应，守卫仅允许访问服务端下发的菜单路由。

## 代维（Impersonation）

1. 平台管理员在 `/platform/tenants` 点击「进入租户」
2. 调用 `POST /api/v1/platform/auth/impersonate`
3. 签发 `realm=tenant` + `impersonator_id=平台管理员ID` 的短 TTL 令牌
4. 前端跳转 `/console`，顶栏展示代维横幅与「退出代维」
5. 退出代维撤销租户会话，返回平台登录页

## 安全约束

- 租户用户无法登录平台端，也无法通过 `switch-tenant` 切换到 `tenant_id=0`
- 平台路由要求 `realm=platform && tenant_id=0 && !impersonating`
- Refresh Cookie 路径统一为 `/api/v1`，平台与租户 refresh 接口均可读取

## 关键文件

- 迁移：`migrations/00001_init_schema.sql`
- 平台认证：`app/admin/internal/biz/auth/platform_login.go`
- 代维：`app/admin/internal/biz/auth/impersonate.go`
- 平台 API：`api/admin/v1/platform_auth.proto`
- 前端路由：`app/frontend/src/router/index.ts`
- 导航映射：`app/frontend/src/features/navigation/registry.ts`
