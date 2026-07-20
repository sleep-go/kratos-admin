import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

import FileManagementView from './FileManagementView.vue'

vi.mock('@/api/management', () => ({
  listResources: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 })
}))

describe('文件管理', () => {
  it('展示真实直传入口和文件状态列表', () => {
    const wrapper = mount(FileManagementView, {
      global: {
        plugins: [createTestingPinia({ createSpy: vi.fn })],
        directives: { permission: () => undefined },
        stubs: {
          ElButton: { template: '<button><slot /></button>' },
          ElProgress: { template: '<div />' },
          ElTable: { template: '<div><slot /></div>' },
          ElTableColumn: { template: '<div />' },
          ElPagination: { template: '<div />' }
        }
      }
    })

    expect(wrapper.get('h1').text()).toBe('文件管理')
    expect(wrapper.find('input[type="file"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('直传')
  })
})
