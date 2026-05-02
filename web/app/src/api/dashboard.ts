import { post } from './client'
import type { DashboardOverviewDTO } from '@/types'

export function dashboardOverview(rangeSeconds?: number) {
  return post<DashboardOverviewDTO>('/dashboard/overview', { range_seconds: rangeSeconds })
}
