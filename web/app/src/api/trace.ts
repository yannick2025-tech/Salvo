import { post } from './client'
import type { TraceDTO, ListResponse } from '@/types'

export function listTraces(params?: { scene_id?: number; limit?: number; offset?: number }) {
  return post<ListResponse<TraceDTO>>('/traces/list', params)
}

export function getTrace(id: number) {
  return post<TraceDTO>('/traces/get', { id })
}

export function getTraceByRun(runId: number) {
  return post<TraceDTO>('/traces/get-by-run', { run_id: runId })
}
