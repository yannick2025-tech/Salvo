import { post } from './client'
import type { ReportDTO, ListResponse } from '@/types'

export function listReports(params?: { scene_id?: number; status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<ReportDTO>>('/reports/list', params)
}

export function getReport(id: number) {
  return post<ReportDTO>('/reports/get', { id })
}
