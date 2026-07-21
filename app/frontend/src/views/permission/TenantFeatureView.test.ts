import { mount } from '@vue/test-utils'
import { vi } from 'vitest'

import TenantFeatureView from './TenantFeatureView.vue'

const listResources = vi.hoisted(() =>
  vi.fn((resource: string) => {
    if (resource === 'tenants')
      return Promise.resolve({ items: [{ id: '10', name: '演示租户', code: 'demo' }], total: 1 })
    if (resource === 'resources')
      return Promise.resolve({
        items: [{ id: '9', name: '文件管理', code: 'files', type: 2, scope_mask: 2, status: 1 }],
        total: 1
      })
    return Promise.resolve({ items: [{ id: '99', tenant_id: '10', resource_id: '9' }], total: 1 })
  })
)

vi.mock('@/api/management', () => ({
  listResources,
  updateTenantFeatures: vi.fn()
}))

describe('租户功能授权', () => {
  it('以租户和权限树呈现已有授权', async () => {
    const wrapper = mount(TenantFeatureView, {
      global: {
        directives: { permission: () => undefined, loading: () => undefined },
        stubs: {
          ElButton: { template: '<button><slot /></button>' },
          ElTree: { template: '<div data-testid="feature-tree">文件管理</div>' }
        }
      }
    })
    await vi.waitFor(() => expect(wrapper.text()).toContain('演示租户'))

    expect(wrapper.get('h1').text()).toBe('租户功能授权')
    expect(wrapper.find('[data-testid="feature-tree"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('文件管理')
  })

  it('固定目标租户时不展示租户选择列表', async () => {
    const wrapper = mount(TenantFeatureView, {
      props: { targetTenantId: '10' },
      global: {
        directives: { permission: () => undefined, loading: () => undefined },
        stubs: {
          ElButton: { template: '<button><slot /></button>' },
          ElTree: { template: '<div data-testid="feature-tree">文件管理</div>' }
        }
      }
    })
    await vi.waitFor(() => expect(wrapper.text()).toContain('演示租户'))

    expect(wrapper.find('.tenant-list').exists()).toBe(false)
  })
})
