import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import TopNavigation from './TopNavigation.vue'

describe('TopNavigation', () => {
  it('展示日志中心二级菜单并在子路由激活顶级入口', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: { template: '<div />' } },
        { path: '/logs/:type', component: { template: '<div />' } }
      ]
    })
    await router.push('/logs/api')
    await router.isReady()

    const wrapper = mount(TopNavigation, {
      props: {
        tenantName: '平台管理',
        userName: '管理员',
        items: [
          { label: '工作台', to: '/' },
          {
            label: '日志中心',
            to: '/logs/audit',
            children: [
              { label: '登录日志', to: '/logs/login' },
              { label: '操作审计', to: '/logs/audit' },
              { label: 'API 日志', to: '/logs/api' },
              { label: '导出记录', to: '/logs/exports' }
            ]
          }
        ]
      },
      global: { plugins: [router] }
    })

    const group = wrapper.get('[data-testid="navigation-group-日志中心"]')
    expect(group.text()).toContain('登录日志')
    expect(group.text()).toContain('操作审计')
    expect(group.text()).toContain('API 日志')
    expect(group.text()).toContain('导出记录')
    expect(group.get('button').classes()).toContain('navigation-link--active')
    expect(group.findAll('[role="menuitem"]')).toHaveLength(4)
  })
})
