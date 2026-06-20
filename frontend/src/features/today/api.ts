import { apiClient } from '@/api'
import type { OccurrenceNode, OccurrenceState } from './types'

export const todayApi = {
  list: (): Promise<OccurrenceNode[]> =>
    apiClient.get<OccurrenceNode[]>('/today'),

  setState: (id: string, state: OccurrenceState): Promise<OccurrenceNode[]> =>
    apiClient.patch<OccurrenceNode[]>(`/occurrences/${id}`, { state }),
}
