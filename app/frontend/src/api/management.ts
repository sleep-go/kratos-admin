import { http } from './http'
import type {
  AdminV1GetEffectiveSettingsResponse,
  AdminV1ListResourcesResponse,
  AdminV1TestProviderConnectionResponse,
  AdminV1UpdateRoleAuthorizationRequest,
  AdminV1UpdateTenantFeaturesRequest
} from './generated'

export type ResourceRow = Record<string, unknown>

export interface ManagementScopeOptions {
  targetTenantId?: string
}

function targetTenantConfig(options?: ManagementScopeOptions) {
  return options?.targetTenantId
    ? { params: { target_tenant_id: options.targetTenantId } }
    : undefined
}

export async function listResources(
  resource: string,
  params: Record<string, unknown>,
  options?: ManagementScopeOptions
) {
  const response = await http.get<AdminV1ListResourcesResponse>(`/management/${resource}`, {
    params: { ...params, ...targetTenantConfig(options)?.params }
  })
  return response.data
}

export async function createResource(
  resource: string,
  data: ResourceRow,
  options?: ManagementScopeOptions
) {
  const response = await http.post<{ id: string }>(
    `/management/${resource}`,
    data,
    targetTenantConfig(options)
  )
  return response.data
}

export async function updateResource(
  resource: string,
  id: string,
  data: ResourceRow,
  options?: ManagementScopeOptions
) {
  await http.put(`/management/${resource}/${id}`, data, targetTenantConfig(options))
}

export async function deleteResource(resource: string, id: string, options?: ManagementScopeOptions) {
  await http.delete(`/management/${resource}/${id}`, targetTenantConfig(options))
}

export async function getEffectiveSettings(category = '') {
  const response = await http.get<AdminV1GetEffectiveSettingsResponse>('/settings/effective', {
    params: { category }
  })
  return response.data
}

export async function testProviderConnection(id: string) {
  const response = await http.post<AdminV1TestProviderConnectionResponse>(
    `/management/providers/${id}/connection-test`
  )
  return response.data
}

export async function updateRoleAuthorization(request: AdminV1UpdateRoleAuthorizationRequest) {
  if (!request.roleId) throw new Error('角色 ID 不能为空')
  await http.put(`/management/roles/${encodeURIComponent(request.roleId)}/authorization`, request)
}

export async function updateTenantFeatures(request: AdminV1UpdateTenantFeaturesRequest) {
  if (!request.tenantId) throw new Error('租户 ID 不能为空')
  await http.put(`/management/tenants/${encodeURIComponent(request.tenantId)}/features`, request)
}
