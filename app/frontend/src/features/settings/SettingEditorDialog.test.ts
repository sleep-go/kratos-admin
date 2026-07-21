import { mount } from '@vue/test-utils'

import SettingEditorDialog from './SettingEditorDialog.vue'

const stubs = {
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
    template: '<input data-testid="setting-input" :type="type" />'
  },
  ElSelect: { template: '<select><slot /></select>' },
  ElOption: true,
  ElSwitch: true,
  ElButton: true
}

function mountDialog(isSecret: boolean) {
  return mount(SettingEditorDialog, {
    props: {
      open: true,
      item: {
        category: 'security',
        setting_key: isSecret ? 'signing_key' : 'access_token_minutes',
        setting_value: isSecret ? undefined : 15,
        value_type: 'string',
        is_secret: isSecret
      },
      targetTenantId: '0',
      platformContext: true
    },
    global: { stubs }
  })
}

describe('SettingEditorDialog', () => {
  it('仅对敏感配置启用密码显示切换', () => {
    const regularInput = mountDialog(false).getComponent({ name: 'ElInput' })
    const secretInput = mountDialog(true).getComponent({ name: 'ElInput' })

    expect(regularInput.props('type')).toBe('text')
    expect(regularInput.props('showPassword')).toBe(false)
    expect(secretInput.props('type')).toBe('password')
    expect(secretInput.props('showPassword')).toBe(true)
  })
})
