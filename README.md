# Kratos Admin

Kratos Admin 是基于 go-kratos 与 Vue 3 的前后端分离、多租户通用管理后台。

## 目标能力

- 原生多租户与平台治理
- 用户、部门、岗位和租户成员管理
- Casbin domain RBAC、动态菜单及仓储强制数据范围
- 登录、操作审计和 API 访问日志
- 参数字典、系统设置及可插拔邮件/短信渠道
- 本地文件与阿里云 OSS 对象存储
- API 与 Worker 分进程运行

## 技术栈

- 后端：Go 1.26、go-kratos、GORM Gen、Goose、Casbin、Redis、Asynq
- 前端：Vue 3、TypeScript、Element Plus、Tailwind CSS、SCSS、Pinia
- 数据库：PolarDB MySQL 8（本地开发使用 MySQL 8）

详细设计和实施计划见 `docs/superpowers/`。

## 配置与 Provider

- 系统设置按代码安全默认、平台默认、租户允许覆盖值三级解析。
- 敏感设置及 Provider JSON 使用 AES-256-GCM 加密，读取接口仅返回配置状态。
- 内置本地邮件/短信模拟器、SMTP、阿里云短信、本地文件及阿里云 OSS。
- SMTP、短信与 OSS 连接测试使用已保存密文配置；阿里云短信只查询已配置签名，不发送付费短信。

五类数据范围支持租户内全部、本部门及下级、本部门、仅本人和自定义部门。多角色取并集，但成员、部门、文件和日志查询始终先强制认证上下文中的 `tenant_id`，前端提交的租户或数据范围参数不会被信任。
