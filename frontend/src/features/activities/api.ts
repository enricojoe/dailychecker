import { apiClient } from '@/api'
import type { Activity, ActivityNode, CreateActivityDto, UpdateActivityDto } from './types'

export const activitiesApi = {
  list: (): Promise<ActivityNode[]> =>
    apiClient.get<ActivityNode[]>('/activities'),

  create: (dto: CreateActivityDto): Promise<Activity> =>
    apiClient.post<Activity>('/activities', dto),

  update: (id: string, dto: UpdateActivityDto): Promise<Activity> =>
    apiClient.patch<Activity>(`/activities/${id}`, dto),

  delete: (id: string): Promise<void> =>
    apiClient.delete<void>(`/activities/${id}`),
}
