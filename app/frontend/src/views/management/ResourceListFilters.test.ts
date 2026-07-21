import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import * as logApi from '@/api/logs'
import * as managementApi from '@/api/management'
import ResourceListView from './ResourceListView.vue'

vi.mock('@/api/management', () => ({
  listResources: vi.fn().mockResolvedValue({ items: [], total: 0 }),
  createResource: vi.fn(),
  updateResource: vi.fn(),
  deleteResource: vi.fn()
}))

vi.mock('@/api/logs', () => ({
  createLogExport: vi.fn().mockResolvedValue({ id: 'export-1', status: 3 }),
  getLogExport: vi.fn(),
  getLogExportDownloadURL: vi.fn()
}))

function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }]
  })
  return mount(ResourceListView, {
    props: { resourceKey: 'login-logs' },
    global: {
      plugins: [createPinia(), router],
      directives: { permission: () => undefined, loading: () => undefined },
      stubs: {
        ElButton: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
        ElInput: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        ElSelect: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template:
            '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><slot /></select>'
        },
        ElOption: {
          props: ['label', 'value'],
          template: '<option :value="value">{{ label }}</option>'
        },
        ElTable: { template: '<div><slot /></div>' },
        ElTableColumn: { template: '<div />' },
        ElPagination: true,
        ElDrawer: true,
        ElDialog: true,
        RouterLink: { template: '<a><slot /></a>' }
      }
    }
  })
}

beforeEach(() => {
  vi.mocked(managementApi.listResources).mockClear()
  vi.mocked(logApi.createLogExport).mockClear()
})

describe('日志结构化筛选', () => {
  it('查询和导出使用相同的非空筛选条件', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="filter-result"]').setValue('2')
    await wrapper.get('[data-testid="query-button"]').trigger('click')
    await flushPromises()

    expect(managementApi.listResources).toHaveBeenLastCalledWith(
      'login-logs',
      expect.objectContaining({ filters: { result: '2' } }),
      undefined
    )

    await wrapper.get('[data-testid="export-button"]').trigger('click')
    await flushPromises()

    expect(logApi.createLogExport).toHaveBeenCalledWith(
      expect.objectContaining({ logType: 'login', filters: { result: '2' } })
    )
  })

  it('重置时清空关键词和全部结构化筛选', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="filter-result"]').setValue('3')
    await wrapper.get('[data-testid="reset-button"]').trigger('click')
    await flushPromises()

    expect(managementApi.listResources).toHaveBeenLastCalledWith(
      'login-logs',
      expect.objectContaining({ keyword: '', filters: {} }),
      undefined
    )
  })
})
