import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ResourceListView from './ResourceListView.vue'

const listResources = vi.hoisted(() => vi.fn())
const createResource = vi.hoisted(() => vi.fn())

function mockListResources(resource: string) {
  if (resource === 'app-users') {
    return Promise.resolve({
      items: [{ id: '1', username: 'app-user', display_name: 'App 用户', status: 1 }],
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
}

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

function mountView(resourceKey = 'app-users', targetTenantId?: string) {
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
        ElTabs: {
          props: ['modelValue'],
          emits: ['tab-change', 'update:modelValue'],
          template: `
            <div data-testid="resource-scope-tabs">
              <button
                data-tab="all"
                type="button"
                @click="$emit('update:modelValue', 'all'); $emit('tab-change', 'all')"
              >
                全部
              </button>
              <button
                data-tab="platform"
                type="button"
                @click="$emit('update:modelValue', 'platform'); $emit('tab-change', 'platform')"
              >
                平台资源
              </button>
              <button
                data-tab="tenant"
                type="button"
                @click="$emit('update:modelValue', 'tenant'); $emit('tab-change', 'tenant')"
              >
                租户资源
              </button>
            </div>
          `
        },
        ElTabPane: { template: '<div><slot /></div>' },
        RouterLink: { template: '<a><slot /></a>' }
      }
    }
  })
}

describe('通用数据管理表单', () => {
  beforeEach(() => {
    listResources.mockReset()
    listResources.mockImplementation(mockListResources)
    createResource.mockClear()
  })

  it('平台管理员列表可正常加载', async () => {
    const wrapper = mountView('platform-admins')
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('平台管理员')
    expect(listResources).toHaveBeenCalledWith(
      'platform-admins',
      expect.objectContaining({ page: 1, page_size: 20 }),
      undefined
    )
  })

  it('未知资源键展示占位提示且不请求接口', async () => {
    const wrapper = mountView('unknown-resource')
    await flushPromises()

    expect(wrapper.text()).toContain('未找到资源定义：unknown-resource')
    expect(listResources).not.toHaveBeenCalled()
  })

  it('菜单与权限资源按平台与租户分开展示', async () => {
    const wrapper = mountView('resources')
    await flushPromises()

    expect(wrapper.get('[data-testid="resource-scope-tabs"]').text()).toContain('全部')
    expect(wrapper.get('[data-testid="resource-scope-tabs"]').text()).toContain('平台资源')
    expect(wrapper.get('[data-testid="resource-scope-tabs"]').text()).toContain('租户资源')
    expect(listResources).toHaveBeenCalledWith(
      'resources',
      expect.objectContaining({
        page: 1,
        page_size: 500,
        sort: 'sort_order:asc',
        filters: {}
      }),
      undefined
    )

    await wrapper.get('[data-tab="platform"]').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-tab="tenant"]').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledTimes(1)
  })

  it('菜单与权限资源以树形结构展示', async () => {
    listResources.mockImplementation((resource: string) => {
      if (resource === 'resources') {
        return Promise.resolve({
          items: [
            { id: '1', parent_id: 0, name: '平台管理', code: 'menu.platform', type: 1, sort_order: 10 },
            { id: '2', parent_id: 1, name: 'App 用户', code: 'app-users', type: 2, sort_order: 11 }
          ],
          total: 2
        })
      }
      return Promise.resolve({ items: [], total: 0 })
    })
    const wrapper = mountView('resources')
    await flushPromises()

    expect(wrapper.find('.tree-summary').text()).toContain('共 2 项资源')
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
    expect(drawer.attributes('data-title')).toBe('编辑App 用户')
  })

  it('租户列表提供独立开通配置入口', async () => {
    const wrapper = mountView('tenants')
    await flushPromises()

    expect(wrapper.get('[data-testid="tenant-setup-1"]').attributes('to')).toBe(
      '/platform/tenants/1/setup'
    )
  })

  it('目标租户管理员通过查询上下文调用接口', async () => {
    const wrapper = mountView('tenant-admins', '8')
    await flushPromises()

    expect(listResources).toHaveBeenCalledWith(
      'tenant-admins',
      expect.objectContaining({ page: 1, page_size: 20 }),
      { targetTenantId: '8' }
    )
    await wrapper.get('.heading-actions button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="resource-form-drawer"]').attributes('data-open')).toBe('true')
  })

  it('切换Casbin策略类型时重新加载对应关联候选项', async () => {
    const wrapper = mountView('casbin-rules')
    await flushPromises()

    await wrapper.get('.heading-actions > button').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledWith(
      'resources',
      { page: 1, page_size: 500, sort: 'sort_order:asc', filters: {} },
      undefined
    )

    await wrapper.get('[data-field="ptype"]').trigger('click')
    await flushPromises()
    expect(listResources).toHaveBeenCalledWith(
      'tenant-admins',
      { page: 1, page_size: 200, sort: 'id:asc', filters: {} },
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
