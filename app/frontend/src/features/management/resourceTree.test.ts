import { describe, expect, it } from 'vitest'

import {
  buildResourceTree,
  canAddResourceChild,
  filterResourceRows,
  filterResourceRowsByScopeSide,
  flattenResourceTree,
  matchesResourceKeyword
} from './resourceTree'

describe('resourceTree', () => {
  const rows = [
    { id: '1', parent_id: 0, name: '平台管理', code: 'menu.platform', sort_order: 10, type: 1, scope_mask: 1 },
    { id: '2', parent_id: 1, name: '全局用户', code: 'users', sort_order: 11, type: 2, scope_mask: 1 },
    { id: '3', parent_id: 1, name: '租户管理', code: 'tenants', sort_order: 12, type: 2, scope_mask: 1 },
    { id: '4', parent_id: 0, name: '组织管理', code: 'menu.organization', sort_order: 20, type: 1, scope_mask: 2 },
    { id: '5', parent_id: 4, name: '成员管理', code: 'members', sort_order: 21, type: 2, scope_mask: 2 },
    { id: '6', parent_id: 0, name: '日志中心', code: 'menu.logs', sort_order: 30, type: 1, scope_mask: 3 },
    { id: '7', parent_id: 6, name: '登录日志', code: 'login-logs', sort_order: 31, type: 2, scope_mask: 3 },
    { id: '8', parent_id: 0, name: '系统设置', code: 'menu.settings', sort_order: 40, type: 1, scope_mask: 3 },
    { id: '9', parent_id: 8, name: '系统设置', code: 'settings', sort_order: 41, type: 2, scope_mask: 3 },
    { id: '10', parent_id: 8, name: '参数字典', code: 'dictionary-types', sort_order: 42, type: 2, scope_mask: 2 }
  ]

  it('按 parent_id 与 sort_order 组装树', () => {
    const tree = buildResourceTree(rows)
    expect(tree.map((item) => item.code)).toEqual(['menu.platform', 'menu.organization', 'menu.logs', 'menu.settings'])
    expect(tree[0]?.children?.map((item) => item.code)).toEqual(['users', 'tenants'])
  })

  it('按平台或租户视角过滤时保留祖先节点', () => {
    const filtered = filterResourceRowsByScopeSide(rows, 'platform')
    expect(filtered.map((item) => item.code)).toEqual([
      'menu.platform',
      'users',
      'tenants',
      'menu.logs',
      'login-logs',
      'menu.settings',
      'settings'
    ])
  })

  it('关键词过滤时保留祖先节点', () => {
    const filtered = filterResourceRows(rows, '成员')
    expect(filtered.map((item) => item.code)).toEqual(['menu.organization', 'members'])
  })

  it('拍平树时保留层级深度', () => {
    const flattened = flattenResourceTree(buildResourceTree(rows))
    expect(flattened.map((item) => `${item.depth}:${item.code}`)).toEqual([
      '0:menu.platform',
      '1:users',
      '1:tenants',
      '0:menu.organization',
      '1:members',
      '0:menu.logs',
      '1:login-logs',
      '0:menu.settings',
      '1:settings',
      '1:dictionary-types'
    ])
  })

  it('识别可添加子资源的节点类型', () => {
    expect(canAddResourceChild({ type: 1 })).toBe(true)
    expect(canAddResourceChild({ type: 2 })).toBe(true)
    expect(canAddResourceChild({ type: 3 })).toBe(false)
    expect(matchesResourceKeyword({ name: '登录日志', code: 'login-logs' }, 'login')).toBe(true)
  })
})
