import { describe, expect, it } from 'vitest'

import { iconRegistry, resolveNavigationIcon } from './registry'

describe('iconRegistry', () => {
  it('包含导航所需的核心图标', () => {
    expect(iconRegistry.dashboard).toBeTruthy()
    expect(iconRegistry.platform).toBeTruthy()
    expect(iconRegistry.settings).toBeTruthy()
  })

  it('根据导航文案返回对应图标', () => {
    expect(resolveNavigationIcon('工作台')).toBe(iconRegistry.dashboard)
    expect(resolveNavigationIcon('系统设置')).toBe(iconRegistry.settings)
    expect(resolveNavigationIcon('未知菜单')).toBeUndefined()
  })
})
