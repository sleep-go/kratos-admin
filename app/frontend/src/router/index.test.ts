import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppRouter } from './index'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  logout: vi.fn(),
  refresh: vi.fn().mockRejectedValue(new Error('unauthorized')),
  platformRefresh: vi.fn().mockRejectedValue(new Error('unauthorized'))
}))

vi.mock('@/views/dashboard/DashboardView.vue', () => ({
  default: { template: '<div>工作台</div>' }
}))

describe('路由鉴权', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.restoreAllMocks()
  })

  it('未登录访问工作台时跳转到登录页', async () => {
    const router = createAppRouter('memory')
    await router.push('/console')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('已登录平台用户不能重复进入租户登录页', async () => {
    const authStore = useAuthStore()
    authStore.accessToken = 'token'
    authStore.sessionRestored = true
    authStore.currentUser = { id: '1', displayName: '超级管理员', realm: 'platform' }
    authStore.currentTenant = { id: '0', name: '平台管理' }
    const router = createAppRouter('memory')
    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.path).toBe('/platform/tenants')
  })

  it('拒绝访问未出现在服务端菜单中的编译期页面', async () => {
    const authStore = useAuthStore()
    authStore.accessToken = 'token'
    authStore.sessionRestored = true
    authStore.currentUser = { id: '1', displayName: '普通用户', realm: 'tenant' }
    authStore.navigationItems = [
      { name: '文件管理', routePath: '/console/files', componentKey: 'files' }
    ]
    const router = createAppRouter('memory')

    await router.push('/console/permission/roles')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('仅允许平台上下文管理员从租户菜单进入开通配置页', async () => {
    const authStore = useAuthStore()
    authStore.accessToken = 'token'
    authStore.sessionRestored = true
    authStore.currentUser = { id: '1', displayName: '平台管理员', realm: 'platform' }
    authStore.currentTenant = { id: '0', name: '平台' }
    authStore.navigationItems = [
      { name: '租户管理', routePath: '/platform/tenants', componentKey: 'tenants' }
    ]
    const router = createAppRouter('memory')

    await router.push('/platform/tenants/8/setup')
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('tenant-setup')

    authStore.currentTenant = { id: '8', name: '演示租户' }
    await router.push('/platform/tenants/9/setup')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })
})
