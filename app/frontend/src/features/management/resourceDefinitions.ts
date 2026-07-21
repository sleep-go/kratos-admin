import {
  auditActionLabel,
  auditActionOptions,
  auditResourceLabel,
  auditResourceOptions,
  auditSummaryLabel
} from './auditLabels'

export interface ResourceOption {
  label: string
  value: string | number
  children?: ResourceOption[]
}

export interface ResourceLookupDefinition {
  resource: string
  valueKey?: string
  labelKeys: string[]
  parentKey?: string
  emptyLabel?: string
  onlyActive?: boolean
  excludeCurrentTree?: boolean
  exclude?: {
    resource: string
    valueKey: string
  }
}

export interface ResourceLookupByDefinition {
  field: string
  values: Record<string, ResourceLookupDefinition>
}

export interface ResourceField {
  key: string
  label: string
  type?:
    | 'text'
    | 'number'
    | 'status'
    | 'boolean'
    | 'textarea'
    | 'password'
    | 'select'
    | 'relation'
    | 'tree'
  required?: boolean
  table?: boolean
  default?: string | number | boolean
  valueLabels?: Record<string, string>
  form?: boolean
  createOnly?: boolean
  options?: ResourceOption[]
  lookup?: ResourceLookupDefinition
  lookupBy?: ResourceLookupByDefinition
  format?: (value: unknown, row: Record<string, unknown>) => unknown
}

export interface ResourceFilter {
  key: string
  label: string
  type: 'text' | 'select'
  options?: ResourceOption[]
}

export interface ResourceScopeTab {
  key: string
  label: string
  description: string
  scopeSide: 'all' | 'platform' | 'tenant'
  defaultScopeMask: number
  scopeMaskOptions: ResourceOption[]
}

export interface ResourceDefinition {
  resource: string
  title: string
  description: string
  fields: ResourceField[]
  readOnly?: boolean
  exportLogType?: 'login' | 'audit' | 'api'
  filters?: ResourceFilter[]
  scopeTabs?: ResourceScopeTab[]
  listMode?: 'table' | 'tree'
}

const commonStatusOptions: ResourceOption[] = [
  { label: '启用', value: 1 },
  { label: '禁用', value: 2 }
]
const commonStatus: ResourceField = {
  key: 'status',
  label: '状态',
  type: 'status',
  table: true,
  options: commonStatusOptions
}
const createdAt: ResourceField = { key: 'created_at', label: '创建时间', table: true }

export const resourceDefinitions: Record<string, ResourceDefinition> = {
  'app-users': {
    resource: 'app-users',
    title: 'App 用户',
    description: '管理 App 端账号及登录 MFA 安全策略。',
    fields: [
      { key: 'username', label: '用户名', required: true, table: true },
      { key: 'email', label: '邮箱', table: true },
      { key: 'phone', label: '手机号', table: true },
      { key: 'display_name', label: '显示名称', required: true, table: true },
      {
        key: 'initial_password',
        label: '初始密码',
        type: 'password',
        required: true,
        createOnly: true
      },
      {
        key: 'status',
        label: '状态',
        type: 'status',
        default: 1,
        table: true,
        options: [...commonStatusOptions, { label: '锁定', value: 3 }]
      },
      { key: 'mfa_enabled', label: '启用 MFA', type: 'boolean', default: false, table: true },
      {
        key: 'mfa_channel',
        label: 'MFA 渠道',
        type: 'select',
        default: 'email',
        table: true,
        options: [
          { label: '邮件', value: 'email' },
          { label: '短信', value: 'sms' }
        ]
      }
    ]
  },
  tenants: {
    resource: 'tenants',
    title: '租户管理',
    description: '创建、冻结和启用平台租户。',
    fields: [
      { key: 'code', label: '租户编码', required: true, table: true },
      { key: 'name', label: '租户名称', required: true, table: true },
      {
        key: 'status',
        label: '状态',
        type: 'status',
        table: true,
        options: [
          { label: '启用', value: 1 },
          { label: '冻结', value: 2 }
        ]
      },
      { key: 'permission_version', label: '权限版本', table: true },
      createdAt
    ]
  },
  'tenant-admins': {
    resource: 'tenant-admins',
    title: '租户管理员',
    description: '管理租户后台管理员账号及登录安全策略。',
    fields: [
      { key: 'username', label: '用户名', required: true, table: true },
      { key: 'email', label: '邮箱', table: true },
      { key: 'phone', label: '手机号', table: true },
      { key: 'display_name', label: '显示名称', required: true, table: true },
      {
        key: 'initial_password',
        label: '初始密码',
        type: 'password',
        required: true,
        createOnly: true
      },
      {
        key: 'status',
        label: '状态',
        type: 'status',
        default: 1,
        table: true,
        options: [...commonStatusOptions, { label: '锁定', value: 3 }]
      },
      { key: 'mfa_enabled', label: '启用 MFA', type: 'boolean', default: false, table: true },
      {
        key: 'mfa_channel',
        label: 'MFA 渠道',
        type: 'select',
        default: 'email',
        table: true,
        options: [
          { label: '邮件', value: 'email' },
          { label: '短信', value: 'sms' }
        ]
      },
      createdAt
    ]
  },
  'platform-admins': {
    resource: 'platform-admins',
    title: '平台管理员',
    description: '管理平台级管理员账号及登录安全策略。',
    fields: [
      { key: 'username', label: '用户名', required: true, table: true },
      { key: 'email', label: '邮箱', table: true },
      { key: 'phone', label: '手机号', table: true },
      { key: 'display_name', label: '显示名称', required: true, table: true },
      {
        key: 'initial_password',
        label: '初始密码',
        type: 'password',
        required: true,
        createOnly: true
      },
      {
        key: 'status',
        label: '状态',
        type: 'status',
        default: 1,
        table: true,
        options: [...commonStatusOptions, { label: '锁定', value: 3 }]
      },
      { key: 'mfa_enabled', label: '启用 MFA', type: 'boolean', default: false, table: true },
      {
        key: 'mfa_channel',
        label: 'MFA 渠道',
        type: 'select',
        default: 'email',
        table: true,
        options: [
          { label: '邮件', value: 'email' },
          { label: '短信', value: 'sms' }
        ]
      },
      createdAt
    ]
  },
  members: {
    resource: 'members',
    title: '成员管理',
    description: '维护当前租户成员及组织归属。',
    fields: [
      {
        key: 'user_id',
        label: '全局用户',
        type: 'relation',
        required: true,
        table: true,
        lookup: {
          resource: 'users',
          labelKeys: ['display_name', 'username'],
          onlyActive: true,
          exclude: { resource: 'members', valueKey: 'user_id' }
        }
      },
      { key: 'display_name', label: '显示名称', required: true, table: true },
      {
        key: 'primary_department_id',
        label: '主部门',
        type: 'tree',
        default: 0,
        table: true,
        lookup: {
          resource: 'departments',
          labelKeys: ['name', 'code'],
          parentKey: 'parent_id',
          emptyLabel: '未分配',
          onlyActive: true
        }
      },
      {
        key: 'position_id',
        label: '岗位',
        type: 'relation',
        default: 0,
        table: true,
        lookup: {
          resource: 'positions',
          labelKeys: ['name', 'code'],
          emptyLabel: '未分配',
          onlyActive: true
        }
      },
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
      {
        key: 'parent_id',
        label: '上级部门',
        type: 'tree',
        default: 0,
        table: true,
        lookup: {
          resource: 'departments',
          labelKeys: ['name', 'code'],
          parentKey: 'parent_id',
          emptyLabel: '根部门',
          onlyActive: true,
          excludeCurrentTree: true
        }
      },
      { key: 'path', label: '部门路径', table: true, form: false },
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
    title: '角色与权限',
    description: '配置角色与状态。',
    fields: [
      { key: 'code', label: '角色编码', required: true, table: true },
      { key: 'name', label: '角色名称', required: true, table: true },
      { key: 'is_builtin', label: '内置角色', type: 'boolean', table: true },
      commonStatus
    ]
  },
  'platform-roles': {
    resource: 'platform-roles',
    title: '平台角色',
    description: '配置平台域角色与状态。',
    fields: [
      { key: 'code', label: '角色编码', required: true, table: true },
      { key: 'name', label: '角色名称', required: true, table: true },
      { key: 'is_builtin', label: '内置角色', type: 'boolean', table: true },
      commonStatus
    ]
  },
  resources: {
    resource: 'resources',
    title: '菜单与权限资源',
    description: '按平台与租户视角分别维护目录、菜单、按钮和 API 资源。',
    listMode: 'tree',
    scopeTabs: [
      {
        key: 'all',
        label: '全部',
        description: '查看完整的目录、菜单、按钮和 API 资源树。',
        scopeSide: 'all',
        defaultScopeMask: 3,
        scopeMaskOptions: [
          { label: '仅平台', value: 1 },
          { label: '仅租户', value: 2 },
          { label: '平台与租户共用', value: 3 }
        ]
      },
      {
        key: 'platform',
        label: '平台资源',
        description: '用于平台治理视角的目录、菜单、按钮和 API。',
        scopeSide: 'platform',
        defaultScopeMask: 1,
        scopeMaskOptions: [
          { label: '仅平台', value: 1 },
          { label: '平台与租户共用', value: 3 }
        ]
      },
      {
        key: 'tenant',
        label: '租户资源',
        description: '用于租户控制台与功能授权的目录、菜单、按钮和 API。',
        scopeSide: 'tenant',
        defaultScopeMask: 2,
        scopeMaskOptions: [
          { label: '仅租户', value: 2 },
          { label: '平台与租户共用', value: 3 }
        ]
      }
    ],
    fields: [
      { key: 'name', label: '资源名称', required: true, table: true },
      { key: 'code', label: '资源编码', required: true, table: true },
      {
        key: 'type',
        label: '类型',
        type: 'select',
        required: true,
        table: true,
        options: [
          { label: '目录', value: 1 },
          { label: '菜单', value: 2 },
          { label: '按钮', value: 3 },
          { label: 'API', value: 4 }
        ]
      },
      {
        key: 'scope_mask',
        label: '适用范围',
        type: 'select',
        required: true,
        default: 3,
        table: true,
        options: [
          { label: '仅平台', value: 1 },
          { label: '仅租户', value: 2 },
          { label: '平台与租户共用', value: 3 }
        ]
      },
      {
        key: 'parent_id',
        label: '父资源',
        type: 'tree',
        default: 0,
        lookup: {
          resource: 'resources',
          labelKeys: ['name', 'code'],
          parentKey: 'parent_id',
          emptyLabel: '根资源',
          onlyActive: true,
          excludeCurrentTree: true
        }
      },
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
    readOnly: true,
    fields: [
      {
        key: 'tenant_id',
        label: '租户',
        type: 'relation',
        required: true,
        table: true,
        lookup: { resource: 'tenants', labelKeys: ['name', 'code'], onlyActive: true }
      },
      {
        key: 'resource_id',
        label: '权限资源',
        type: 'tree',
        required: true,
        table: true,
        lookup: {
          resource: 'resources',
          labelKeys: ['name', 'code'],
          parentKey: 'parent_id',
          onlyActive: true
        }
      },
      { key: 'created_by', label: '授权人', table: true },
      createdAt
    ]
  },
  'casbin-rules': {
    resource: 'casbin-rules',
    title: '按钮与 API 授权',
    description: '维护 Casbin domain 角色和资源策略。',
    fields: [
      {
        key: 'ptype',
        label: '策略类型',
        type: 'select',
        default: 'p',
        required: true,
        table: true,
        options: [
          { label: '资源授权', value: 'p' },
          { label: '角色继承', value: 'g' }
        ]
      },
      {
        key: 'v1',
        label: '成员 / 角色',
        type: 'relation',
        required: true,
        table: true,
        lookupBy: {
          field: 'ptype',
          values: {
            p: { resource: 'roles', labelKeys: ['name', 'code'], onlyActive: true },
            g: { resource: 'tenant-admins', labelKeys: ['display_name', 'username'], onlyActive: true }
          }
        }
      },
      {
        key: 'v2',
        label: '资源 / 角色',
        type: 'relation',
        required: true,
        table: true,
        lookupBy: {
          field: 'ptype',
          values: {
            p: {
              resource: 'resources',
              valueKey: 'code',
              labelKeys: ['name', 'code'],
              onlyActive: true
            },
            g: { resource: 'roles', labelKeys: ['name', 'code'], onlyActive: true }
          }
        }
      },
      { key: 'v3', label: '动作', table: true },
      { key: 'v4', label: '扩展值 4' },
      { key: 'v5', label: '扩展值 5' }
    ]
  },
  'platform-casbin-rules': {
    resource: 'platform-casbin-rules',
    title: '平台按钮与 API 授权',
    description: '维护平台域 Casbin 角色和资源策略。',
    fields: [
      {
        key: 'ptype',
        label: '策略类型',
        type: 'select',
        default: 'p',
        required: true,
        table: true,
        options: [
          { label: '资源授权', value: 'p' },
          { label: '角色继承', value: 'g' }
        ]
      },
      {
        key: 'v1',
        label: '管理员 / 角色',
        type: 'relation',
        required: true,
        table: true,
        lookupBy: {
          field: 'ptype',
          values: {
            p: { resource: 'platform-roles', labelKeys: ['name', 'code'], onlyActive: true },
            g: { resource: 'platform-admins', labelKeys: ['display_name', 'username'], onlyActive: true }
          }
        }
      },
      {
        key: 'v2',
        label: '资源 / 角色',
        type: 'relation',
        required: true,
        table: true,
        lookupBy: {
          field: 'ptype',
          values: {
            p: {
              resource: 'resources',
              valueKey: 'code',
              labelKeys: ['name', 'code'],
              onlyActive: true
            },
            g: { resource: 'platform-roles', labelKeys: ['name', 'code'], onlyActive: true }
          }
        }
      },
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
    exportLogType: 'login',
    filters: [
      {
        key: 'result',
        label: '登录结果',
        type: 'select',
        options: [
          { label: '成功', value: '1' },
          { label: '失败', value: '2' },
          { label: '账号锁定', value: '3' },
          { label: '需要 MFA', value: '4' }
        ]
      },
      { key: 'user_id', label: '用户 ID', type: 'text' }
    ],
    fields: [
      { key: 'identifier', label: '登录标识', table: true },
      {
        key: 'result',
        label: '结果',
        table: true,
        valueLabels: { '1': '成功', '2': '失败', '3': '账号锁定', '4': '需要 MFA' }
      },
      {
        key: 'reason',
        label: '原因',
        table: true,
        valueLabels: {
          AUTH_INVALID_ARGUMENT: '账号和密码不能为空',
          CAPTCHA_INVALID: '图形验证码错误或已过期',
          AUTH_INVALID_CREDENTIALS: '账号或密码错误',
          AUTH_ACCOUNT_LOCKED: '账号已被临时锁定',
          AUTH_ACCOUNT_DISABLED: '账号已被禁用',
          AUTH_NO_TENANT: '账号没有可用租户',
          AUTH_INTERNAL: '认证服务暂时不可用',
          MFA_REQUIRED: '需要 MFA 验证'
        }
      },
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
    exportLogType: 'audit',
    filters: [
      { key: 'action', label: '操作类型', type: 'select', options: auditActionOptions },
      {
        key: 'resource_type',
        label: '资源类型',
        type: 'select',
        options: auditResourceOptions
      },
      { key: 'user_id', label: '用户 ID', type: 'text' },
      { key: 'member_id', label: '成员 ID', type: 'text' }
    ],
    fields: [
      {
        key: 'summary',
        label: '操作摘要',
        table: true,
        format: (value, row) =>
          auditSummaryLabel(value, row.action, row.resource_type, row.resource_id)
      },
      { key: 'action', label: '动作', table: true, format: auditActionLabel },
      {
        key: 'resource_type',
        label: '资源类型',
        table: true,
        format: auditResourceLabel
      },
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
    exportLogType: 'api',
    filters: [
      {
        key: 'method',
        label: 'HTTP 方法',
        type: 'select',
        options: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((value) => ({ label: value, value }))
      },
      {
        key: 'status_code',
        label: '状态码',
        type: 'select',
        options: ['200', '400', '401', '403', '404', '409', '500'].map((value) => ({
          label: value,
          value
        }))
      },
      { key: 'user_id', label: '用户 ID', type: 'text' }
    ],
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
  'log-exports': {
    resource: 'log-exports',
    title: '日志导出记录',
    description: '查询异步导出状态并下载已完成的受保护文件。',
    readOnly: true,
    filters: [
      {
        key: 'log_type',
        label: '日志类型',
        type: 'select',
        options: [
          { label: '登录日志', value: 'login' },
          { label: '操作审计', value: 'audit' },
          { label: 'API 日志', value: 'api' }
        ]
      },
      {
        key: 'status',
        label: '任务状态',
        type: 'select',
        options: [
          { label: '等待处理', value: '1' },
          { label: '处理中', value: '2' },
          { label: '已完成', value: '3' },
          { label: '失败', value: '4' }
        ]
      }
    ],
    fields: [
      {
        key: 'log_type',
        label: '日志类型',
        table: true,
        valueLabels: { login: '登录日志', audit: '操作审计', api: 'API 日志' }
      },
      {
        key: 'status',
        label: '任务状态',
        table: true,
        valueLabels: { '1': '等待处理', '2': '处理中', '3': '已完成', '4': '失败' }
      },
      { key: 'row_count', label: '导出条数', table: true },
      { key: 'file_id', label: '文件 ID', table: true },
      { key: 'retry_count', label: '重试次数', table: true },
      { key: 'failure_reason', label: '失败原因', table: true },
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
      {
        key: 'status',
        label: '状态',
        type: 'status',
        table: true,
        valueLabels: {
          '1': '待确认',
          '2': '可用',
          '3': '已删除',
          '4': '清理失败',
          '5': '等待后台清理'
        }
      },
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
      {
        key: 'type_id',
        label: '字典类型',
        type: 'relation',
        required: true,
        table: true,
        lookup: {
          resource: 'dictionary-types',
          labelKeys: ['name', 'code'],
          onlyActive: true
        }
      },
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
