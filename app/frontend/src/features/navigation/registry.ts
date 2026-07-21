import type { AdminV1NavigationItem } from '@/api/generated'
import type { NavigationItem } from './types'

export interface ResolvedNavigationItem {
  label: string
  to: string
  componentKey: string
  sortOrder: number
}

const componentRouteRegistry: Readonly<Record<string, string>> = {
  dashboard: '/',
  users: '/platform/users',
  tenants: '/platform/tenants',
  members: '/organization/users',
  departments: '/organization/departments',
  positions: '/organization/positions',
  roles: '/permission/roles',
  resources: '/permission/resources',
  'tenant-resources': '/permission/tenant-features',
  'casbin-rules': '/permission/policies',
  'login-logs': '/logs/login',
  'audit-logs': '/logs/audit',
  'api-logs': '/logs/api',
  'log-exports': '/logs/exports',
  files: '/files',
  settings: '/settings',
  providers: '/settings/providers',
  'dictionary-types': '/settings/dictionaries',
  'dictionary-items': '/settings/dictionary-items'
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

export function resolveTopNavigation(items: ResolvedNavigationItem[]): NavigationItem[] {
  const groups = [
    { label: '平台管理', to: '/platform/tenants', prefixes: ['/platform/'] },
    { label: '组织管理', to: '/organization/users', prefixes: ['/organization/'] },
    { label: '权限中心', to: '/permission/roles', prefixes: ['/permission/'] },
    { label: '日志中心', to: '/logs/audit', prefixes: ['/logs/'] },
    { label: '文件管理', to: '/files', prefixes: ['/files'] },
    { label: '系统设置', to: '/settings', prefixes: ['/settings'] }
  ]
  return [
    { label: '工作台', to: '/' },
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
