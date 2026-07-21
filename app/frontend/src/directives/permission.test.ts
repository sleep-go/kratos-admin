import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent } from 'vue'

import { hasPermission, permissionDirective } from './permission'
import { useAuthStore } from '@/stores/auth'

describe('权限指令', () => {
  it('支持精确权限和通配权限并隐藏无权操作', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useAuthStore()
    store.currentUser = {
      id: '1',
      displayName: '权限用户',
      platformAdmin: false,
      permissions: ['roles:list', 'departments:*']
    }
    const component = defineComponent({
      template: `
        <button id="roles-list" v-permission="'roles:list'">查看角色</button>
        <button id="departments-create" v-permission="'departments:create'">新增部门</button>
        <button id="roles-delete" v-permission="'roles:delete'">删除角色</button>
      `
    })

    const wrapper = mount(component, {
      global: { plugins: [pinia], directives: { permission: permissionDirective } }
    })

    expect(wrapper.get('#roles-list').attributes('hidden')).toBeUndefined()
    expect(wrapper.get('#departments-create').attributes('hidden')).toBeUndefined()
    expect(wrapper.get('#roles-delete').attributes('hidden')).toBeDefined()
  })

  it('平台管理员只有在平台上下文才可绕过权限检查', () => {
    expect(hasPermission([], 'api-logs:export', true)).toBe(true)

    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useAuthStore()
    store.currentUser = {
      id: '1',
      displayName: '平台管理员',
      platformAdmin: true,
      permissions: []
    }
    store.currentTenant = { id: '8', name: '演示租户' }
    const component = defineComponent({
      template: `<button id="platform-action" v-permission="'tenants:update'">编辑租户</button>`
    })

    const wrapper = mount(component, {
      global: { plugins: [pinia], directives: { permission: permissionDirective } }
    })

    expect(wrapper.get('#platform-action').attributes('hidden')).toBeDefined()
  })
})
