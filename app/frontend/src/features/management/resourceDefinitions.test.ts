import { describe, expect, it } from 'vitest'

import { resourceDefinitions } from './resourceDefinitions'

describe('后台资源页面注册表', () => {
  it('覆盖计划中的核心管理模块', () => {
    for (const key of [
      'tenants',
      'app-users',
      'tenant-admins',
      'platform-roles',
      'platform-casbin-rules',
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
    expect(resourceDefinitions['tenant-resources'].readOnly).toBe(true)
  })

  it('菜单与权限资源按平台与租户拆分维护', () => {
    const tabs = resourceDefinitions.resources.scopeTabs
    expect(resourceDefinitions.resources.listMode).toBe('tree')
    expect(tabs?.map((tab) => tab.label)).toEqual(['全部', '平台资源', '租户资源'])
    expect(tabs?.[0]?.scopeSide).toBe('all')
    expect(tabs?.[1]?.scopeSide).toBe('platform')
    expect(tabs?.[2]?.scopeSide).toBe('tenant')
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
    const reason = resourceDefinitions['login-logs'].fields.find((item) => item.key === 'reason')
    const logType = resourceDefinitions['log-exports'].fields.find(
      (item) => item.key === 'log_type'
    )

    expect(result?.valueLabels?.['1']).toBe('成功')
    expect(result?.valueLabels?.['4']).toBe('需要 MFA')
    expect(reason?.valueLabels?.AUTH_INVALID_CREDENTIALS).toBe('账号或密码错误')
    expect(reason?.valueLabels?.CAPTCHA_INVALID).toBe('图形验证码错误或已过期')
    expect(logType?.valueLabels?.api).toBe('API 日志')
  })

  it('按不同业务语义展示用户、租户和文件状态', () => {
    const userStatus = resourceDefinitions['app-users'].fields.find((item) => item.key === 'status')
    const tenantStatus = resourceDefinitions.tenants.fields.find((item) => item.key === 'status')
    const fileStatus = resourceDefinitions.files.fields.find((item) => item.key === 'status')

    expect(userStatus?.options).toContainEqual({ label: '锁定', value: 3 })
    expect(tenantStatus?.options).toContainEqual({ label: '冻结', value: 2 })
    expect(fileStatus?.valueLabels).toMatchObject({
      '1': '待确认',
      '2': '可用',
      '4': '清理失败',
      '5': '等待后台清理'
    })
  })

  it('操作审计筛选项使用中文选项并保留英文传输值', () => {
    const action = resourceDefinitions['audit-logs'].filters?.find((item) => item.key === 'action')
    const resourceType = resourceDefinitions['audit-logs'].filters?.find(
      (item) => item.key === 'resource_type'
    )

    expect(action).toMatchObject({ type: 'select' })
    expect(action?.options).toContainEqual({ label: '更新授权', value: 'update_authorization' })
    expect(resourceType).toMatchObject({ type: 'select' })
    expect(resourceType?.options).toContainEqual({ label: '菜单与权限资源', value: 'resources' })
  })

  it('操作审计列表字段使用同一套中文展示规则', () => {
    const fields = resourceDefinitions['audit-logs'].fields
    const row = {
      action: 'update_authorization',
      resource_type: 'roles',
      resource_id: '8',
      summary: 'update_authorization roles 8'
    }

    expect(fields.find((item) => item.key === 'summary')?.format?.(row.summary, row)).toBe(
      '更新授权角色 #8'
    )
    expect(fields.find((item) => item.key === 'action')?.format?.(row.action, row)).toBe('更新授权')
    expect(
      fields.find((item) => item.key === 'resource_type')?.format?.(row.resource_type, row)
    ).toBe('角色')
  })

  it('可写关联字段使用接口选项或树选择而不是数字输入', () => {
    const expectedTypes: Record<string, Record<string, 'relation' | 'tree'>> = {
      members: {
        user_id: 'relation',
        primary_department_id: 'tree',
        position_id: 'relation'
      },
      departments: { parent_id: 'tree' },
      resources: { parent_id: 'tree' },
      'dictionary-items': { type_id: 'relation' }
    }

    for (const [resource, fields] of Object.entries(expectedTypes)) {
      for (const [key, type] of Object.entries(fields)) {
        const field = resourceDefinitions[resource].fields.find((item) => item.key === key)
        expect(field, `${resource}.${key}`).toMatchObject({ type })
        expect(field?.lookup ?? field?.lookupBy, `${resource}.${key} lookup`).toBeDefined()
      }
    }
  })

  it('创建租户不要求或默认创建管理员', () => {
    expect(resourceDefinitions.tenants.fields.some((field) => field.key === 'admin_user_id')).toBe(
      false
    )
  })

  it('业务枚举使用带中文标签的下拉选择', () => {
    for (const [resource, key] of [
      ['app-users', 'mfa_channel'],
      ['resources', 'type'],
      ['resources', 'scope_mask'],
      ['casbin-rules', 'ptype']
    ] as const) {
      const field = resourceDefinitions[resource].fields.find((item) => item.key === key)
      expect(field, `${resource}.${key}`).toMatchObject({ type: 'select' })
      expect(field?.options?.length, `${resource}.${key} options`).toBeGreaterThan(1)
    }
  })

  it('部门路径由后端维护且Casbin关联值使用动态候选项', () => {
    const path = resourceDefinitions.departments.fields.find((field) => field.key === 'path')
    expect(path?.form).toBe(false)

    const policyMember = resourceDefinitions['casbin-rules'].fields.find(
      (field) => field.key === 'v1'
    )
    const policyTarget = resourceDefinitions['casbin-rules'].fields.find(
      (field) => field.key === 'v2'
    )
    expect(policyMember?.lookupBy?.field).toBe('ptype')
    expect(policyTarget?.lookupBy?.field).toBe('ptype')
  })
})
