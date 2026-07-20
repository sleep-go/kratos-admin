import { describe, expect, it } from 'vitest'

import { resourceDefinitions } from './resourceDefinitions'

describe('后台资源页面注册表', () => {
  it('覆盖计划中的核心管理模块', () => {
    for (const key of [
      'tenants',
      'members',
      'departments',
      'positions',
      'roles',
      'resources',
      'login-logs',
      'audit-logs',
      'api-logs',
      'files',
      'dictionary-types',
      'providers',
      'settings'
    ]) {
      expect(resourceDefinitions[key], key).toBeDefined()
    }
  })

  it('日志和文件元数据不能通过通用页面伪造', () => {
    expect(resourceDefinitions['audit-logs'].readOnly).toBe(true)
    expect(resourceDefinitions.files.readOnly).toBe(true)
  })
})
