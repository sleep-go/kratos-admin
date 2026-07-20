import {
  createMemoryHistory,
  createRouter,
  createWebHistory,
  type RouterHistory,
  type RouteRecordRaw
} from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const managementRoutes: Array<[string, string, string]> = [
  ['platform/users', 'user-management', 'users'],
  ['platform/tenants', 'tenant-management', 'tenants'],
  ['organization/users', 'member-management', 'members'],
  ['organization/departments', 'department-management', 'departments'],
  ['organization/positions', 'position-management', 'positions'],
  ['permission/roles', 'role-management', 'roles'],
  ['permission/resources', 'resource-management', 'resources'],
  ['permission/tenant-features', 'tenant-resource-management', 'tenant-resources'],
  ['permission/policies', 'policy-management', 'casbin-rules'],
  ['logs/login', 'login-logs', 'login-logs'],
  ['logs/audit', 'audit-logs', 'audit-logs'],
  ['logs/api', 'api-logs', 'api-logs'],
  ['settings/dictionaries', 'dictionary-types', 'dictionary-types'],
  ['settings/dictionary-items', 'dictionary-items', 'dictionary-items'],
  ['settings/providers', 'provider-management', 'providers'],
  ['settings', 'system-settings', 'settings']
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
  })

  return router
}
