import { post } from './client'
import type { DashboardOverviewDTO, DashboardHistoryDTO } from '@/types'

<<<<<<< HEAD
export function dashboardOverview(rangeSeconds?: number, sceneId?: string) {
  return post<DashboardOverviewDTO>('/dashboard/overview', { range_seconds: rangeSeconds, scene_id: sceneId })
}

export function dashboardHistory(sceneId?: number, limit?: number) {
  return post<DashboardHistoryDTO>('/dashboard/history', { scene_id: sceneId, limit: limit || 20 })
=======
export function dashboardOverview(rangeSeconds?: number, sceneId?: string, runId?: string) {
  return post<DashboardOverviewDTO>('/dashboard/overview', {
    range_seconds: rangeSeconds,
    scene_id: sceneId,
    run_id: runId
  })
}

export function dashboardHistory(sceneId?: string, runId?: string, limit?: number) {
  return post<DashboardHistoryDTO>('/dashboard/history', { scene_id: sceneId, run_id: runId, limit: limit || 20 })
>>>>>>> dev
}
