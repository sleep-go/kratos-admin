-- +goose Up
CREATE TABLE log_exports (
    id CHAR(36) NOT NULL COMMENT '日志导出任务UUID',
    tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属租户ID，0表示平台跨租户导出',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '发起导出的用户ID',
    member_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '发起导出的租户成员ID，平台域为0',
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
    failure_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '最后失败原因',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
    started_at DATETIME(3) NULL COMMENT '开始处理时间',
    finished_at DATETIME(3) NULL COMMENT '处理完成或最终失败时间',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_log_exports_idempotency (idempotency_key),
    KEY idx_log_exports_pending (status, next_retry_at, created_at),
    KEY idx_log_exports_query (tenant_id, user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='异步日志导出任务表';

-- +goose Down
DROP TABLE IF EXISTS log_exports;
