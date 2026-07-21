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

  it('可写关联字段使用接口选项或树选择而不是数字输入', () => {
    const expectedTypes: Record<string, Record<string, 'relation' | 'tree'>> = {
      tenants: { admin_user_id: 'relation' },
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

  it('业务枚举使用带中文标签的下拉选择', () => {
    for (const [resource, key] of [
      ['users', 'mfa_channel'],
      ['roles', 'data_scope'],
      ['resources', 'type'],
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
