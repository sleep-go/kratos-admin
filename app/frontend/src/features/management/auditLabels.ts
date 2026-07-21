const actionLabels: Readonly<Record<string, string>> = {
  create: '创建',
  update: '更新',
  delete: '删除',
  update_authorization: '更新授权',
  update_features: '更新功能权限',
  'create-upload': '创建文件上传',
  confirm: '确认文件',
  'request-delete': '请求删除'
}

const resourceLabels: Readonly<Record<string, string>> = {
  users: '用户',
  tenants: '租户',
  members: '成员',
  departments: '部门',
  positions: '岗位',
  roles: '角色',
  resources: '菜单与权限资源',
  'tenant-resources': '租户功能授权',
  'casbin-rules': '权限策略',
  'role-scope-departments': '角色部门数据范围',
  settings: '系统设置',
  'dictionary-types': '参数字典',
  'dictionary-items': '字典项',
  providers: '渠道配置',
  files: '文件'
}

export const auditActionOptions = Object.entries(actionLabels).map(([value, label]) => ({
  label,
  value
}))

export const auditResourceOptions = Object.entries(resourceLabels).map(([value, label]) => ({
  label,
  value
}))

export function auditActionLabel(value: unknown) {
  const key = String(value ?? '')
  return actionLabels[key] ?? key
}

export function auditResourceLabel(value: unknown) {
  const key = String(value ?? '')
  return resourceLabels[key] ?? key
}

export function auditSummaryLabel(
  summary: unknown,
  action: unknown,
  resourceType: unknown,
  resourceID: unknown
) {
  const rawSummary = String(summary ?? '').trim()
  const rawAction = String(action ?? '').trim()
  const rawResourceType = String(resourceType ?? '').trim()
  const rawResourceID = String(resourceID ?? '').trim()
  const generatedSummary = [rawAction, rawResourceType, rawResourceID].filter(Boolean).join(' ')
  if (rawSummary && rawSummary !== generatedSummary) return rawSummary

  const actionLabel = auditActionLabel(rawAction)
  const resourceLabel = auditResourceLabel(rawResourceType)
  if (actionLabel === rawAction && resourceLabel === rawResourceType)
    return rawSummary || generatedSummary
  const subject = actionLabel.includes(resourceLabel)
    ? actionLabel
    : `${actionLabel}${resourceLabel}`
  return `${subject}${rawResourceID ? ` #${rawResourceID}` : ''}`
}
