import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

import LoginView from './LoginView.vue'

describe('LoginView', () => {
  it('展示三标识登录和安全提示', () => {
    const wrapper = mount(LoginView, {
      global: {
        plugins: [createTestingPinia({ createSpy: vi.fn })]
      }
    })

    expect(wrapper.get('h1').text()).toBe('欢迎回来')
    expect(wrapper.get('input[name="identifier"]').attributes('placeholder')).toContain(
      '用户名 / 邮箱 / 手机号'
    )
    expect(wrapper.find('input[name="password"]').exists()).toBe(true)
    expect(wrapper.get('button[type="submit"]').text()).toContain('安全登录')
  })
})
