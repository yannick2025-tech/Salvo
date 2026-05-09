import { post, get } from './client'
import type { ReportDTO, ListResponse } from '@/types'

export function listReports(params?: { scene_id?: string; status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<ReportDTO>>('/reports/list', params)
}

export function getReport(id: string) {
  return post<ReportDTO>('/reports/get', { id })
}

export function exportReportHTML(id: string): Promise<Blob> {
  return get(`/reports/${id}/export`, { responseType: 'blob' })
}
