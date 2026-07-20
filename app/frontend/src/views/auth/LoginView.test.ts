import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import LoginView from './LoginView.vue'

vi.mock('@/api/auth', () => ({
  getCaptcha: vi.fn().mockResolvedValue({
    captchaId: 'captcha-id',
    imageDataUri: 'data:image/png;base64,AA=='
  })
}))

describe('LoginView', () => {
  it('展示三标识登录和安全提示', () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }]
    })
    const wrapper = mount(LoginView, {
      global: {
        plugins: [createTestingPinia({ createSpy: vi.fn }), router]
      }
    })

    expect(wrapper.get('h1').text()).toBe('欢迎回来')
    expect(wrapper.get('input[name="identifier"]').attributes('placeholder')).toContain(
      '用户名 / 邮箱 / 手机号'
    )
    expect(wrapper.find('input[name="password"]').exists()).toBe(true)
    expect(wrapper.find('input[name="captcha"]').exists()).toBe(true)
    expect(wrapper.get('button[type="submit"]').text()).toContain('安全登录')
  })
})
