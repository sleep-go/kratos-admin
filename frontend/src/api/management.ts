import { http } from './http'
import type { AdminV1ListResourcesResponse } from './generated'

export type ResourceRow = Record<string, unknown>

export async function listResources(resource: string, params: Record<string, unknown>) {
  const response = await http.get<AdminV1ListResourcesResponse>(`/management/${resource}`, {
    params
  })
  return response.data
}

export async function createResource(resource: string, data: ResourceRow) {
  const response = await http.post<{ id: string }>(`/management/${resource}`, data)
  return response.data
}

export async function updateResource(resource: string, id: string, data: ResourceRow) {
  await http.put(`/management/${resource}/${id}`, data)
}

export async function deleteResource(resource: string, id: string) {
  await http.delete(`/management/${resource}/${id}`)
}
