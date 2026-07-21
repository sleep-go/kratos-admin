import { flushPromises, mount } from '@vue/test-utils'
import { vi } from 'vitest'

import TenantSetupView from './TenantSetupView.vue'

vi.mock('@/api/management', () => ({
  listResources: vi.fn().mockResolvedValue({
    items: [{ id: '8', code: 'pending', name: '待开通租户', status: 1 }],
    total: 1
  })
}))

describe('租户开通', () => {
  it('按路由租户展示功能、角色与租户管理员开通入口', async () => {
    const wrapper = mount(TenantSetupView, {
      props: { tenantId: '8' },
      global: {
        stubs: {
          TenantFeatureView: {
            props: ['targetTenantId'],
            template: '<div data-testid="features" />'
          },
          RolePermissionView: {
            props: ['targetTenantId'],
            template: '<div data-testid="roles" />'
          },
          ResourceListView: {
            props: ['resourceKey', 'targetTenantId'],
            template: '<div />'
          },
          ElTabs: { template: '<div><slot /></div>' },
          ElTabPane: {
            props: ['label'],
            template: '<section data-testid="setup-tab">{{ label }}<slot /></section>'
          },
          RouterLink: { template: '<a><slot /></a>' }
        }
      }
    })
    await flushPromises()

    expect(wrapper.get('h1').text()).toContain('待开通租户')
    expect(wrapper.text()).toContain('目标租户 ID 8')
    expect(wrapper.findAll('[data-testid="setup-tab"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('功能授权')
    expect(wrapper.text()).toContain('角色管理')
    expect(wrapper.text()).toContain('租户管理员')
  })
})
