import { flushPromises, mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

import EffectiveSettingsPanel from './EffectiveSettingsPanel.vue'

vi.mock('@/api/management', () => ({
  getEffectiveSettings: vi.fn().mockResolvedValue({
    items: [
      {
        category: 'security',
        setting_key: 'access_token_minutes',
        setting_value: 15,
        value_type: 'number',
        source: 'code'
      },
      {
        category: 'log',
        setting_key: 'audit_retention_days',
        setting_value: 365,
        value_type: 'number',
        source: 'platform',
        allow_tenant_override: true
      }
    ]
  }),
  listResources: vi.fn().mockResolvedValue({ items: [], total: 0 })
}))

describe('EffectiveSettingsPanel', () => {
  it('用中文展示分类、配置名称、类型和来源', async () => {
    const wrapper = mount(EffectiveSettingsPanel, {
      global: {
        plugins: [
          createTestingPinia({
            createSpy: vi.fn,
            initialState: {
              auth: {
                currentTenant: { id: '0', name: '平台管理' },
                currentUser: { id: '1', displayName: '管理员', platformAdmin: true }
              }
            }
          })
        ],
        directives: { loading: () => undefined },
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          ElButton: { template: '<button><slot /></button>' },
          ElTag: { template: '<span><slot /></span>' },
          SettingEditorDialog: true
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('安全策略')
    expect(wrapper.text()).toContain('访问令牌有效时间（分钟）')
    expect(wrapper.text()).toContain('日志保留')
    expect(wrapper.text()).toContain('操作审计保留天数')
    expect(wrapper.text()).toContain('数字')
    expect(wrapper.text()).toContain('代码安全默认')
    expect(wrapper.text()).toContain('平台默认')
    expect(wrapper.text()).not.toContain('access_token_minutes')
  })
})
