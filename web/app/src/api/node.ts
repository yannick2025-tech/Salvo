import { post } from './client'
import type { NodeDTO, EdgeDTO, AddNodeRequest, UpdateNodeRequest, AddEdgeRequest, ListResponse, ApiResponse } from '@/types'

export function listNodes(sceneId: number) {
  return post<ListResponse<NodeDTO>>('/scenes/nodes/list', { scene_id: sceneId, limit: 200 })
}

export function addNode(req: AddNodeRequest) {
  return post<NodeDTO>('/scenes/nodes/add', req)
}

export function updateNode(req: UpdateNodeRequest) {
  return post<NodeDTO>('/scenes/nodes/update', req)
}

export function deleteNode(id: number): Promise<ApiResponse<any>> {
  return post('/scenes/nodes/delete', { id })
}

export function listEdges(sceneId: number) {
  return post<ListResponse<EdgeDTO>>('/scenes/edges/list', { scene_id: sceneId, limit: 200 })
}

export function addEdge(req: AddEdgeRequest) {
  return post<EdgeDTO>('/scenes/edges/add', req)
}

export function deleteEdge(id: number): Promise<ApiResponse<any>> {
  return post('/scenes/edges/delete', { id })
}
