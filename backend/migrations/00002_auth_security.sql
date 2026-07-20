-- +goose Up
ALTER TABLE users
    ADD COLUMN mfa_enabled TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否启用登录MFA：0否，1是' AFTER phone_verified_at,
    ADD COLUMN mfa_channel VARCHAR(16) NOT NULL DEFAULT 'email' COMMENT 'MFA渠道：email邮件，sms短信' AFTER mfa_enabled;

ALTER TABLE verification_codes
    ADD COLUMN context_data JSON NULL COMMENT 'MFA设备上下文等非敏感挑战数据' AFTER consumed_at;

-- +goose Down
ALTER TABLE verification_codes DROP COLUMN context_data;
ALTER TABLE users DROP COLUMN mfa_channel, DROP COLUMN mfa_enabled;
