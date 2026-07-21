-- +goose Up
ALTER TABLE resources
    ADD COLUMN scope_mask TINYINT UNSIGNED NOT NULL DEFAULT 3
        COMMENT '资源适用范围位标记：1仅平台，2仅租户，3平台与租户共用'
        AFTER type;

UPDATE resources SET scope_mask = 1
WHERE code IN ('menu.platform', 'users', 'tenants', 'resources', 'tenant-resources');

UPDATE resources SET scope_mask = 2
WHERE code IN ('menu.organization', 'members', 'departments', 'positions', 'roles', 'casbin-rules');

UPDATE resources SET scope_mask = 3
WHERE code IN (
    'menu.permission', 'menu.logs', 'menu.storage', 'menu.settings',
    'login-logs', 'audit-logs', 'api-logs', 'log-exports', 'files',
    'settings', 'providers', 'dictionary-types', 'dictionary-items'
);

INSERT INTO resources (
    parent_id, type, scope_mask, code, name, route_path, component_key,
    icon, sort_order, visible, status
)
SELECT id, 3, 1, 'tenant-setup', '租户初始化', '', '', 'Tools', 13, 0, 1
FROM resources
WHERE code = 'menu.platform'
ON DUPLICATE KEY UPDATE
    scope_mask = VALUES(scope_mask), name = VALUES(name), visible = VALUES(visible), status = VALUES(status);

-- +goose Down
DELETE FROM resources WHERE code = 'tenant-setup';

ALTER TABLE resources DROP COLUMN scope_mask;
