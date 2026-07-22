import type { AdminV1NavigationItem } from '@/api/generated'
import type { NavigationChildItem, NavigationItem } from './types'

export interface ResolvedNavigationItem {
  label: string
  to: string
  componentKey: string
  sortOrder: number
}

const componentRouteRegistry: Readonly<Record<string, string>> = {
  dashboard: '/console',
  'app-users': '/platform/app-users',
  tenants: '/platform/tenants',
  'platform-admins': '/platform/admins',
  'platform-roles': '/platform/permission/roles',
  members: '/console/organization/users',
  departments: '/console/organization/departments',
  positions: '/console/organization/positions',
  roles: '/console/permission/roles',
  resources: '/platform/resources',
  'casbin-rules': '/console/permission/policies',
  'login-logs': '/console/logs/login',
  'audit-logs': '/console/logs/audit',
  'api-logs': '/console/logs/api',
  'log-exports': '/console/logs/exports',
  'platform-login-logs': '/platform/logs/login',
  'platform-audit-logs': '/platform/logs/audit',
  'platform-api-logs': '/platform/logs/api',
  'platform-log-exports': '/platform/logs/exports',
  files: '/console/files',
  settings: '/console/settings',
  providers: '/platform/settings/providers',
  'dictionary-types': '/console/settings/dictionaries',
  'dictionary-items': '/console/settings/dictionary-items'
}

function sortNavigationItems(items: AdminV1NavigationItem[]): AdminV1NavigationItem[] {
  return [...items].sort((left, right) => {
    const orderDiff = Number(left.sortOrder ?? 0) - Number(right.sortOrder ?? 0)
    if (orderDiff !== 0) return orderDiff
    return String(left.id ?? '').localeCompare(String(right.id ?? ''))
  })
}

function isRootParent(parentId?: string): boolean {
  return !parentId || parentId === '0'
}

function isDirectory(item: AdminV1NavigationItem): boolean {
  return !(item.componentKey ?? '').trim()
}

function resolveLeaf(item: AdminV1NavigationItem): NavigationChildItem | null {
  const componentKey = item.componentKey ?? ''
  if (!componentKey) return null
  const registeredPath = componentRouteRegistry[componentKey]
  if (!registeredPath || item.routePath !== registeredPath) return null
  return { label: item.name || componentKey, to: registeredPath }
}

function collectMenuLeaves(
  items: AdminV1NavigationItem[],
  childrenByParent: Map<string, AdminV1NavigationItem[]>
): NavigationChildItem[] {
  const leaves: NavigationChildItem[] = []
  for (const item of sortNavigationItems(items)) {
    if (isDirectory(item)) {
      const children = childrenByParent.get(String(item.id ?? '')) ?? []
      leaves.push(...collectMenuLeaves(children, childrenByParent))
      continue
    }
    const leaf = resolveLeaf(item)
    if (leaf) leaves.push(leaf)
  }
  return leaves
}

/** 按 parentId 递归组装 DB 菜单树，目录节点 componentKey 为空。 */
export function buildNavigationTree(items: AdminV1NavigationItem[]): NavigationItem[] {
  const childrenByParent = new Map<string, AdminV1NavigationItem[]>()
  for (const item of items) {
    const parentKey = isRootParent(item.parentId) ? '0' : String(item.parentId)
    const bucket = childrenByParent.get(parentKey) ?? []
    bucket.push(item)
    childrenByParent.set(parentKey, bucket)
  }

  const roots = sortNavigationItems(
    childrenByParent.get('0') ?? items.filter((item) => isRootParent(item.parentId))
  )

  return roots.flatMap((item) => {
    if (isDirectory(item)) {
      const children = collectMenuLeaves(childrenByParent.get(String(item.id ?? '')) ?? [], childrenByParent)
      if (children.length === 0) return []
      return [{ label: item.name || item.code || '目录', to: children[0].to, children }]
    }
    const leaf = resolveLeaf(item)
    return leaf ? [{ label: leaf.label, to: leaf.to }] : []
  })
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

/** @deprecated 请改用 buildNavigationTree(authStore.navigationItems) */
export function resolveTopNavigation(
  items: AdminV1NavigationItem[],
  _context: 'platform' | 'console' = 'console'
): NavigationItem[] {
  return buildNavigationTree(items)
}
