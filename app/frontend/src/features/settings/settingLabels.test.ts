import {
  settingCategoryLabel,
  settingKeyLabel,
  settingSourceLabel,
  settingValueTypeLabel
} from './settingLabels'

describe('系统设置中文标签', () => {
  it('转换当前内置分类和配置键', () => {
    expect(settingCategoryLabel('platform')).toBe('平台设置')
    expect(settingCategoryLabel('security')).toBe('安全策略')
    expect(settingCategoryLabel('log')).toBe('日志保留')
    expect(settingCategoryLabel('file')).toBe('文件设置')
    expect(settingKeyLabel('security', 'access_token_minutes')).toBe('访问令牌有效时间（分钟）')
    expect(settingKeyLabel('log', 'audit_retention_days')).toBe('操作审计保留天数')
  })

  it('转换值类型和配置来源', () => {
    expect(settingValueTypeLabel('string')).toBe('字符串')
    expect(settingValueTypeLabel('number')).toBe('数字')
    expect(settingValueTypeLabel('boolean')).toBe('布尔值')
    expect(settingValueTypeLabel('json')).toBe('JSON 数据')
    expect(settingSourceLabel('tenant')).toBe('租户覆盖')
    expect(settingSourceLabel('platform')).toBe('平台默认')
    expect(settingSourceLabel('code')).toBe('代码安全默认')
  })

  it('未知标识保留原值以避免配置项消失', () => {
    expect(settingCategoryLabel('custom')).toBe('custom')
    expect(settingKeyLabel('custom', 'new_key')).toBe('new_key')
    expect(settingValueTypeLabel('duration')).toBe('duration')
  })
})
