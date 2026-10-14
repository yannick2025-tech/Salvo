import { post } from './client'
<<<<<<< HEAD
import type { SceneDTO, CreateSceneRequest, ListResponse, RunRecordDTO, SceneStatusDTO, StartSceneRequest } from '@/types'
=======
import type { SceneDTO, CreateSceneRequest, ListResponse, RunRecordDTO, SceneStatusDTO, StartSceneRequest, ExportYAMLResponse } from '@/types'
>>>>>>> dev

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

<<<<<<< HEAD
=======
export function copyScene(sceneId: string, name: string) {
  return post<SceneDTO>('/scenes/copy', { scene_id: sceneId, name })
}

export function exportYAML(id: string) {
  return post<ExportYAMLResponse>('/scenes/export', { id })
}

>>>>>>> dev
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

<<<<<<< HEAD
=======
export function batchSetVariables(sceneId: string, variables: Record<string, string>) {
  return post<{ variables: Record<string, string> }>('/scenes/variables/batch-set', { scene_id: sceneId, variables })
}

>>>>>>> dev
export function listRuns(params?: { scene_id?: string; status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<RunRecordDTO>>('/runs/list', params)
}

export function getRun(id: string) {
  return post<RunRecordDTO>('/runs/get', { id })
}
