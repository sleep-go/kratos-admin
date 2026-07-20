import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

import AppShell from './AppShell.vue'

describe('AppShell', () => {
  it('展示顶部主导航和当前租户', () => {
    const wrapper = mount(AppShell, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: { auth: { currentTenant: { id: '10', name: '示例租户' } } }
          })
        ],
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          RouterView: { template: '<main>页面内容</main>' }
        }
      }
    })

    expect(wrapper.get('[data-testid="brand"]').text()).toContain('KRATOS')
    expect(wrapper.get('[data-testid="tenant-switcher"]').text()).toContain('示例租户')
    expect(wrapper.text()).toContain('工作台')
    expect(wrapper.text()).toContain('组织管理')
    expect(wrapper.text()).toContain('权限中心')
    expect(wrapper.text()).toContain('日志中心')
    expect(wrapper.text()).toContain('系统设置')
  })

  it('在移动端按钮点击后打开导航抽屉', async () => {
    const wrapper = mount(AppShell, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: { auth: { currentTenant: { id: '10', name: '示例租户' } } }
          })
        ],
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          RouterView: { template: '<main>页面内容</main>' }
        }
      }
    })

    await wrapper.get('[data-testid="mobile-menu-button"]').trigger('click')

    expect(wrapper.get('[data-testid="mobile-navigation"]').attributes('aria-hidden')).toBe('false')
  })
})
