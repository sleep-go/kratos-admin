import { mount } from '@vue/test-utils'

import ProviderConfigDialog from './ProviderConfigDialog.vue'

describe('ProviderConfigDialog', () => {
  it('仅对 schema 标记的敏感参数启用密码显示切换', () => {
    const wrapper = mount(ProviderConfigDialog, {
      props: {
        open: true,
        row: {
          id: '1',
          provider_type: 'email',
          provider_name: 'smtp',
          display_name: '生产 SMTP',
          config: {}
        },
        targetTenantId: '0'
      },
      global: {
        stubs: {
          ElDialog: { template: '<div><slot /></div>' },
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<label><slot /></label>' },
          ElInput: {
            name: 'ElInput',
            props: {
              modelValue: { type: [String, Number] },
              type: { type: String, default: 'text' },
              showPassword: { type: Boolean, default: false }
            },
            template: '<input data-testid="provider-input" :type="type" />'
          },
          ElSelect: { template: '<select><slot /></select>' },
          ElOption: true,
          ElSwitch: true,
          ElButton: true
        }
      }
    })

    const inputs = wrapper.findAllComponents({ name: 'ElInput' })
    const passwordInputs = inputs.filter((input) => input.props('type') === 'password')
    const regularInputs = inputs.filter((input) => input.props('type') === 'text')

    expect(passwordInputs).toHaveLength(1)
    expect(passwordInputs[0]?.props('showPassword')).toBe(true)
    expect(regularInputs.length).toBeGreaterThan(0)
    expect(regularInputs.every((input) => input.props('showPassword') === false)).toBe(true)
  })
})
