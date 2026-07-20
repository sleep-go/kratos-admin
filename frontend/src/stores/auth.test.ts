import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as authApi from '@/api/auth'
import { useAuthStore } from './auth'

vi.mock('@/api/auth', () => ({
  getCaptcha: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  refresh: vi.fn(),
  verifyMfa: vi.fn(),
  switchTenant: vi.fn()
}))

describe('认证状态恢复', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('切换租户后再次刷新服务端权限上下文', async () => {
    vi.mocked(authApi.switchTenant).mockResolvedValue({
      accessToken: 'switched-token',
      currentTenant: { id: '11', name: '租户甲' }
    })
    vi.mocked(authApi.refresh).mockResolvedValue({
      accessToken: 'tenant-token',
      user: {
        id: '8',
        displayName: '租户管理员',
        platformAdmin: true,
        permissions: ['files:*']
      },
      tenants: [{ id: '11', name: '租户甲' }],
      currentTenant: { id: '11', name: '租户甲' }
    })
    const store = useAuthStore()

    await store.switchTenant('11')

    expect(store.currentTenant?.id).toBe('11')
    expect(store.currentUser?.permissions).toEqual(['files:*'])
  })

  it('使用 HttpOnly refresh cookie 恢复用户与租户上下文', async () => {
    vi.mocked(authApi.refresh).mockResolvedValue({
      accessToken: 'renewed-access-token',
      user: { id: '8', displayName: '恢复用户', platformAdmin: true },
      tenants: [{ id: '11', name: '租户甲' }],
      currentTenant: { id: '0', name: '平台管理' }
    })
    const store = useAuthStore()

    const restored = await store.restoreSession()

    expect(restored).toBe(true)
    expect(store.isAuthenticated).toBe(true)
    expect(store.currentUser?.displayName).toBe('恢复用户')
    expect(store.currentTenant?.name).toBe('平台管理')
  })

  it('refresh cookie 无效时保持访客状态', async () => {
    vi.mocked(authApi.refresh).mockRejectedValue(new Error('unauthorized'))
    const store = useAuthStore()

    await expect(store.restoreSession()).resolves.toBe(false)
    expect(store.isAuthenticated).toBe(false)
  })
})
