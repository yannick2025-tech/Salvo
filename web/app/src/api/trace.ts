import { post } from './client'
import type { TraceDTO, ListResponse } from '@/types'

<<<<<<< HEAD
export function listTraces(params?: { scene_id?: string; limit?: number; offset?: number }) {
=======
export interface TraceListQuery {
  scene_id?: string
  trace_id?: string
  scene_name?: string
  status?: string
  min_duration_ms?: number
  max_duration_ms?: number
  limit?: number
  offset?: number
}

export function listTraces(params?: TraceListQuery) {
>>>>>>> dev
  return post<ListResponse<TraceDTO>>('/traces/list', params)
}

export function getTrace(id: string) {
  return post<TraceDTO>('/traces/get', { id })
}

export function getTraceByRun(runId: string) {
  return post<TraceDTO>('/traces/get-by-run', { run_id: runId })
}
