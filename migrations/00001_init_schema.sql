-- +goose Up
CREATE TABLE tenants (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '租户主键',
    code VARCHAR(64) NOT NULL COMMENT '租户唯一编码',
    name VARCHAR(128) NOT NULL COMMENT '租户名称',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '租户状态：1启用，2冻结',
    permission_version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '权限版本号',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创建用户ID，0表示系统初始化',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenants_code (code),
    KEY idx_tenants_status (status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户表';

CREATE TABLE platform_admins (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '平台管理员主键',
    username VARCHAR(64) NOT NULL COMMENT '平台管理员唯一用户名',
    email VARCHAR(191) NULL COMMENT '平台管理员邮箱',
    phone VARCHAR(32) NULL COMMENT '平台管理员手机号',
    password_hash VARCHAR(255) NOT NULL COMMENT 'Argon2id密码哈希',
    display_name VARCHAR(128) NOT NULL COMMENT '显示名称',
    avatar_url TEXT NULL COMMENT '头像地址',
    is_super_admin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否超级管理员：0否，1是（拥有*:*）',
    mfa_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用登录MFA：0否，1是',
    mfa_channel VARCHAR(16) NOT NULL DEFAULT 'email' COMMENT 'MFA渠道：email邮件，sms短信',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '账号状态：1启用，2禁用，3锁定',
    failed_login_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '连续登录失败次数',
    locked_until DATETIME(3) NULL COMMENT '锁定截止时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_platform_admins_username (username),
    UNIQUE KEY uk_platform_admins_email (email),
    UNIQUE KEY uk_platform_admins_phone (phone),
    KEY idx_platform_admins_status (status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='平台管理员表';

CREATE TABLE tenant_admins (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '租户管理员主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    username VARCHAR(64) NOT NULL COMMENT '租户内唯一用户名',
    email VARCHAR(191) NULL COMMENT '租户内邮箱',
    phone VARCHAR(32) NULL COMMENT '租户内手机号',
    password_hash VARCHAR(255) NOT NULL COMMENT 'Argon2id密码哈希',
    display_name VARCHAR(128) NOT NULL COMMENT '显示名称',
    avatar_url TEXT NULL COMMENT '头像地址',
    mfa_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用登录MFA：0否，1是',
    mfa_channel VARCHAR(16) NOT NULL DEFAULT 'email' COMMENT 'MFA渠道：email邮件，sms短信',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '账号状态：1启用，2禁用，3锁定',
    failed_login_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '连续登录失败次数',
    locked_until DATETIME(3) NULL COMMENT '锁定截止时间',
    password_changed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '密码最后修改时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_admins_tenant_username (tenant_id, username),
    KEY idx_tenant_admins_tenant_status (tenant_id, status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户管理员表';

CREATE TABLE app_users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'App用户主键',
    username VARCHAR(64) NOT NULL COMMENT '全局唯一用户名',
    email VARCHAR(191) NULL COMMENT '全局唯一邮箱',
    phone VARCHAR(32) NULL COMMENT '全局唯一手机号',
    password_hash VARCHAR(255) NOT NULL COMMENT 'Argon2id密码哈希',
    display_name VARCHAR(128) NOT NULL COMMENT '用户显示名称',
    avatar_url TEXT NULL COMMENT '头像地址',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '用户状态：1启用，2禁用，3锁定',
    failed_login_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '连续登录失败次数',
    locked_until DATETIME(3) NULL COMMENT '锁定截止时间',
    email_verified_at DATETIME(3) NULL COMMENT '邮箱验证时间',
    phone_verified_at DATETIME(3) NULL COMMENT '手机号验证时间',
    mfa_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用登录MFA：0否，1是',
    mfa_channel VARCHAR(16) NOT NULL DEFAULT 'email' COMMENT 'MFA渠道：email邮件，sms短信',
    password_changed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '密码最后修改时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_app_users_username (username),
    UNIQUE KEY uk_app_users_email (email),
    UNIQUE KEY uk_app_users_phone (phone),
    KEY idx_app_users_status (status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='App用户表';

CREATE TABLE roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID，0表示平台角色',
    code VARCHAR(64) NOT NULL COMMENT '作用域内唯一角色编码',
    name VARCHAR(128) NOT NULL COMMENT '角色名称',
    data_scope TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '数据范围：1全部，2本部门及下级，3本部门，4仅本人，5自定义部门',
    is_builtin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否系统内置：0否，1是',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '角色状态：1启用，2禁用',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_roles_tenant_code (tenant_id, code),
    KEY idx_roles_tenant_status (tenant_id, status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色表';

CREATE TABLE resources (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '权限资源主键',
    parent_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '父资源ID，0表示根资源',
    type TINYINT UNSIGNED NOT NULL COMMENT '资源类型：1目录，2菜单，3按钮，4API',
    scope_mask TINYINT UNSIGNED NOT NULL DEFAULT 3 COMMENT '资源适用范围位标记：1仅平台，2仅租户，3平台与租户共用',
    code VARCHAR(128) NOT NULL COMMENT '全局唯一资源编码',
    name VARCHAR(128) NOT NULL COMMENT '资源名称',
    route_path VARCHAR(255) NOT NULL DEFAULT '' COMMENT '前端路由路径',
    component_key VARCHAR(128) NOT NULL DEFAULT '' COMMENT '前端预注册组件键',
    http_method VARCHAR(16) NOT NULL DEFAULT '' COMMENT 'API资源HTTP方法',
    api_path VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'API资源路由模板',
    icon VARCHAR(64) NOT NULL DEFAULT '' COMMENT '前端图标名称',
    sort_order INT NOT NULL DEFAULT 0 COMMENT '显示排序值',
    visible TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否显示：0隐藏，1显示',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '资源状态：1启用，2禁用',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_resources_code (code),
    KEY idx_resources_parent (parent_id, type, status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='菜单与权限资源表';

CREATE TABLE tenant_resources (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '租户资源授权主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    resource_id BIGINT UNSIGNED NOT NULL COMMENT '权限资源ID',
    created_by BIGINT UNSIGNED NOT NULL COMMENT '授权用户ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '授权时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_resources_relation (tenant_id, resource_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户功能授权表';

CREATE TABLE casbin_rules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Casbin策略主键',
    ptype VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '策略类型：p资源策略，g角色继承策略',
    v0 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值0：租户域ID',
    v1 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值1：角色或主体ID',
    v2 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值2：资源编码或角色ID',
    v3 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值3：资源动作',
    v4 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值4：预留',
    v5 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值5：预留',
    PRIMARY KEY (id),
    UNIQUE KEY uk_casbin_rules_policy (ptype, v0, v1, v2, v3, v4, v5),
    KEY idx_casbin_rules_domain (v0, ptype)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Casbin权限策略表';

CREATE TABLE auth_sessions (
    id CHAR(36) NOT NULL COMMENT '会话UUID',
    realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '认证域：platform平台，tenant租户，app应用',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '主体ID：platform=platform_admin.id，tenant=tenant_admin.id，app=app_user.id',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '当前租户ID，平台域为0',
    member_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '兼容字段，阶段1固定为0',
    impersonator_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '代维平台管理员ID，非代维为0',
    permission_version BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '会话最近一次签发时的权限版本号',
    refresh_jti_hash CHAR(64) NOT NULL COMMENT 'Refresh JWT jti的SHA256摘要',
    device_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备名称',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT '登录User-Agent',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '登录IP地址',
    expires_at DATETIME(3) NOT NULL COMMENT '会话过期时间',
    revoked_at DATETIME(3) NULL COMMENT '会话撤销时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_auth_sessions_refresh_hash (refresh_jti_hash),
    KEY idx_auth_sessions_user (realm, user_id, revoked_at, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='认证会话表';

CREATE TABLE verification_codes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '验证码主键',
    user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联用户ID，未知用户为0',
    target VARCHAR(191) NOT NULL COMMENT '邮箱或手机号',
    scene VARCHAR(32) NOT NULL COMMENT '验证码场景：mfa登录，password_reset重置密码',
    channel VARCHAR(16) NOT NULL COMMENT '发送渠道：email邮件，sms短信',
    code_hash CHAR(64) NOT NULL COMMENT '验证码SHA256摘要',
    attempt_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已验证失败次数',
    expires_at DATETIME(3) NOT NULL COMMENT '过期时间',
    consumed_at DATETIME(3) NULL COMMENT '消费时间',
    context_data JSON NULL COMMENT 'MFA设备上下文等非敏感挑战数据',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_verification_codes_lookup (target, scene, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邮件短信验证码表';

CREATE TABLE login_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '登录日志主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '登录租户ID，0表示未选择或平台域',
    realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '域：platform平台，tenant租户，app应用',
    user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID，未识别用户为0',
    identifier VARCHAR(191) NOT NULL COMMENT '脱敏后的登录标识',
    result TINYINT UNSIGNED NOT NULL COMMENT '登录结果：1成功，2失败，3锁定，4需要MFA',
    reason VARCHAR(128) NOT NULL DEFAULT '' COMMENT '登录结果原因',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '客户端IP',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT '客户端User-Agent',
    request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求追踪ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '发生时间',
    PRIMARY KEY (id),
    KEY idx_login_logs_realm_tenant (realm, tenant_id),
    KEY idx_login_logs_query (tenant_id, created_at),
    KEY idx_login_logs_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='登录安全日志表';

CREATE TABLE audit_outbox (
    id CHAR(36) NOT NULL COMMENT 'Outbox事件UUID',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台域',
    event_type VARCHAR(64) NOT NULL COMMENT '审计事件类型',
    aggregate_type VARCHAR(64) NOT NULL COMMENT '业务聚合类型',
    aggregate_id VARCHAR(64) NOT NULL COMMENT '业务聚合ID',
    payload JSON NOT NULL COMMENT '审计事件载荷',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理，2已完成，3等待重试，4最终失败',
    retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '重试次数',
    next_retry_at DATETIME(3) NULL COMMENT '下次重试时间',
    dispatched_at DATETIME(3) NULL COMMENT '最近一次RabbitMQ确认投递时间',
    last_error VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '最后一次处理失败原因',
    published_at DATETIME(3) NULL COMMENT '成功发布时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_audit_outbox_dispatch (status, next_retry_at, dispatched_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='事务审计Outbox表';

CREATE TABLE audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '操作审计日志主键',
    event_id CHAR(36) NOT NULL COMMENT '来源Outbox事件UUID',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台域',
    realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '域：platform平台，tenant租户，app应用',
    user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作用户ID，系统任务为0',
    member_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '兼容字段，阶段1固定为0',
    impersonator_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '代维平台管理员ID，非代维为0',
    action VARCHAR(64) NOT NULL COMMENT '业务动作',
    resource_type VARCHAR(64) NOT NULL COMMENT '资源类型',
    resource_id VARCHAR(64) NOT NULL COMMENT '资源ID',
    summary VARCHAR(512) NOT NULL COMMENT '中文操作摘要',
    before_data JSON NULL COMMENT '变更前脱敏数据',
    after_data JSON NULL COMMENT '变更后脱敏数据',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '客户端IP',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT '客户端User-Agent',
    request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求追踪ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '发生时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_audit_logs_event (event_id),
    KEY idx_audit_logs_realm_tenant (realm, tenant_id),
    KEY idx_audit_logs_query (tenant_id, created_at),
    KEY idx_audit_logs_actor (tenant_id, user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='操作审计日志表';

CREATE TABLE api_access_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'API访问日志主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示未认证或平台域',
    realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '域：platform平台，tenant租户，app应用',
    user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID，未认证为0',
    request_id VARCHAR(64) NOT NULL COMMENT '请求追踪ID',
    method VARCHAR(16) NOT NULL COMMENT 'HTTP方法',
    route VARCHAR(255) NOT NULL COMMENT 'HTTP路由模板',
    status_code SMALLINT UNSIGNED NOT NULL COMMENT 'HTTP响应状态码',
    duration_ms INT UNSIGNED NOT NULL COMMENT '请求耗时毫秒',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '客户端IP',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT '客户端User-Agent',
    request_data JSON NULL COMMENT '白名单脱敏请求数据',
    error_reason VARCHAR(128) NOT NULL DEFAULT '' COMMENT '业务错误原因',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '发生时间',
    PRIMARY KEY (id),
    KEY idx_api_access_logs_realm_tenant (realm, tenant_id),
    KEY idx_api_access_logs_query (tenant_id, created_at),
    KEY idx_api_access_logs_request (request_id),
    KEY idx_api_access_logs_error (tenant_id, status_code, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='API访问与异常日志表';

CREATE TABLE system_settings (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '系统配置主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '配置租户ID，0表示平台默认',
    category VARCHAR(64) NOT NULL COMMENT '配置分类',
    setting_key VARCHAR(128) NOT NULL COMMENT '配置键',
    value_type VARCHAR(16) NOT NULL COMMENT '值类型：string字符串，number数字，boolean布尔，json对象',
    setting_value JSON NOT NULL COMMENT '配置值',
    allow_tenant_override TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否允许租户覆盖：0否，1是',
    is_secret TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否敏感配置：0否，1是',
    version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '配置版本号',
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最后更新用户ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_system_settings_scope_key (tenant_id, category, setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='平台与租户系统配置表';

CREATE TABLE dictionary_types (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '字典类型主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台字典',
    code VARCHAR(64) NOT NULL COMMENT '字典类型编码',
    name VARCHAR(128) NOT NULL COMMENT '字典类型名称',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '字典状态：1启用，2禁用',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_dictionary_types_scope_code (tenant_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='参数字典类型表';

CREATE TABLE dictionary_items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '字典项主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台字典',
    type_id BIGINT UNSIGNED NOT NULL COMMENT '字典类型ID',
    item_value VARCHAR(191) NOT NULL COMMENT '字典项值',
    label VARCHAR(128) NOT NULL COMMENT '字典项显示名称',
    sort_order INT NOT NULL DEFAULT 0 COMMENT '显示排序值',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '字典项状态：1启用，2禁用',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_dictionary_items_value (tenant_id, type_id, item_value),
    KEY idx_dictionary_items_type (tenant_id, type_id, status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='参数字典项表';

CREATE TABLE provider_configs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Provider配置主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '配置租户ID，0表示平台默认',
    provider_type VARCHAR(32) NOT NULL COMMENT 'Provider类型：email邮件，sms短信，storage对象存储',
    provider_name VARCHAR(64) NOT NULL COMMENT 'Provider实现名称',
    display_name VARCHAR(128) NOT NULL COMMENT '配置显示名称',
    encrypted_config TEXT NOT NULL COMMENT '主密钥加密后的Provider配置',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '配置状态：1启用，2禁用',
    is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否作用域默认：0否，1是',
    updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最后更新用户ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_provider_configs_scope_name (tenant_id, provider_type, provider_name),
    KEY idx_provider_configs_default (tenant_id, provider_type, is_default, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邮件短信与存储Provider配置表';

CREATE TABLE files (
    id CHAR(36) NOT NULL COMMENT '文件UUID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    uploader_id BIGINT UNSIGNED NOT NULL COMMENT '上传者ID（租户管理员或App用户）',
    provider_name VARCHAR(64) NOT NULL COMMENT '存储Provider名称',
    object_key VARCHAR(512) NOT NULL COMMENT '对象存储键',
    original_name VARCHAR(255) NOT NULL COMMENT '原始文件名',
    content_type VARCHAR(128) NOT NULL COMMENT '文件MIME类型',
    size_bytes BIGINT UNSIGNED NOT NULL COMMENT '文件大小字节数',
    sha256 CHAR(64) NOT NULL DEFAULT '' COMMENT '文件SHA256摘要',
    etag VARCHAR(191) NOT NULL DEFAULT '' COMMENT '对象存储返回的ETag',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '文件状态：1待确认，2可用，3已删除，4清理失败，5等待后台清理',
    cleanup_dispatched_at DATETIME(3) NULL COMMENT '文件清理任务最近一次RabbitMQ确认投递时间',
    cleanup_retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件清理失败重试次数',
    cleanup_next_retry_at DATETIME(3) NULL COMMENT '文件清理下次重试时间',
    cleanup_failure_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '文件清理最后失败原因',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_files_provider_object (provider_name, object_key),
    KEY idx_files_tenant (tenant_id, status, created_at),
    KEY idx_files_cleanup (status, cleanup_next_retry_at, cleanup_dispatched_at, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户文件元数据表';

CREATE TABLE file_references (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '文件引用主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    file_id CHAR(36) NOT NULL COMMENT '文件UUID',
    business_type VARCHAR(64) NOT NULL COMMENT '引用业务类型',
    business_id VARCHAR(64) NOT NULL COMMENT '引用业务ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_file_references_relation (tenant_id, file_id, business_type, business_id),
    KEY idx_file_references_business (tenant_id, business_type, business_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文件业务引用表';

CREATE TABLE log_exports (
    id CHAR(36) NOT NULL COMMENT '日志导出任务UUID',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台跨租户导出',
    realm VARCHAR(16) NOT NULL DEFAULT 'tenant' COMMENT '域：platform平台，tenant租户，app应用',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '发起导出的用户ID',
    member_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '兼容字段，阶段1固定为0',
    log_type VARCHAR(16) NOT NULL COMMENT '日志类型：login登录日志，audit操作审计，api接口访问日志',
    keyword VARCHAR(191) NOT NULL DEFAULT '' COMMENT '导出查询关键词',
    filters JSON NULL COMMENT '导出查询白名单筛选条件',
    payload_version SMALLINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '异步任务载荷版本',
    idempotency_key VARCHAR(191) NOT NULL COMMENT '异步任务幂等键',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '导出状态：1待处理，2处理中，3已完成，4失败',
    row_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '实际导出行数，最多100000条',
    file_id CHAR(36) NOT NULL DEFAULT '' COMMENT '完成后生成的受保护文件UUID',
    retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行重试次数',
    next_retry_at DATETIME(3) NULL COMMENT '下次允许执行时间',
    dispatched_at DATETIME(3) NULL COMMENT '最近一次RabbitMQ确认投递时间',
    failure_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '最后失败原因',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    started_at DATETIME(3) NULL COMMENT '开始处理时间',
    finished_at DATETIME(3) NULL COMMENT '处理完成或最终失败时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_log_exports_idempotency (idempotency_key),
    KEY idx_log_exports_realm_tenant (realm, tenant_id),
    KEY idx_log_exports_pending (status, next_retry_at, dispatched_at, created_at),
    KEY idx_log_exports_query (tenant_id, user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='异步日志导出任务表';

CREATE TABLE failed_tasks (
    id CHAR(36) NOT NULL COMMENT '失败任务UUID',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台任务',
    task_type VARCHAR(64) NOT NULL COMMENT '异步任务类型',
    payload_version SMALLINT UNSIGNED NOT NULL COMMENT '任务载荷版本',
    idempotency_key VARCHAR(191) NOT NULL COMMENT '任务幂等键',
    payload JSON NOT NULL COMMENT '脱敏后的任务载荷',
    retry_count INT UNSIGNED NOT NULL COMMENT '已重试次数',
    last_error VARCHAR(1024) NOT NULL COMMENT '最后失败原因',
    next_retry_at DATETIME(3) NULL COMMENT '人工重试后的计划时间',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '任务状态：1失败待处理，2已重新入队，3已忽略',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_failed_tasks_idempotency (idempotency_key),
    KEY idx_failed_tasks_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='异步失败任务记录表';

-- 平台菜单树：四个一级目录（parent_id=0, scope_mask=1）
INSERT INTO resources (parent_id, type, scope_mask, code, name, route_path, component_key, icon, sort_order, visible, status)
VALUES
    (0, 1, 1, 'menu.platform.tenants', '租户运营', '', '', 'OfficeBuilding', 10, 1, 1),
    (0, 1, 1, 'menu.platform.identity', '身份与账号', '', '', 'UserFilled', 20, 1, 1),
    (0, 1, 1, 'menu.platform.system', '系统配置', '', '', 'Setting', 30, 1, 1),
    (0, 1, 1, 'menu.platform.logs', '日志审计', '', '', 'Document', 40, 1, 1);

INSERT INTO resources (parent_id, type, scope_mask, code, name, route_path, component_key, icon, sort_order, visible, status)
VALUES
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.tenants') AS p), 2, 1, 'tenants', '租户管理', '/platform/tenants', 'tenants', 'OfficeBuilding', 11, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.tenants') AS p), 3, 1, 'tenant-setup', '开通配置', '', 'tenant-setup', 'Tools', 12, 0, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.tenants') AS p), 3, 1, 'tenant-resources', '功能授权', '', 'tenant-resources', 'Connection', 13, 0, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.tenants') AS p), 3, 1, 'tenant-admins', '租户管理员', '', 'tenant-admins', 'UserFilled', 14, 0, 1),

    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.identity') AS p), 2, 1, 'app-users', 'App 用户', '/platform/app-users', 'app-users', 'User', 21, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.identity') AS p), 2, 1, 'platform-admins', '平台管理员', '/platform/admins', 'platform-admins', 'UserFilled', 22, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.identity') AS p), 2, 1, 'platform-roles', '平台角色', '/platform/permission/roles', 'platform-roles', 'Lock', 23, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.identity') AS p), 3, 1, 'platform-casbin-rules', '按钮与 API 授权', '', 'platform-casbin-rules', 'Key', 24, 0, 1),

    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.system') AS p), 2, 1, 'resources', '菜单与权限资源', '/platform/resources', 'resources', 'Menu', 31, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.system') AS p), 2, 1, 'providers', '渠道配置', '/platform/settings/providers', 'providers', 'Connection', 32, 1, 1),

    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p), 2, 1, 'platform-login-logs', '登录日志', '/platform/logs/login', 'platform-login-logs', 'List', 41, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p), 2, 1, 'platform-audit-logs', '操作审计', '/platform/logs/audit', 'platform-audit-logs', 'DocumentChecked', 42, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p), 2, 1, 'platform-api-logs', 'API 日志', '/platform/logs/api', 'platform-api-logs', 'Monitor', 43, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform.logs') AS p), 2, 1, 'platform-log-exports', '日志导出', '/platform/logs/exports', 'platform-log-exports', 'Download', 44, 1, 1);

-- 租户侧角色资源（开通配置 Tab 使用，scope_mask=2 不在平台顶栏展示）
INSERT INTO resources (parent_id, type, scope_mask, code, name, route_path, component_key, icon, sort_order, visible, status)
VALUES
    (0, 1, 2, 'menu.tenant.permission', '权限中心', '', '', 'Lock', 10, 0, 1);

INSERT INTO resources (parent_id, type, scope_mask, code, name, route_path, component_key, icon, sort_order, visible, status)
VALUES
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.tenant.permission') AS p), 2, 2, 'roles', '租户角色', '', 'roles', 'Lock', 11, 0, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.tenant.permission') AS p), 2, 2, 'casbin-rules', '按钮与 API 授权', '', 'casbin-rules', 'Key', 12, 0, 1);

-- +goose Down
DROP TABLE IF EXISTS failed_tasks;
DROP TABLE IF EXISTS log_exports;
DROP TABLE IF EXISTS file_references;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS provider_configs;
DROP TABLE IF EXISTS dictionary_items;
DROP TABLE IF EXISTS dictionary_types;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS api_access_logs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS audit_outbox;
DROP TABLE IF EXISTS login_logs;
DROP TABLE IF EXISTS verification_codes;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS casbin_rules;
DROP TABLE IF EXISTS tenant_resources;
DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS app_users;
DROP TABLE IF EXISTS tenant_admins;
DROP TABLE IF EXISTS platform_admins;
DROP TABLE IF EXISTS tenants;
