import { apiClient } from '@/api'
import type { OccurrenceNode } from '@/features/today/types'
import type { DaySummary, ActivityHistoryEntry } from './types'

export const historyApi = {
  calendar: (from: string, to: string): Promise<DaySummary[]> =>
    apiClient.get<DaySummary[]>(`/history/calendar?from=${from}&to=${to}`),

  calendarDay: (date: string): Promise<OccurrenceNode[]> =>
    apiClient.get<OccurrenceNode[]>(`/history/calendar/${date}`),

  activityHistory: (id: string, from: string, to: string): Promise<ActivityHistoryEntry[]> =>
    apiClient.get<ActivityHistoryEntry[]>(
      `/history/activities/${id}?from=${from}&to=${to}`
    ),
}
