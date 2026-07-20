export interface ResourceField {
  key: string
  label: string
  type?: 'text' | 'number' | 'status' | 'boolean' | 'textarea'
  required?: boolean
  table?: boolean
}

export interface ResourceDefinition {
  resource: string
  title: string
  description: string
  fields: ResourceField[]
  readOnly?: boolean
}

const commonStatus: ResourceField = { key: 'status', label: '状态', type: 'status', table: true }
const createdAt: ResourceField = { key: 'created_at', label: '创建时间', table: true }

export const resourceDefinitions: Record<string, ResourceDefinition> = {
  tenants: {
    resource: 'tenants',
    title: '租户管理',
    description: '创建、冻结和启用平台租户。',
    fields: [
      { key: 'code', label: '租户编码', required: true, table: true },
      { key: 'name', label: '租户名称', required: true, table: true },
      {
        key: 'admin_user_id',
        label: '租户管理员用户 ID（留空则为当前平台管理员）',
        type: 'number'
      },
      commonStatus,
      { key: 'permission_version', label: '权限版本', table: true },
      createdAt
    ]
  },
  members: {
    resource: 'members',
    title: '成员管理',
    description: '维护当前租户成员及组织归属。',
    fields: [
      { key: 'user_id', label: '用户 ID', type: 'number', required: true, table: true },
      { key: 'display_name', label: '显示名称', required: true, table: true },
      { key: 'primary_department_id', label: '主部门 ID', type: 'number', table: true },
      { key: 'position_id', label: '岗位 ID', type: 'number', table: true },
      { key: 'is_tenant_admin', label: '租户管理员', type: 'boolean', table: true },
      commonStatus
    ]
  },
  departments: {
    resource: 'departments',
    title: '部门管理',
    description: '维护支持层级路径的部门树。',
    fields: [
      { key: 'name', label: '部门名称', required: true, table: true },
      { key: 'code', label: '部门编码', required: true, table: true },
      { key: 'parent_id', label: '上级部门 ID', type: 'number', table: true },
      { key: 'path', label: '部门路径', required: true, table: true },
      { key: 'sort_order', label: '排序', type: 'number' },
      commonStatus
    ]
  },
  positions: {
    resource: 'positions',
    title: '岗位管理',
    description: '配置租户岗位和展示顺序。',
    fields: [
      { key: 'code', label: '岗位编码', required: true, table: true },
      { key: 'name', label: '岗位名称', required: true, table: true },
      { key: 'sort_order', label: '排序', type: 'number', table: true },
      commonStatus
    ]
  },
  roles: {
    resource: 'roles',
    title: '角色与数据权限',
    description: '配置角色、状态及数据范围。',
    fields: [
      { key: 'code', label: '角色编码', required: true, table: true },
      { key: 'name', label: '角色名称', required: true, table: true },
      { key: 'data_scope', label: '数据范围', type: 'number', table: true },
      { key: 'is_builtin', label: '内置角色', type: 'boolean', table: true },
      commonStatus
    ]
  },
  resources: {
    resource: 'resources',
    title: '菜单与权限资源',
    description: '平台统一维护目录、菜单、按钮和 API 资源。',
    fields: [
      { key: 'name', label: '资源名称', required: true, table: true },
      { key: 'code', label: '资源编码', required: true, table: true },
      { key: 'type', label: '类型', type: 'number', table: true },
      { key: 'parent_id', label: '父资源 ID', type: 'number' },
      { key: 'route_path', label: '路由路径', table: true },
      { key: 'component_key', label: '组件键', table: true },
      { key: 'http_method', label: 'HTTP 方法' },
      { key: 'api_path', label: 'API 路径' },
      { key: 'sort_order', label: '排序', type: 'number' },
      { key: 'visible', label: '显示', type: 'boolean' },
      commonStatus
    ]
  },
  'tenant-resources': {
    resource: 'tenant-resources',
    title: '租户功能授权',
    description: '控制每个租户可使用的功能集合。',
    fields: [
      { key: 'tenant_id', label: '租户 ID', type: 'number', required: true, table: true },
      { key: 'resource_id', label: '资源 ID', type: 'number', required: true, table: true },
      { key: 'created_by', label: '授权人', table: true },
      createdAt
    ]
  },
  'casbin-rules': {
    resource: 'casbin-rules',
    title: '按钮与 API 授权',
    description: '维护 Casbin domain 角色和资源策略。',
    fields: [
      { key: 'ptype', label: '策略类型', required: true, table: true },
      { key: 'v1', label: '成员 / 角色', required: true, table: true },
      { key: 'v2', label: '资源 / 角色', required: true, table: true },
      { key: 'v3', label: '动作', table: true },
      { key: 'v4', label: '扩展值 4' },
      { key: 'v5', label: '扩展值 5' }
    ]
  },
  'login-logs': {
    resource: 'login-logs',
    title: '登录日志',
    description: '查看登录成功、失败、锁定与 MFA 事件。',
    readOnly: true,
    fields: [
      { key: 'identifier', label: '登录标识', table: true },
      { key: 'result', label: '结果', table: true },
      { key: 'reason', label: '原因', table: true },
      { key: 'ip', label: 'IP', table: true },
      { key: 'request_id', label: '请求 ID', table: true },
      createdAt
    ]
  },
  'audit-logs': {
    resource: 'audit-logs',
    title: '操作审计',
    description: '查询由事务 Outbox 可靠生成的业务审计。',
    readOnly: true,
    fields: [
      { key: 'summary', label: '操作摘要', table: true },
      { key: 'action', label: '动作', table: true },
      { key: 'resource_type', label: '资源类型', table: true },
      { key: 'resource_id', label: '资源 ID', table: true },
      { key: 'user_id', label: '用户 ID', table: true },
      { key: 'request_id', label: '请求 ID', table: true },
      createdAt
    ]
  },
  'api-logs': {
    resource: 'api-logs',
    title: 'API 访问与异常日志',
    description: '按路由、状态码和请求 ID 排查接口异常。',
    readOnly: true,
    fields: [
      { key: 'method', label: '方法', table: true },
      { key: 'route', label: '路由', table: true },
      { key: 'status_code', label: '状态码', table: true },
      { key: 'duration_ms', label: '耗时(ms)', table: true },
      { key: 'request_id', label: '请求 ID', table: true },
      { key: 'error_reason', label: '异常原因', table: true },
      createdAt
    ]
  },
  files: {
    resource: 'files',
    title: '文件管理',
    description: '查看租户文件元数据与存储状态。',
    readOnly: true,
    fields: [
      { key: 'original_name', label: '文件名', table: true },
      { key: 'content_type', label: '类型', table: true },
      { key: 'size_bytes', label: '大小', table: true },
      { key: 'provider_name', label: '存储', table: true },
      commonStatus,
      createdAt
    ]
  },
  'dictionary-types': {
    resource: 'dictionary-types',
    title: '参数字典',
    description: '维护平台与租户参数字典类型。',
    fields: [
      { key: 'code', label: '字典编码', required: true, table: true },
      { key: 'name', label: '字典名称', required: true, table: true },
      commonStatus,
      createdAt
    ]
  },
  'dictionary-items': {
    resource: 'dictionary-items',
    title: '字典项',
    description: '维护字典项值、标签和排序。',
    fields: [
      { key: 'type_id', label: '类型 ID', type: 'number', required: true, table: true },
      { key: 'item_value', label: '字典值', required: true, table: true },
      { key: 'label', label: '显示名称', required: true, table: true },
      { key: 'sort_order', label: '排序', type: 'number', table: true },
      commonStatus
    ]
  },
  providers: {
    resource: 'providers',
    title: '渠道配置',
    description: '查看邮件、短信与对象存储 Provider 的启用状态。',
    readOnly: true,
    fields: [
      { key: 'provider_type', label: '类型', table: true },
      { key: 'provider_name', label: '实现', table: true },
      { key: 'display_name', label: '名称', table: true },
      { key: 'is_default', label: '默认', type: 'boolean', table: true },
      commonStatus
    ]
  },
  settings: {
    resource: 'settings',
    title: '系统设置',
    description: '维护平台默认值和租户允许覆盖的配置。',
    fields: [
      { key: 'category', label: '分类', required: true, table: true },
      { key: 'setting_key', label: '配置键', required: true, table: true },
      { key: 'value_type', label: '值类型', required: true, table: true },
      { key: 'setting_value', label: '配置值', type: 'textarea' },
      { key: 'allow_tenant_override', label: '允许租户覆盖', type: 'boolean', table: true },
      { key: 'is_secret', label: '敏感值', type: 'boolean', table: true },
      { key: 'version', label: '版本', table: true }
    ]
  }
}
