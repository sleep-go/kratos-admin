-- +goose Up
ALTER TABLE audit_outbox
    DROP INDEX idx_audit_outbox_dispatch,
    ADD COLUMN dispatched_at DATETIME(3) NULL COMMENT '最近一次RabbitMQ确认投递时间' AFTER next_retry_at,
    ADD COLUMN last_error VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '最后一次处理失败原因' AFTER dispatched_at,
    MODIFY COLUMN status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理，2已完成，3等待重试，4最终失败',
    ADD KEY idx_audit_outbox_dispatch (status, next_retry_at, dispatched_at, created_at);

ALTER TABLE log_exports
    DROP INDEX idx_log_exports_pending,
    ADD COLUMN dispatched_at DATETIME(3) NULL COMMENT '最近一次RabbitMQ确认投递时间' AFTER next_retry_at,
    ADD KEY idx_log_exports_pending (status, next_retry_at, dispatched_at, created_at);

ALTER TABLE files
    ADD COLUMN cleanup_dispatched_at DATETIME(3) NULL COMMENT '文件清理任务最近一次RabbitMQ确认投递时间' AFTER status,
    ADD COLUMN cleanup_retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件清理失败重试次数' AFTER cleanup_dispatched_at,
    ADD COLUMN cleanup_next_retry_at DATETIME(3) NULL COMMENT '文件清理下次重试时间' AFTER cleanup_retry_count,
    ADD COLUMN cleanup_failure_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '文件清理最后失败原因' AFTER cleanup_next_retry_at,
    MODIFY COLUMN status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '文件状态：1待确认，2可用，3已删除，4清理失败，5等待后台清理',
    ADD KEY idx_files_cleanup (status, cleanup_next_retry_at, cleanup_dispatched_at, updated_at);

-- +goose Down
ALTER TABLE files
    DROP INDEX idx_files_cleanup,
    DROP COLUMN cleanup_failure_reason,
    DROP COLUMN cleanup_next_retry_at,
    DROP COLUMN cleanup_retry_count,
    DROP COLUMN cleanup_dispatched_at,
    MODIFY COLUMN status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '文件状态：1待确认，2可用，3已删除，4清理失败，5等待Worker清理';

ALTER TABLE log_exports
    DROP INDEX idx_log_exports_pending,
    DROP COLUMN dispatched_at,
    ADD KEY idx_log_exports_pending (status, next_retry_at, created_at);

ALTER TABLE audit_outbox
    DROP INDEX idx_audit_outbox_dispatch,
    DROP COLUMN last_error,
    DROP COLUMN dispatched_at,
    MODIFY COLUMN status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理，2已发布，3失败',
    ADD KEY idx_audit_outbox_dispatch (status, next_retry_at, created_at);
