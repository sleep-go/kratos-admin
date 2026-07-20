import {
  createMemoryHistory,
  createRouter,
  createWebHistory,
  type RouterHistory,
  type RouteRecordRaw
} from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { resolveNavigation } from '@/features/navigation/registry'

const managementRoutes: Array<[string, string, string]> = [
  ['platform/users', 'user-management', 'users'],
  ['platform/tenants', 'tenant-management', 'tenants'],
  ['organization/users', 'member-management', 'members'],
  ['organization/departments', 'department-management', 'departments'],
  ['organization/positions', 'position-management', 'positions'],
  ['permission/resources', 'resource-management', 'resources'],
  ['permission/policies', 'policy-management', 'casbin-rules'],
  ['logs/login', 'login-logs', 'login-logs'],
  ['logs/audit', 'audit-logs', 'audit-logs'],
  ['logs/api', 'api-logs', 'api-logs'],
  ['logs/exports', 'log-exports', 'log-exports'],
  ['settings/dictionaries', 'dictionary-types', 'dictionary-types'],
  ['settings/dictionary-items', 'dictionary-items', 'dictionary-items']
]

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { guestOnly: true }
  },
  {
    path: '/mfa',
    name: 'mfa',
    component: () => import('@/views/auth/MFAView.vue'),
    meta: { guestOnly: true }
  },
  {
    path: '/forgot-password',
    name: 'forgot-password',
    component: () => import('@/views/auth/PasswordRecoveryView.vue'),
    meta: { guestOnly: true }
  },
  {
    path: '/',
    component: () => import('@/layouts/AppShell.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('@/views/dashboard/DashboardView.vue')
      },
      {
        path: 'files',
        name: 'file-management',
        component: () => import('@/views/files/FileManagementView.vue')
      },
      {
        path: 'settings/providers',
        name: 'provider-management',
        component: () => import('@/views/settings/ProviderManagementView.vue')
      },
      {
        path: 'permission/roles',
        name: 'role-management',
        component: () => import('@/views/permission/RolePermissionView.vue')
      },
      {
        path: 'permission/tenant-features',
        name: 'tenant-resource-management',
        component: () => import('@/views/permission/TenantFeatureView.vue')
      },
      {
        path: 'settings',
        name: 'system-settings',
        component: () => import('@/views/settings/SystemSettingsView.vue')
      },
      {
        path: 'account',
        name: 'user-center',
        component: () => import('@/views/account/UserCenterView.vue')
      },
      ...managementRoutes.map(([path, name, resourceKey]) => ({
        path,
        name,
        component: () => import('@/views/management/ResourceListView.vue'),
        props: { resourceKey }
      }))
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

export function createAppRouter(mode: 'web' | 'memory' = 'web') {
  const history: RouterHistory = mode === 'memory' ? createMemoryHistory() : createWebHistory()
  const router = createRouter({ history, routes })

  router.beforeEach(async (to) => {
    const authStore = useAuthStore()
    if (!authStore.isAuthenticated && !authStore.sessionRestored) {
      await authStore.restoreSession()
    }
    if (to.meta.requiresAuth && !authStore.isAuthenticated) {
      return { name: 'login', query: { redirect: to.fullPath } }
    }
    if (to.meta.guestOnly && authStore.isAuthenticated) {
      return { name: 'dashboard' }
    }
    if (authStore.isAuthenticated && !to.meta.guestOnly && !['/', '/account'].includes(to.path)) {
      const allowed = resolveNavigation(authStore.navigationItems).some(
        (item) => item.to === to.path
      )
      if (!allowed) return { name: 'dashboard' }
    }
  })

  return router
}
