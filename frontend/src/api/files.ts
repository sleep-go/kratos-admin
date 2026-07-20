import axios from 'axios'

import { http } from './http'
import type {
  AdminV1ConfirmUploadResponse,
  AdminV1CreateUploadRequest,
  AdminV1CreateUploadResponse,
  AdminV1GetDownloadUrlResponse,
  AdminV1SignedFileRequest
} from './generated'

export async function createUpload(
  request: AdminV1CreateUploadRequest
): Promise<AdminV1CreateUploadResponse> {
  const response = await http.post<AdminV1CreateUploadResponse>('/files/uploads', request)
  return response.data
}

export async function putSignedFile(
  signed: AdminV1SignedFileRequest,
  file: File,
  onProgress?: (percent: number) => void
) {
  if (!signed.url || !signed.method) throw new Error('上传签名不完整')
  const headers = { ...(signed.headers ?? {}) }
  delete headers.host
  delete headers.Host
  delete headers['content-length']
  delete headers['Content-Length']
  await axios.request({
    method: signed.method,
    url: signed.url,
    headers,
    data: file,
    onUploadProgress: (event) => {
      if (event.total && onProgress) onProgress(Math.round((event.loaded / event.total) * 100))
    }
  })
}

export async function confirmUpload(fileId: string): Promise<AdminV1ConfirmUploadResponse> {
  const response = await http.post<AdminV1ConfirmUploadResponse>(
    '/files/' + encodeURIComponent(fileId) + '/confirm',
    { fileId }
  )
  return response.data
}

export async function getDownloadURL(fileId: string): Promise<AdminV1GetDownloadUrlResponse> {
  const response = await http.get<AdminV1GetDownloadUrlResponse>(
    '/files/' + encodeURIComponent(fileId) + '/download-url'
  )
  return response.data
}

export async function deleteFile(fileId: string): Promise<void> {
  await http.delete('/files/' + encodeURIComponent(fileId))
}
