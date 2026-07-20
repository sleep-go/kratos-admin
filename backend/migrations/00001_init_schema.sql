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

CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户主键',
    username VARCHAR(64) NOT NULL COMMENT '全局唯一用户名',
    email VARCHAR(191) NULL COMMENT '全局唯一邮箱',
    phone VARCHAR(32) NULL COMMENT '全局唯一手机号',
    password_hash VARCHAR(255) NOT NULL COMMENT 'Argon2id密码哈希',
    display_name VARCHAR(128) NOT NULL COMMENT '用户显示名称',
    avatar_url TEXT NULL COMMENT '头像地址',
    is_platform_admin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否平台管理员：0否，1是',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '用户状态：1启用，2禁用，3锁定',
    failed_login_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '连续登录失败次数',
    locked_until DATETIME(3) NULL COMMENT '锁定截止时间',
    email_verified_at DATETIME(3) NULL COMMENT '邮箱验证时间',
    phone_verified_at DATETIME(3) NULL COMMENT '手机号验证时间',
    password_changed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '密码最后修改时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_username (username),
    UNIQUE KEY uk_users_email (email),
    UNIQUE KEY uk_users_phone (phone),
    KEY idx_users_status (status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='全局用户表';

CREATE TABLE departments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '部门主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    parent_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '父部门ID，0表示根部门',
    name VARCHAR(128) NOT NULL COMMENT '部门名称',
    code VARCHAR(64) NOT NULL COMMENT '租户内唯一部门编码',
    path VARCHAR(1024) NOT NULL COMMENT '包含自身的部门层级路径',
    sort_order INT NOT NULL DEFAULT 0 COMMENT '显示排序值',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '部门状态：1启用，2禁用',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_departments_tenant_code (tenant_id, code),
    KEY idx_departments_tenant_parent (tenant_id, parent_id, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='部门表';

CREATE TABLE positions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '岗位主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    code VARCHAR(64) NOT NULL COMMENT '租户内唯一岗位编码',
    name VARCHAR(128) NOT NULL COMMENT '岗位名称',
    sort_order INT NOT NULL DEFAULT 0 COMMENT '显示排序值',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '岗位状态：1启用，2禁用',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_positions_tenant_code (tenant_id, code),
    KEY idx_positions_tenant_status (tenant_id, status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='岗位表';

CREATE TABLE tenant_members (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '租户成员主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '关联全局用户ID',
    primary_department_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '主部门ID，0表示未分配',
    position_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '岗位ID，0表示未分配',
    display_name VARCHAR(128) NOT NULL COMMENT '租户内显示名称',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '成员状态：1启用，2禁用',
    is_tenant_admin TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否租户管理员：0否，1是',
    joined_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '加入租户时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_members_tenant_user (tenant_id, user_id),
    KEY idx_tenant_members_user (user_id, status, deleted_at),
    KEY idx_tenant_members_department (tenant_id, primary_department_id, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户成员表';

CREATE TABLE member_departments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '成员部门关系主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    member_id BIGINT UNSIGNED NOT NULL COMMENT '租户成员ID',
    department_id BIGINT UNSIGNED NOT NULL COMMENT '部门ID',
    is_primary TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否主部门：0否，1是',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_member_departments_relation (tenant_id, member_id, department_id),
    KEY idx_member_departments_department (tenant_id, department_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='成员部门关系表';

CREATE TABLE roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID，0表示平台角色',
    code VARCHAR(64) NOT NULL COMMENT '作用域内唯一角色编码',
    name VARCHAR(128) NOT NULL COMMENT '角色名称',
    data_scope TINYINT UNSIGNED NOT NULL DEFAULT 4 COMMENT '数据范围：1全部，2本部门及下级，3本部门，4仅本人，5自定义部门',
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
    v1 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值1：角色或成员ID',
    v2 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值2：资源编码或角色ID',
    v3 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值3：资源动作',
    v4 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值4：预留',
    v5 VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' COMMENT '策略值5：预留',
    PRIMARY KEY (id),
    UNIQUE KEY uk_casbin_rules_policy (ptype, v0, v1, v2, v3, v4, v5),
    KEY idx_casbin_rules_domain (v0, ptype)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Casbin权限策略表';

CREATE TABLE role_scope_departments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色自定义部门范围主键',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '所属租户ID',
    role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
    department_id BIGINT UNSIGNED NOT NULL COMMENT '允许访问的部门ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_role_scope_departments_relation (tenant_id, role_id, department_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色自定义部门数据范围表';

CREATE TABLE auth_sessions (
    id CHAR(36) NOT NULL COMMENT '会话UUID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    tenant_id BIGINT UNSIGNED NOT NULL COMMENT '当前租户ID，0表示平台域',
    member_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前租户成员ID，平台域为0',
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
    KEY idx_auth_sessions_user (user_id, revoked_at, expires_at)
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
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_verification_codes_lookup (target, scene, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邮件短信验证码表';

CREATE TABLE login_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '登录日志主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '登录租户ID，0表示未选择或平台域',
    user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID，未识别用户为0',
    identifier VARCHAR(191) NOT NULL COMMENT '脱敏后的登录标识',
    result TINYINT UNSIGNED NOT NULL COMMENT '登录结果：1成功，2失败，3锁定，4需要MFA',
    reason VARCHAR(128) NOT NULL DEFAULT '' COMMENT '登录结果原因',
    ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '客户端IP',
    user_agent VARCHAR(512) NOT NULL DEFAULT '' COMMENT '客户端User-Agent',
    request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求追踪ID',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '发生时间',
    PRIMARY KEY (id),
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
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理，2已发布，3失败',
    retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '重试次数',
    next_retry_at DATETIME(3) NULL COMMENT '下次重试时间',
    published_at DATETIME(3) NULL COMMENT '成功发布时间',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_audit_outbox_dispatch (status, next_retry_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='事务审计Outbox表';

CREATE TABLE audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '操作审计日志主键',
    event_id CHAR(36) NOT NULL COMMENT '来源Outbox事件UUID',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台域',
    user_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作用户ID，系统任务为0',
    member_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作成员ID，平台域或系统任务为0',
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
    KEY idx_audit_logs_query (tenant_id, created_at),
    KEY idx_audit_logs_actor (tenant_id, user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='操作审计日志表';

CREATE TABLE api_access_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'API访问日志主键',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示未认证或平台域',
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
    uploader_member_id BIGINT UNSIGNED NOT NULL COMMENT '上传成员ID',
    provider_name VARCHAR(64) NOT NULL COMMENT '存储Provider名称',
    object_key VARCHAR(512) NOT NULL COMMENT '对象存储键',
    original_name VARCHAR(255) NOT NULL COMMENT '原始文件名',
    content_type VARCHAR(128) NOT NULL COMMENT '文件MIME类型',
    size_bytes BIGINT UNSIGNED NOT NULL COMMENT '文件大小字节数',
    sha256 CHAR(64) NOT NULL DEFAULT '' COMMENT '文件SHA256摘要',
    status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '文件状态：1待确认，2可用，3已删除，4清理失败',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    deleted_at DATETIME(3) NULL COMMENT '逻辑删除时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_files_provider_object (provider_name, object_key),
    KEY idx_files_tenant (tenant_id, status, created_at)
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

-- +goose Down
DROP TABLE IF EXISTS failed_tasks;
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
DROP TABLE IF EXISTS role_scope_departments;
DROP TABLE IF EXISTS casbin_rules;
DROP TABLE IF EXISTS tenant_resources;
DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS member_departments;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;
