import {
  createMemoryHistory,
  createRouter,
  createWebHistory,
  type RouterHistory,
  type RouteRecordRaw
} from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { resolveNavigation } from '@/features/navigation/registry'

const platformManagementRoutes: Array<[string, string, string]> = [
  ['users', 'platform-user-management', 'users'],
  ['tenants', 'tenant-management', 'tenants'],
  ['admins', 'platform-admin-management', 'platform-admins'],
  ['resources', 'platform-resource-management', 'resources'],
  ['tenant-features', 'platform-tenant-feature-management', 'tenant-resources'],
  ['settings/providers', 'platform-provider-management', 'providers']
]

const consoleManagementRoutes: Array<[string, string, string]> = [
  ['organization/users', 'member-management', 'members'],
  ['organization/departments', 'department-management', 'departments'],
  ['organization/positions', 'position-management', 'positions'],
  ['permission/policies', 'policy-management', 'casbin-rules'],
  ['logs/login', 'login-logs', 'login-logs'],
  ['logs/audit', 'audit-logs', 'audit-logs'],
  ['logs/api', 'api-logs', 'api-logs'],
  ['logs/exports', 'log-exports', 'log-exports'],
  ['settings/dictionaries', 'dictionary-types', 'dictionary-types'],
  ['settings/dictionary-items', 'dictionary-items', 'dictionary-items']
]

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/console' },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { guestOnly: true, realm: 'tenant' }
  },
  {
    path: '/platform/login',
    name: 'platform-login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { guestOnly: true, realm: 'platform' }
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
    path: '/platform',
    component: () => import('@/layouts/PlatformShell.vue'),
    meta: { requiresAuth: true, realm: 'platform' },
    children: [
      { path: '', redirect: '/platform/tenants' },
      {
        path: 'account',
        name: 'platform-user-center',
        component: () => import('@/views/account/UserCenterView.vue')
      },
      {
        path: 'tenants/:tenantId/setup',
        name: 'tenant-setup',
        component: () => import('@/views/platform/TenantSetupView.vue'),
        props: (route) => ({ tenantId: String(route.params.tenantId) }),
        meta: { platformOnly: true }
      },
      ...platformManagementRoutes.map(([path, name, resourceKey]) => ({
        path,
        name,
        component: () => import('@/views/management/ResourceListView.vue'),
        props: { resourceKey }
      }))
    ]
  },
  {
    path: '/console',
    component: () => import('@/layouts/ConsoleShell.vue'),
    meta: { requiresAuth: true, realm: 'tenant' },
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
        path: 'permission/roles',
        name: 'role-management',
        component: () => import('@/views/permission/RolePermissionView.vue')
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
      ...consoleManagementRoutes.map(([path, name, resourceKey]) => ({
        path,
        name,
        component: () => import('@/views/management/ResourceListView.vue'),
        props: { resourceKey }
      }))
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/console' }
]

function isPlatformContext(authStore: ReturnType<typeof useAuthStore>) {
  return (
    authStore.currentUser?.realm === 'platform' &&
    !authStore.currentUser?.impersonating &&
    Number(authStore.currentTenant?.id ?? 0) === 0
  )
}

function defaultHome(authStore: ReturnType<typeof useAuthStore>) {
  return isPlatformContext(authStore) ? '/platform/tenants' : '/console'
}

export function createAppRouter(mode: 'web' | 'memory' = 'web') {
  const history: RouterHistory = mode === 'memory' ? createMemoryHistory() : createWebHistory()
  const router = createRouter({ history, routes })

  router.beforeEach(async (to) => {
    const authStore = useAuthStore()
    if (!authStore.isAuthenticated && !authStore.sessionRestored) {
      await authStore.restoreSession(to.path)
    }

    if (to.meta.requiresAuth && !authStore.isAuthenticated) {
      const loginName = to.meta.realm === 'platform' ? 'platform-login' : 'login'
      return { name: loginName, query: { redirect: to.fullPath } }
    }

    if (to.meta.guestOnly && authStore.isAuthenticated) {
      return defaultHome(authStore)
    }

    if (to.meta.realm === 'platform' && authStore.isAuthenticated && !isPlatformContext(authStore)) {
      return authStore.currentUser?.impersonating ? '/console' : { name: 'login' }
    }

    if (
      to.meta.realm === 'tenant' &&
      authStore.isAuthenticated &&
      isPlatformContext(authStore) &&
      !to.meta.guestOnly
    ) {
      return '/platform/tenants'
    }

    if (to.meta.platformOnly) {
      const canManageTenants = resolveNavigation(authStore.navigationItems).some(
        (item) => item.to === '/platform/tenants'
      )
      if (!isPlatformContext(authStore) || !canManageTenants) {
        return defaultHome(authStore)
      }
      return
    }

    const resolved = resolveNavigation(authStore.navigationItems)
    if (
      authStore.isAuthenticated &&
      !to.meta.guestOnly &&
      resolved.length > 0 &&
      !['/console', '/console/account', '/platform/account'].includes(to.path)
    ) {
      const allowed = resolved.some((item) => item.to === to.path)
      if (!allowed) return defaultHome(authStore)
    }
  })

  return router
}
