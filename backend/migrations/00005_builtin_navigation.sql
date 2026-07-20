-- +goose Up
INSERT INTO resources (parent_id, type, code, name, route_path, component_key, icon, sort_order, visible, status)
VALUES
    (0, 1, 'menu.platform', '平台管理', '', '', 'OfficeBuilding', 10, 1, 1),
    (0, 1, 'menu.organization', '组织管理', '', '', 'UserFilled', 20, 1, 1),
    (0, 1, 'menu.permission', '权限中心', '', '', 'Lock', 30, 1, 1),
    (0, 1, 'menu.logs', '日志中心', '', '', 'Document', 40, 1, 1),
    (0, 1, 'menu.storage', '文件管理', '', '', 'Folder', 50, 1, 1),
    (0, 1, 'menu.settings', '系统设置', '', '', 'Setting', 60, 1, 1)
ON DUPLICATE KEY UPDATE name = VALUES(name), type = VALUES(type), sort_order = VALUES(sort_order), visible = 1, status = 1;

INSERT INTO resources (parent_id, type, code, name, route_path, component_key, icon, sort_order, visible, status)
VALUES
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform') AS parent), 2, 'users', '全局用户', '/platform/users', 'users', 'User', 11, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.platform') AS parent), 2, 'tenants', '租户管理', '/platform/tenants', 'tenants', 'OfficeBuilding', 12, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.organization') AS parent), 2, 'members', '成员管理', '/organization/users', 'members', 'UserFilled', 21, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.organization') AS parent), 2, 'departments', '部门管理', '/organization/departments', 'departments', 'Share', 22, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.organization') AS parent), 2, 'positions', '岗位管理', '/organization/positions', 'positions', 'Postcard', 23, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.permission') AS parent), 2, 'roles', '角色与数据权限', '/permission/roles', 'roles', 'Lock', 31, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.permission') AS parent), 2, 'resources', '菜单与权限资源', '/permission/resources', 'resources', 'Menu', 32, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.permission') AS parent), 2, 'tenant-resources', '租户功能授权', '/permission/tenant-features', 'tenant-resources', 'Connection', 33, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.permission') AS parent), 2, 'casbin-rules', '按钮与 API 授权', '/permission/policies', 'casbin-rules', 'Key', 34, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.logs') AS parent), 2, 'login-logs', '登录日志', '/logs/login', 'login-logs', 'List', 41, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.logs') AS parent), 2, 'audit-logs', '操作审计', '/logs/audit', 'audit-logs', 'DocumentChecked', 42, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.logs') AS parent), 2, 'api-logs', 'API 日志', '/logs/api', 'api-logs', 'Monitor', 43, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.logs') AS parent), 2, 'log-exports', '日志导出', '/logs/exports', 'log-exports', 'Download', 44, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.storage') AS parent), 2, 'files', '文件管理', '/files', 'files', 'Folder', 51, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.settings') AS parent), 2, 'settings', '系统设置', '/settings', 'settings', 'Setting', 61, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.settings') AS parent), 2, 'providers', '渠道配置', '/settings/providers', 'providers', 'Connection', 62, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.settings') AS parent), 2, 'dictionary-types', '参数字典', '/settings/dictionaries', 'dictionary-types', 'Collection', 63, 1, 1),
    ((SELECT id FROM (SELECT id FROM resources WHERE code = 'menu.settings') AS parent), 2, 'dictionary-items', '字典项', '/settings/dictionary-items', 'dictionary-items', 'Tickets', 64, 1, 1)
ON DUPLICATE KEY UPDATE
    parent_id = VALUES(parent_id), type = VALUES(type), name = VALUES(name), route_path = VALUES(route_path),
    component_key = VALUES(component_key), icon = VALUES(icon), sort_order = VALUES(sort_order), visible = 1, status = 1;

-- +goose Down
DELETE FROM resources
WHERE code IN (
    'users', 'tenants', 'members', 'departments', 'positions', 'roles', 'resources', 'tenant-resources',
    'casbin-rules', 'login-logs', 'audit-logs', 'api-logs', 'log-exports', 'files', 'settings',
    'providers', 'dictionary-types', 'dictionary-items', 'menu.platform', 'menu.organization',
    'menu.permission', 'menu.logs', 'menu.storage', 'menu.settings'
);
