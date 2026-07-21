const categoryLabels: Readonly<Record<string, string>> = {
  platform: '平台设置',
  security: '安全策略',
  log: '日志保留',
  file: '文件设置',
  email: '邮件设置',
  sms: '短信设置',
  storage: '存储设置'
}

const keyLabels: Readonly<Record<string, string>> = {
  'platform.site_name': '系统名称',
  'security.access_token_minutes': '访问令牌有效时间（分钟）',
  'security.refresh_token_days': '刷新令牌有效时间（天）',
  'security.login_failure_limit': '连续登录失败上限',
  'security.login_lock_minutes': '登录锁定时间（分钟）',
  'log.audit_retention_days': '操作审计保留天数',
  'log.login_retention_days': '登录日志保留天数',
  'log.api_retention_days': 'API 日志保留天数',
  'file.max_upload_mb': '单文件上传上限（MB）'
}

const valueTypeLabels: Readonly<Record<string, string>> = {
  string: '字符串',
  number: '数字',
  boolean: '布尔值',
  json: 'JSON 数据'
}

const sourceLabels: Readonly<Record<string, string>> = {
  tenant: '租户覆盖',
  platform: '平台默认',
  code: '代码安全默认'
}

export function settingCategoryLabel(category: string) {
  return categoryLabels[category] ?? category
}

export function settingKeyLabel(category: string, key: string) {
  return keyLabels[`${category}.${key}`] ?? key
}

export function settingValueTypeLabel(valueType: string) {
  return valueTypeLabels[valueType] ?? valueType
}

export function settingSourceLabel(source: string) {
  return sourceLabels[source] ?? source
}
