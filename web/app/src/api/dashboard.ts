import { post } from './client'
import type { DashboardOverviewDTO, DashboardHistoryDTO } from '@/types'

export function dashboardOverview(rangeSeconds?: number, sceneId?: string) {
  return post<DashboardOverviewDTO>('/dashboard/overview', { range_seconds: rangeSeconds, scene_id: sceneId })
}

export function dashboardHistory(sceneId?: number, limit?: number) {
  return post<DashboardHistoryDTO>('/dashboard/history', { scene_id: sceneId, limit: limit || 20 })
}
