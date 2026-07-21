import { beforeEach, expect, it, vi } from 'vitest'

import { http } from './http'
import { createResource, deleteResource, updateResource } from './management'

vi.mock('./http', () => ({
  http: {
    post: vi.fn().mockResolvedValue({ data: { id: '1' } }),
    put: vi.fn().mockResolvedValue({ data: {} }),
    delete: vi.fn().mockResolvedValue({ data: {} })
  }
}))

beforeEach(() => vi.clearAllMocks())

it('平台初始化资源请求通过查询参数传递目标租户', async () => {
  const options = { targetTenantId: '8' }

  await createResource('roles', { name: '管理员' }, options)
  await updateResource('roles', '3', { name: '审计员' }, options)
  await deleteResource('roles', '3', options)

  expect(http.post).toHaveBeenCalledWith('/management/roles', { name: '管理员' }, {
    params: { target_tenant_id: '8' }
  })
  expect(http.put).toHaveBeenCalledWith('/management/roles/3', { name: '审计员' }, {
    params: { target_tenant_id: '8' }
  })
  expect(http.delete).toHaveBeenCalledWith('/management/roles/3', {
    params: { target_tenant_id: '8' }
  })
})
