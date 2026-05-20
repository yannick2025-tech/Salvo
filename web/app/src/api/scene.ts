import { post } from './client'
import type { SceneDTO, CreateSceneRequest, ListResponse, RunRecordDTO, SceneStatusDTO, StartSceneRequest } from '@/types'

export function listScenes(params?: { status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<SceneDTO>>('/scenes/list', params)
}

export function getScene(id: string) {
  return post<SceneDTO>('/scenes/get', { id })
}

export function createScene(req: CreateSceneRequest) {
  return post<SceneDTO>('/scenes/create', req)
}

export function importYAML(req: { name: string; description?: string; yaml: string }) {
  return post<SceneDTO>('/scenes/import', req)
}

export function updateScene(req: any) {
  return post<SceneDTO>('/scenes/update', req)
}

export function deleteScene(id: string) {
  return post('/scenes/delete', { id })
}

export function startScene(req: StartSceneRequest) {
  return post<SceneStatusDTO>('/scenes/start', req)
}

export function stopScene(sceneId: string) {
  return post('/scenes/stop', { scene_id: sceneId })
}

export function sceneStatus(sceneId: string) {
  return post<SceneStatusDTO>('/scenes/status', { scene_id: sceneId })
}

export function listRuns(params?: { scene_id?: string; status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<RunRecordDTO>>('/runs/list', params)
}

export function getRun(id: string) {
  return post<RunRecordDTO>('/runs/get', { id })
}
