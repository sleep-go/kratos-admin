import { flushPromises, mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

import UserCenterView from './UserCenterView.vue'

vi.mock('@/api/auth', () => ({
  listSessions: vi.fn().mockResolvedValue({
    items: [{ id: 'current', deviceName: 'Chrome on macOS', ip: '127.0.0.1', current: true }]
  }),
  platformListSessions: vi.fn().mockResolvedValue({ items: [] }),
  revokeSession: vi.fn(),
  platformRevokeSession: vi.fn(),
  updateProfile: vi.fn()
}))

describe('个人中心', () => {
  it('展示真实账号资料和设备会话', async () => {
    const wrapper = mount(UserCenterView, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              auth: {
                currentUser: {
                  id: '8',
                  username: 'admin',
                  displayName: '超级管理员',
                  email: 'admin@example.com',
                  realm: 'tenant'
                },
                currentTenant: { id: '10', name: '演示租户' }
              }
            }
          })
        ],
        stubs: {
          ElButton: { template: '<button><slot /></button>' },
          ElInput: { template: '<input />' },
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<label><slot /></label>' },
          RouterLink: { template: '<a><slot /></a>' }
        }
      }
    })
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('个人中心')
    expect(wrapper.text()).toContain('admin@example.com')
    expect(wrapper.text()).toContain('Chrome on macOS')
    expect(wrapper.text()).toContain('当前设备')
  })
})
