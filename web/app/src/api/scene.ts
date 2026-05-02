import { post } from './client'
import type { SceneDTO, CreateSceneRequest, ListResponse, RunRecordDTO, SceneStatusDTO, StartSceneRequest } from '@/types'

export function listScenes(params?: { status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<SceneDTO>>('/scenes/list', params)
}

export function getScene(id: number) {
  return post<SceneDTO>('/scenes/get', { id })
}

export function createScene(req: CreateSceneRequest) {
  return post<SceneDTO>('/scenes/create', req)
}

export function updateScene(req: any) {
  return post<SceneDTO>('/scenes/update', req)
}

export function deleteScene(id: number) {
  return post('/scenes/delete', { id })
}

export function startScene(req: StartSceneRequest) {
  return post<SceneStatusDTO>('/scenes/start', req)
}

export function stopScene(sceneId: number) {
  return post('/scenes/stop', { scene_id: sceneId })
}

export function sceneStatus(sceneId: number) {
  return post<SceneStatusDTO>('/scenes/status', { scene_id: sceneId })
}

export function listRuns(params?: { scene_id?: number; status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<RunRecordDTO>>('/runs/list', params)
}

export function getRun(id: number) {
  return post<RunRecordDTO>('/runs/get', { id })
}
