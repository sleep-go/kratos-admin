# Kratos Admin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可运行、可测试、可容器化部署的多租户通用管理后台。

**Architecture:** 后端为模块化单体，API 与 Worker 分进程；前端为 Vue 3 SPA。Protobuf 定义契约，PolarDB MySQL 8 保存业务真相，Redis 支撑缓存、验证码、限流和异步任务。

**Tech Stack:** Go 1.26、go-kratos、GORM Gen、Goose、Casbin、Asynq、Vue 3、TypeScript、Element Plus、Tailwind CSS、SCSS、Pinia、Vitest、Playwright、Docker Compose。

## Global Constraints

- Go module 固定为 `github.com/sleep-go/kratos-admin`。
- 所有导出 Go 标识包含以名称开头的中文 GoDoc。
- Goose 是唯一迁移入口，服务不得执行 AutoMigrate。
- 所有租户数据查询强制包含服务端认证上下文中的 tenant_id。
- 业务代码先写失败测试，再编写最小实现。
- 前端采用顶部导航、全响应式布局和已确认的灰红配色。

## Tasks

- [x] Task 1：初始化单仓、Kratos API/Worker、Protobuf、配置、MySQL、Redis、代码生成和健康检查。
- [x] Task 2：通过 Goose 建立身份、租户、组织、权限、日志、配置、任务和文件数据结构。
- [x] Task 3：实现密码、验证码、MFA、JWT 双令牌、会话撤销和租户切换。
- [x] Task 4：实现租户、成员、部门、岗位管理和平台治理接口。
- [x] Task 5：实现 Casbin domain RBAC、菜单资源、租户功能授权和五类数据范围。
- [x] Task 6：实现 outbox、Asynq Worker、三类日志、保留策略和异步导出。
- [x] Task 7：实现三级配置、字典、邮件/短信 Provider 和敏感配置加密。
- [ ] Task 8：实现本地/OSS 文件上传、校验、下载、引用与异步清理。
- [ ] Task 9：实现 Vue 3 登录、工作台、平台管理、组织、权限、日志、文件、设置和个人中心页面。
- [ ] Task 10：完善响应式适配、Vitest、Playwright、Docker Compose、初始化命令和运维文档。
- [ ] Task 11：执行生成一致性、迁移、单元、集成、race、vet、lint、E2E 和镜像构建验证。

## Acceptance

- 空环境可按文档启动 API、Worker 和前端。
- 后端拒绝全部跨租户访问与未授权 API 调用。
- 令牌轮换、MFA、菜单、数据范围、审计、异步任务和文件权限符合设计。
- 所有页面使用真实 API，不以 Mock 数据作为最终交付。
- 核心测试、静态检查和容器构建通过。
