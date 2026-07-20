-- +goose Up
ALTER TABLE files
    MODIFY COLUMN status TINYINT UNSIGNED NOT NULL DEFAULT 1
    COMMENT '文件状态：1待确认，2可用，3已删除，4清理失败，5等待Worker清理';

-- +goose Down
ALTER TABLE files
    MODIFY COLUMN status TINYINT UNSIGNED NOT NULL DEFAULT 1
    COMMENT '文件状态：1待确认，2可用，3已删除，4清理失败';
