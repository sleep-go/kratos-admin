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

  it('为三类日志声明后端允许的结构化筛选项', () => {
    expect(resourceDefinitions['login-logs'].filters?.map((item) => item.key)).toEqual([
      'result',
      'user_id'
    ])
    expect(resourceDefinitions['audit-logs'].filters?.map((item) => item.key)).toEqual([
      'action',
      'resource_type',
      'user_id',
      'member_id'
    ])
    expect(resourceDefinitions['api-logs'].filters?.map((item) => item.key)).toEqual([
      'method',
      'status_code',
      'user_id'
    ])
  })

  it('为日志枚举值提供中文标签', () => {
    const result = resourceDefinitions['login-logs'].fields.find((item) => item.key === 'result')
    const logType = resourceDefinitions['log-exports'].fields.find(
      (item) => item.key === 'log_type'
    )

    expect(result?.valueLabels?.['1']).toBe('成功')
    expect(result?.valueLabels?.['4']).toBe('需要 MFA')
    expect(logType?.valueLabels?.api).toBe('API 日志')
  })
})
