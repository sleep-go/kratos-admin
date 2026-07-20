import { http } from './http'
import type {
  AdminV1CreateExportRequest,
  AdminV1CreateExportResponse,
  AdminV1GetExportDownloadUrlResponse,
  AdminV1GetExportResponse,
  AdminV1LogExport
} from './generated'

export async function createLogExport(payload: AdminV1CreateExportRequest) {
  const response = await http.post<AdminV1CreateExportResponse>('/logs/exports', payload)
  return response.data.item as AdminV1LogExport
}

export async function getLogExport(exportId: string) {
  const response = await http.get<AdminV1GetExportResponse>(`/logs/exports/${exportId}`)
  return response.data.item as AdminV1LogExport
}

export async function getLogExportDownloadURL(exportId: string) {
  const response = await http.get<AdminV1GetExportDownloadUrlResponse>(
    `/logs/exports/${exportId}/download-url`
  )
  return response.data
}
