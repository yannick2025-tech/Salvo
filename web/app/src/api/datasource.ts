import { post } from './client'

export interface DataSourceDTO {
  id: string
  scene_id: string
  name: string
  file_name: string
  columns: string[]
  row_count: number
  source: string // "yaml" or "csv"
  removed_empty_rows?: number
  created_at: string
  updated_at: string
}

export interface DataSourcePreviewDTO extends DataSourceDTO {
  rows: Record<string, string>[]
}

export function uploadDataSource(sceneId: string, fileName: string, content: string) {
  return post<DataSourceDTO>('/scenes/datasources/upload', { scene_id: sceneId, file_name: fileName, content })
}

export function listDataSources(sceneId: string) {
  return post<{ items: DataSourceDTO[] }>('/scenes/datasources/list', { scene_id: sceneId })
}

export function previewDataSource(id: string) {
  return post<DataSourcePreviewDTO>('/scenes/datasources/preview', { id })
}

export function deleteDataSource(id: string) {
  return post('/scenes/datasources/delete', { id })
}