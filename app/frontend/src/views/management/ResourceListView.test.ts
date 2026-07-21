import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ResourceListView from './ResourceListView.vue'

const listResources = vi.hoisted(() =>
  vi.fn((resource: string) => {
    if (resource === 'users') {
      return Promise.resolve({
        items: [{ id: '1', username: 'admin', display_name: '管理员', status: 1 }],
        total: 1
      })
    }
    if (resource === 'tenants') {
      return Promise.resolve({
        items: [{ id: '1', code: 'demo', name: '演示租户', status: 1 }],
        total: 1
      })
    }
    return Promise.resolve({ items: [], total: 0 })
  })
)
const createResource = vi.hoisted(() => vi.fn())

vi.mock('@/api/management', () => ({
  listResources,
  createResource,
  updateResource: vi.fn(),
  deleteResource: vi.fn()
}))

vi.mock('@/api/logs', () => ({
  createLogExport: vi.fn(),
  getLogExport: vi.fn(),
  getLogExportDownloadURL: vi.fn()
}))

function mountView(resourceKey = 'users', targetTenantId?: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }]
  })
  return mount(ResourceListView, {
    props: { resourceKey, targetTenantId },
    global: {
      plugins: [createPinia(), router],
      directives: { permission: () => undefined, loading: () => undefined },
      stubs: {
        ResourceFormField: {
          props: ['field', 'options', 'modelValue'],
          emits: ['update:modelValue'],
          template:
            '<button type="button" data-testid="resource-form-field" :data-field="field.key" :data-options="options?.length ?? 0" @click="$emit(\'update:modelValue\', field.key === \'ptype\' ? \'g\' : modelValue)" />'
        },
        ElDrawer: {
          props: ['modelValue', 'direction', 'size', 'title'],
          template: `
            <aside
              data-testid="resource-form-drawer"
              :data-open="String(modelValue)"
              :data-direction="direction"
              :data-size="size"
              :data-title="title"
            >
              <slot />
              <slot name="footer" />
            </aside>
          `
        },
        ElButton: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
        ElForm: { template: '<form><slot /></form>' },
        ElFormItem: { template: '<label><slot /></label>' },
        ElInput: { template: '<input />' },
        ElTable: { template: '<div><slot /></div>' },
        ElTableColumn: { template: '<div />' },
        ElPagination: true,
        RouterLink: { template: '<a><slot /></a>' }
      }
    }
  })
}

describe('通用数据管理表单', () => {
  beforeEach(() => {
    listResources.mockClear()
    createResource.mockClear()
  })

  it('通过右侧抽屉展示新增表单', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.heading-actions button').trigger('click')

    const drawer = wrapper.get('[data-testid="resource-form-drawer"]')
    expect(drawer.attributes('data-open')).toBe('true')
    expect(drawer.attributes('data-direction')).toBe('rtl')
    expect(drawer.attributes('data-size')).toBe('min(560px, 100%)')
    expect(drawer.text()).toContain('保存')
  })

  it('通过同一个右侧抽屉展示编辑表单', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.mobile-cards article button').trigger('click')

    const drawer = wrapper.get('[data-testid="resource-form-drawer"]')
    expect(drawer.attributes('data-open')).toBe('true')
    expect(drawer.attributes('data-title')).toBe('编辑全局用户')
  })

  it('租户列表提供独立初始化入口', async () => {
    const wrapper = mountView('tenants')
    await flushPromises()

    expect(wrapper.get('[data-testid="tenant-setup-1"]').attributes('to')).toBe(
      '/platform/tenants/1/setup'
    )
  })

  it('目标租户成员管理通过查询上下文调用接口且全局用户候选不带租户', async () => {
    const wrapper = mountView('members', '8')
    await flushPromises()

    expect(listResources).toHaveBeenCalledWith(
      'members',
      expect.objectContaining({ page: 1, page_size: 20 }),
      { targetTenantId: '8' }
    )
    await wrapper.get('.heading-actions button').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledWith(
      'users',
      { page: 1, page_size: 200, sort: 'id:asc' },
      undefined
    )
    expect(listResources).toHaveBeenCalledWith(
      'members',
      { page: 1, page_size: 200, sort: 'id:asc' },
      { targetTenantId: '8' }
    )
  })

  it('切换Casbin策略类型时重新加载对应关联候选项', async () => {
    const wrapper = mountView('casbin-rules')
    await flushPromises()

    await wrapper.get('.heading-actions > button').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledWith(
      'resources',
      { page: 1, page_size: 200, sort: 'id:asc' },
      undefined
    )

    await wrapper.get('[data-field="ptype"]').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledWith(
      'members',
      { page: 1, page_size: 200, sort: 'id:asc' },
      undefined
    )
  })

  it('必填关联或枚举未选择时不提交新增请求', async () => {
    const wrapper = mountView('resources')
    await flushPromises()
    await wrapper.get('.heading-actions > button').trigger('click')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((button) => button.text() === '保存')
    await saveButton?.trigger('click')

    expect(createResource).not.toHaveBeenCalled()
  })
})
