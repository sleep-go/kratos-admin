import { flushPromises, mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { describe, expect, it, vi } from 'vitest'

import DashboardView from './DashboardView.vue'

vi.mock('@/api/management', () => ({
  listResources: vi.fn((resource: string) => {
    if (resource === 'members') return Promise.resolve({ items: [], total: 18 })
    if (resource === 'roles') return Promise.resolve({ items: [], total: 6 })
    if (resource === 'api-logs')
      return Promise.resolve({
        total: 42,
        items: [{ id: '1', status_code: 500, created_at: '2026-07-20T08:00:00Z' }]
      })
    return Promise.resolve({
      items: [{ id: '2', summary: '角色权限已更新', created_at: '2026-07-20T09:00:00Z' }],
      total: 1
    })
  })
}))

describe('DashboardView', () => {
  it('通过真实管理接口呈现核心指标和安全动态', async () => {
    const wrapper = mount(DashboardView, {
      global: { plugins: [createTestingPinia({ createSpy: vi.fn })] }
    })

    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('工作台')
    expect(wrapper.text()).toContain('成员总数')
    expect(wrapper.text()).toContain('请求趋势')
    expect(wrapper.text()).toContain('安全动态')
    expect(wrapper.text()).toContain('18')
    expect(wrapper.text()).toContain('角色权限已更新')
  })
})
