import { post } from './client'
import client from './client'
import type { SOPluginDTO, ListResponse, UploadSOPluginRequest } from '@/types'

export function listSOPlugins(params?: { offset?: number; limit?: number; status?: string }) {
  return post<ListResponse<SOPluginDTO>>('/so-plugins/list', params)
}

export function createSOPlugin(req: UploadSOPluginRequest) {
  return post<SOPluginDTO>('/so-plugins/create', req)
}

// UploadSOPluginFile uploads a .so file via multipart/form-data.
// Returns the server-side file path.
export async function uploadSOPluginFile(file: File): Promise<{ file_path: string; filename: string; size: string }> {
  const formData = new FormData()
  formData.append('file', file)
  const resp = await client.post('/so-plugins/upload-file', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return resp.data.data
}

export function getSOPlugin(id: string) {
  return post<SOPluginDTO>('/so-plugins/get', { id })
}

export function updateSOPluginStatus(id: string, status: string) {
  return post<null>('/so-plugins/status', { id, status })
}

export function updateSOPluginConfig(id: string, config: string) {
  return post<null>('/so-plugins/config', { id, config })
}

export function deleteSOPlugin(id: string) {
  return post<null>('/so-plugins/delete', { id })
}