import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

import RolePermissionView from './RolePermissionView.vue'

const listResources = vi.hoisted(() =>
  vi.fn((resource: string) => {
    if (resource === 'roles')
      return Promise.resolve({
        items: [{ id: '1', name: '管理员', code: 'admin', data_scope: 1 }],
        total: 1
      })
    if (resource === 'resources')
      return Promise.resolve({
        items: [{ id: '9', name: '文件管理', code: 'files', type: 2, scope_mask: 2, status: 1 }],
        total: 1
      })
    return Promise.resolve({ items: [], total: 0 })
  })
)

vi.mock('@/api/management', () => ({
  listResources,
  updateResource: vi.fn(),
  updateRoleAuthorization: vi.fn(),
  createResource: vi.fn(),
  deleteResource: vi.fn()
}))

describe('角色授权', () => {
  it('展示角色与资源动作权限配置', async () => {
    const wrapper = mount(RolePermissionView, {
      global: {
        plugins: [createTestingPinia({ createSpy: vi.fn })],
        directives: { permission: () => undefined, loading: () => undefined },
        stubs: {
          ElButton: { template: '<button><slot /></button>' },
          ElCheckbox: { template: '<label><slot /></label>' },
          ElCheckboxGroup: { template: '<div><slot /></div>' }
        }
      }
    })
    await vi.waitFor(() => expect(wrapper.text()).toContain('管理员'))

    expect(wrapper.get('h1').text()).toBe('角色授权')
    expect(wrapper.text()).toContain('文件管理')
    expect(wrapper.text()).not.toContain('本部门及下级')
    expect(wrapper.text()).toContain('新建角色')
  })

  it('为平台初始化页按目标租户加载角色和授权', async () => {
    mount(RolePermissionView, {
      props: { targetTenantId: '8' },
      global: {
        plugins: [createTestingPinia({ createSpy: vi.fn })],
        directives: { permission: () => undefined, loading: () => undefined },
        stubs: {
          ElButton: { template: '<button><slot /></button>' },
          ElCheckbox: { template: '<label><slot /></label>' },
          ElCheckboxGroup: { template: '<div><slot /></div>' }
        }
      }
    })

    await vi.waitFor(() =>
      expect(listResources).toHaveBeenCalledWith(
        'roles',
        expect.objectContaining({ page: 1, page_size: 200 }),
        { targetTenantId: '8' }
      )
    )
  })
})
