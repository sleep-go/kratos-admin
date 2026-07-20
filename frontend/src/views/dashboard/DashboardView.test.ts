import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DashboardView from './DashboardView.vue'

describe('DashboardView', () => {
  it('同时呈现核心指标和安全动态', () => {
    const wrapper = mount(DashboardView)

    expect(wrapper.get('h1').text()).toBe('工作台')
    expect(wrapper.text()).toContain('成员总数')
    expect(wrapper.text()).toContain('请求趋势')
    expect(wrapper.text()).toContain('安全动态')
  })
})
