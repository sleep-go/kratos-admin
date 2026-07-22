-- +goose Up
-- 平台 Casbin 策略改由「平台角色」页统一管理，隐藏独立菜单入口。
UPDATE resources SET visible = 0, type = 3, route_path = '' WHERE code = 'platform-casbin-rules';

-- +goose Down
UPDATE resources SET visible = 1, type = 2, route_path = '/platform/permission/policies' WHERE code = 'platform-casbin-rules';
