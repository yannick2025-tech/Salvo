import { post, get } from './client'
<<<<<<< HEAD
import type { ReportDTO, ListResponse } from '@/types'

export function listReports(params?: { scene_id?: string; status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<ReportDTO>>('/reports/list', params)
}

export function getReport(id: string) {
=======
import type { ReportDTO, ReportListItemDTO, ListResponse, ApiResponse } from '@/types'

export function listReports(params?: { scene_id?: string; status?: string; offset?: number; limit?: number }): Promise<ApiResponse<ListResponse<ReportListItemDTO>>> {
  return post<ListResponse<ReportListItemDTO>>('/reports/list', params)
}

export function getReport(id: string): Promise<ApiResponse<ReportDTO>> {
>>>>>>> dev
  return post<ReportDTO>('/reports/get', { id })
}

export function exportReportHTML(id: string): Promise<Blob> {
  return get(`/reports/${id}/export`, { responseType: 'blob' })
}
