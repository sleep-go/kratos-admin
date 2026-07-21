import { describe, expect, it } from 'vitest'

import { auditActionLabel, auditResourceLabel, auditSummaryLabel } from './auditLabels'

describe('操作审计中文展示', () => {
  it('将已知动作和资源类型转换为中文', () => {
    expect(auditActionLabel('update_authorization')).toBe('更新授权')
    expect(auditActionLabel('request-delete')).toBe('请求删除')
    expect(auditResourceLabel('resources')).toBe('菜单与权限资源')
    expect(auditResourceLabel('dictionary-items')).toBe('字典项')
  })

  it('将后端自动生成的英文摘要转换为中文摘要', () => {
    expect(auditSummaryLabel('update roles 8', 'update', 'roles', '8')).toBe('更新角色 #8')
    expect(
      auditSummaryLabel('create-upload files file-1', 'create-upload', 'files', 'file-1')
    ).toBe('创建文件上传 #file-1')
  })

  it('未知标识和后端自定义摘要保持原值', () => {
    expect(auditActionLabel('custom_action')).toBe('custom_action')
    expect(auditResourceLabel('custom-resource')).toBe('custom-resource')
    expect(auditSummaryLabel('管理员手动调整权限', 'update', 'roles', '8')).toBe(
      '管理员手动调整权限'
    )
  })
})
