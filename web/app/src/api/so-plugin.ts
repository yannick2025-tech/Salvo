import { post } from './client'
import type { SOPluginDTO, ListResponse, UploadSOPluginRequest } from '@/types'

export function listSOPlugins(params?: { offset?: number; limit?: number; status?: string }) {
  return post<ListResponse<SOPluginDTO>>('/so-plugins/list', params)
}

export function createSOPlugin(req: UploadSOPluginRequest) {
  return post<SOPluginDTO>('/so-plugins/create', req)
}

export function getSOPlugin(id: number) {
  return post<SOPluginDTO>('/so-plugins/get', { id })
}

export function updateSOPluginStatus(id: number, status: string) {
  return post<null>('/so-plugins/status', { id, status })
}

export function updateSOPluginConfig(id: number, config: string) {
  return post<null>('/so-plugins/config', { id, config })
}

export function deleteSOPlugin(id: number) {
  return post<null>('/so-plugins/delete', { id })
}