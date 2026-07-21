-- +goose Up
-- 将旧版 schema 升级到平台/租户认证隔离模型；对已对齐的新库保持幂等。

SET @platform_admins_exists = (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'platform_admins'
);
SET @sql = IF(@platform_admins_exists = 0,
    'CREATE TABLE platform_admins (
        id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT ''平台管理员主键'',
        username VARCHAR(64) NOT NULL COMMENT ''平台管理员唯一用户名'',
        email VARCHAR(191) NULL COMMENT ''平台管理员邮箱'',
        phone VARCHAR(32) NULL COMMENT ''平台管理员手机号'',
        password_hash VARCHAR(255) NOT NULL COMMENT ''Argon2id密码哈希'',
        display_name VARCHAR(128) NOT NULL COMMENT ''显示名称'',
        avatar_url TEXT NULL COMMENT ''头像地址'',
        mfa_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT ''是否启用登录MFA：0否，1是'',
        mfa_channel VARCHAR(16) NOT NULL DEFAULT ''email'' COMMENT ''MFA渠道：email邮件，sms短信'',
        status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT ''账号状态：1启用，2禁用，3锁定'',
        failed_login_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''连续登录失败次数'',
        locked_until DATETIME(3) NULL COMMENT ''锁定截止时间'',
        created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT ''创建时间'',
        updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT ''更新时间'',
        deleted_at DATETIME(3) NULL COMMENT ''逻辑删除时间'',
        PRIMARY KEY (id),
        UNIQUE KEY uk_platform_admins_username (username),
        UNIQUE KEY uk_platform_admins_email (email),
        UNIQUE KEY uk_platform_admins_phone (phone),
        KEY idx_platform_admins_status (status, deleted_at)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT=''平台管理员表''',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @legacy_platform_admin_column = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'is_platform_admin'
);
SET @sql = IF(@legacy_platform_admin_column > 0,
    'INSERT INTO platform_admins (
        id, username, email, phone, password_hash, display_name, avatar_url,
        mfa_enabled, mfa_channel, status, failed_login_count, locked_until,
        created_at, updated_at, deleted_at
    )
    SELECT
        u.id, u.username, u.email, u.phone, u.password_hash, u.display_name, u.avatar_url,
        u.mfa_enabled, u.mfa_channel, u.status, u.failed_login_count, u.locked_until,
        u.created_at, u.updated_at, u.deleted_at
    FROM users u
    WHERE u.is_platform_admin = 1
      AND NOT EXISTS (
          SELECT 1 FROM platform_admins pa WHERE pa.username = u.username
      )',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @realm_column_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_sessions' AND COLUMN_NAME = 'realm'
);
SET @sql = IF(@realm_column_exists = 0,
    'ALTER TABLE auth_sessions
        ADD COLUMN realm VARCHAR(16) NOT NULL DEFAULT ''tenant'' COMMENT ''认证域：platform平台，tenant租户'' AFTER id,
        ADD COLUMN impersonator_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''代维平台管理员ID，非代维为0'' AFTER member_id',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @impersonator_column_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_sessions' AND COLUMN_NAME = 'impersonator_id'
);
SET @sql = IF(@impersonator_column_exists = 0,
    'ALTER TABLE auth_sessions
        ADD COLUMN impersonator_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''代维平台管理员ID，非代维为0'' AFTER member_id',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @auth_sessions_index = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_sessions' AND INDEX_NAME = 'idx_auth_sessions_user' AND SEQ_IN_INDEX = 1 AND COLUMN_NAME = 'realm'
);
SET @sql = IF(@auth_sessions_index = 0,
    'ALTER TABLE auth_sessions
        DROP INDEX idx_auth_sessions_user,
        ADD INDEX idx_auth_sessions_user (realm, user_id, revoked_at, expires_at)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @audit_impersonator_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'audit_logs' AND COLUMN_NAME = 'impersonator_id'
);
SET @sql = IF(@audit_impersonator_exists = 0,
    'ALTER TABLE audit_logs
        ADD COLUMN impersonator_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''代维平台管理员ID，非代维为0'' AFTER member_id',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- +goose Down
SET @audit_impersonator_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'audit_logs' AND COLUMN_NAME = 'impersonator_id'
);
SET @sql = IF(@audit_impersonator_exists > 0, 'ALTER TABLE audit_logs DROP COLUMN impersonator_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @auth_sessions_index = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_sessions' AND INDEX_NAME = 'idx_auth_sessions_user' AND SEQ_IN_INDEX = 1 AND COLUMN_NAME = 'realm'
);
SET @sql = IF(@auth_sessions_index > 0,
    'ALTER TABLE auth_sessions
        DROP INDEX idx_auth_sessions_user,
        ADD INDEX idx_auth_sessions_user (user_id, revoked_at, expires_at)',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @realm_column_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'auth_sessions' AND COLUMN_NAME = 'realm'
);
SET @sql = IF(@realm_column_exists > 0,
    'ALTER TABLE auth_sessions DROP COLUMN impersonator_id, DROP COLUMN realm',
    'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @platform_admins_exists = (
    SELECT COUNT(*) FROM information_schema.TABLES
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'platform_admins'
);
SET @sql = IF(@platform_admins_exists > 0, 'DROP TABLE platform_admins', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
