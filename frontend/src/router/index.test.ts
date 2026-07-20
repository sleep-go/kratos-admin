import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppRouter } from './index'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  logout: vi.fn(),
  refresh: vi.fn().mockRejectedValue(new Error('unauthorized'))
}))

describe('路由鉴权', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.restoreAllMocks()
  })

  it('未登录访问工作台时跳转到登录页', async () => {
    const router = createAppRouter('memory')
    await router.push('/')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('已登录用户不能重复进入登录页', async () => {
    const authStore = useAuthStore()
    authStore.accessToken = 'token'
    authStore.currentUser = { id: '1', displayName: '超级管理员', platformAdmin: true }
    const router = createAppRouter('memory')
    await router.push('/login')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  it('拒绝访问未出现在服务端菜单中的编译期页面', async () => {
    const authStore = useAuthStore()
    authStore.accessToken = 'token'
    authStore.sessionRestored = true
    authStore.currentUser = { id: '1', displayName: '普通用户', platformAdmin: false }
    authStore.navigationItems = [{ name: '文件管理', routePath: '/files', componentKey: 'files' }]
    const router = createAppRouter('memory')

    await router.push('/permission/roles')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('dashboard')
  })
})
