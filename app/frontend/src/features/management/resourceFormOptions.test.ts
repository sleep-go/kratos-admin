import { describe, expect, it } from 'vitest'

import type { ResourceField } from './resourceDefinitions'
import { buildLookupOptions, resolveFieldLookup } from './resourceFormOptions'

describe('通用数据表单候选项', () => {
  it('将接口记录转换为包含业务名称和ID的下拉选项', () => {
    const options = buildLookupOptions(
      [
        { id: '8', display_name: '张三', username: 'zhangsan', status: 1 },
        { id: '9', display_name: '已禁用', username: 'disabled', status: 2 }
      ],
      {
        resource: 'users',
        labelKeys: ['display_name', 'username'],
        onlyActive: true
      }
    )

    expect(options).toEqual([{ value: '8', label: '张三 · zhangsan · ID 8' }])
  })

  it('构建树选项时排除当前节点及其全部子节点', () => {
    const options = buildLookupOptions(
      [
        { id: '1', parent_id: '0', name: '总部', status: 1 },
        { id: '2', parent_id: '1', name: '研发部', status: 1 },
        { id: '3', parent_id: '2', name: '研发一组', status: 1 },
        { id: '4', parent_id: '1', name: '财务部', status: 1 }
      ],
      {
        resource: 'departments',
        labelKeys: ['name'],
        parentKey: 'parent_id',
        emptyLabel: '根部门',
        excludeCurrentTree: true
      },
      '2'
    )

    expect(options).toEqual([
      { value: 0, label: '根部门' },
      {
        value: '1',
        label: '总部 · ID 1',
        children: [{ value: '4', label: '财务部 · ID 4' }]
      }
    ])
  })

  it('根据Casbin策略类型解析不同关联资源', () => {
    const field: ResourceField = {
      key: 'v2',
      label: '资源 / 角色',
      type: 'relation',
      lookupBy: {
        field: 'ptype',
        values: {
          p: { resource: 'resources', valueKey: 'code', labelKeys: ['name', 'code'] },
          g: { resource: 'roles', labelKeys: ['name', 'code'] }
        }
      }
    }

    expect(resolveFieldLookup(field, { ptype: 'p' })?.resource).toBe('resources')
    expect(resolveFieldLookup(field, { ptype: 'g' })?.resource).toBe('roles')
  })

  it('排除已经建立关联的候选记录', () => {
    const options = buildLookupOptions(
      [
        { id: '8', display_name: '已加入成员', status: 1 },
        { id: '9', display_name: '可选用户', status: 1 }
      ],
      { resource: 'users', labelKeys: ['display_name'], onlyActive: true },
      '',
      new Set(['8'])
    )

    expect(options).toEqual([{ value: '9', label: '可选用户 · ID 9' }])
  })

  it('保留接口返回的64位ID字符串精度', () => {
    const options = buildLookupOptions(
      [{ id: '9007199254740993', name: '超大主键', status: 1 }],
      { resource: 'departments', labelKeys: ['name'] }
    )

    expect(options[0]?.value).toBe('9007199254740993')
  })
})
