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
                tenants: [
                  { id: '10', name: '示例租户' },
                  { id: '11', name: '备用租户' }
                ],
                navigationItems: [
                  { name: '成员管理', routePath: '/console/organization/users', componentKey: 'members' },
                  { name: '角色管理', routePath: '/console/permission/roles', componentKey: 'roles' },
                  { name: '审计日志', routePath: '/console/logs/audit', componentKey: 'audit-logs' },
                  { name: '文件管理', routePath: '/console/files', componentKey: 'files' },
                  { name: '系统设置', routePath: '/console/settings', componentKey: 'settings' }
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
              auth: {
                currentTenant: { id: '10', name: '示例租户' },
                tenants: [
                  { id: '10', name: '示例租户' },
                  { id: '11', name: '备用租户' }
                ],
                navigationItems: []
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
              auth: {
                currentTenant: { id: '10', name: '示例租户' },
                tenants: [
                  { id: '10', name: '示例租户' },
                  { id: '11', name: '备用租户' }
                ],
                navigationItems: []
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

    await wrapper.get('[data-testid="mobile-menu-button"]').trigger('click')
    await wrapper.get('[data-testid="mobile-tenant-switcher"]').trigger('click')

    expect(wrapper.get('[role="dialog"]').attributes('aria-label')).toBe('切换租户')
    expect(wrapper.get('[data-testid="mobile-navigation"]').attributes('aria-hidden')).toBe('true')
  })

  it('移动端按分组展示所有已授权日志入口', async () => {
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
                currentTenant: { id: '0', name: '平台管理' },
                navigationItems: [
                  { name: '登录日志', routePath: '/console/logs/login', componentKey: 'login-logs' },
                  { name: '操作审计', routePath: '/console/logs/audit', componentKey: 'audit-logs' },
                  { name: 'API 日志', routePath: '/console/logs/api', componentKey: 'api-logs' },
                  { name: '导出记录', routePath: '/console/logs/exports', componentKey: 'log-exports' }
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

    await wrapper.get('[data-testid="mobile-menu-button"]').trigger('click')

    const navigation = wrapper.get('[data-testid="mobile-navigation"]')
    expect(navigation.text()).toContain('日志中心')
    expect(navigation.text()).toContain('登录日志')
    expect(navigation.text()).toContain('操作审计')
    expect(navigation.text()).toContain('API 日志')
    expect(navigation.text()).toContain('导出记录')
  })
})
