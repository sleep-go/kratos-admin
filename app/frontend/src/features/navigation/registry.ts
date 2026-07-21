import type { AdminV1NavigationItem } from '@/api/generated'
import type { NavigationItem } from './types'

export interface ResolvedNavigationItem {
  label: string
  to: string
  componentKey: string
  sortOrder: number
}

const componentRouteRegistry: Readonly<Record<string, string>> = {
  dashboard: '/console',
  users: '/platform/users',
  tenants: '/platform/tenants',
  'platform-admins': '/platform/admins',
  members: '/console/organization/users',
  departments: '/console/organization/departments',
  positions: '/console/organization/positions',
  roles: '/console/permission/roles',
  resources: '/platform/resources',
  'tenant-resources': '/platform/tenant-features',
  'casbin-rules': '/console/permission/policies',
  'login-logs': '/console/logs/login',
  'audit-logs': '/console/logs/audit',
  'api-logs': '/console/logs/api',
  'log-exports': '/console/logs/exports',
  files: '/console/files',
  settings: '/console/settings',
  providers: '/platform/settings/providers',
  'dictionary-types': '/console/settings/dictionaries',
  'dictionary-items': '/console/settings/dictionary-items'
}

export function resolveNavigation(items: AdminV1NavigationItem[]): ResolvedNavigationItem[] {
  return items
    .flatMap((item) => {
      const componentKey = item.componentKey ?? ''
      const registeredPath = componentRouteRegistry[componentKey]
      if (!registeredPath || item.routePath !== registeredPath) return []
      return [
        {
          label: item.name || componentKey,
          to: registeredPath,
          componentKey,
          sortOrder: Number(item.sortOrder ?? 0)
        }
      ]
    })
    .sort((left, right) => left.sortOrder - right.sortOrder)
}

export function resolveTopNavigation(
  items: ResolvedNavigationItem[],
  context: 'platform' | 'console' = 'console'
): NavigationItem[] {
  if (context === 'platform') {
    const platformItems = items
      .filter((item) => item.to.startsWith('/platform/'))
      .map((item) => ({ label: item.label, to: item.to }))
    return platformItems.length > 0
      ? [{ label: '平台治理', to: '/platform/tenants', children: platformItems }]
      : []
  }

  const groups = [
    { label: '组织管理', to: '/console/organization/users', prefixes: ['/console/organization/'] },
    { label: '权限中心', to: '/console/permission/roles', prefixes: ['/console/permission/'] },
    { label: '日志中心', to: '/console/logs/audit', prefixes: ['/console/logs/'] },
    { label: '文件管理', to: '/console/files', prefixes: ['/console/files'] },
    { label: '系统设置', to: '/console/settings', prefixes: ['/console/settings'] }
  ]
  return [
    { label: '工作台', to: '/console' },
    ...groups.flatMap((group) => {
      const children = items
        .filter((item) => group.prefixes.some((prefix) => item.to.startsWith(prefix)))
        .map((item) => ({ label: item.label, to: item.to }))
      if (children.length === 0) return []
      return [
        {
          label: group.label,
          to: children.some((item) => item.to === group.to) ? group.to : children[0].to,
          children
        }
      ]
    })
  ]
}
