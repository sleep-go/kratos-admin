import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import AppShell from './AppShell.vue'

describe('AppShell', () => {
  it('展示顶部主导航和当前租户', () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }]
    })
    const wrapper = mount(AppShell, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              auth: {
                currentTenant: { id: '10', name: '示例租户' },
                navigationItems: [
                  { name: '租户管理', routePath: '/platform/tenants', componentKey: 'tenants' },
                  { name: '成员管理', routePath: '/organization/users', componentKey: 'members' },
                  { name: '角色管理', routePath: '/permission/roles', componentKey: 'roles' },
                  { name: '审计日志', routePath: '/logs/audit', componentKey: 'audit-logs' },
                  { name: '文件管理', routePath: '/files', componentKey: 'files' },
                  { name: '系统设置', routePath: '/settings', componentKey: 'settings' }
                ]
              }
            }
          }),
          router
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
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }]
    })
    const wrapper = mount(AppShell, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              auth: { currentTenant: { id: '10', name: '示例租户' }, navigationItems: [] }
            }
          }),
          router
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

  it('移动端可以从导航抽屉打开租户切换', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }]
    })
    const wrapper = mount(AppShell, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              auth: { currentTenant: { id: '10', name: '示例租户' }, navigationItems: [] }
            }
          }),
          router
        ],
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          RouterView: { template: '<main>页面内容</main>' }
        }
      }
    })

    await wrapper.get('[data-testid="mobile-menu-button"]').trigger('click')
    await wrapper.get('[data-testid="mobile-tenant-switcher"]').trigger('click')

    expect(wrapper.get('[role="dialog"]').attributes('aria-label')).toBe('切换租户')
    expect(wrapper.get('[data-testid="mobile-navigation"]').attributes('aria-hidden')).toBe('true')
  })
})
