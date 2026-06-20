import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { activitiesApi } from './api'
import type { CreateActivityDto, UpdateActivityDto } from './types'

export const ACTIVITIES_KEY = ['activities'] as const
const TODAY_KEY = ['today'] as const

export function useActivities() {
  return useQuery({
    queryKey: ACTIVITIES_KEY,
    queryFn: () => activitiesApi.list(),
    throwOnError: false,
  })
}

export function useCreateActivity() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (dto: CreateActivityDto) => activitiesApi.create(dto),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ACTIVITIES_KEY })
      void qc.invalidateQueries({ queryKey: TODAY_KEY })
    },
  })
}

export function useUpdateActivity() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, dto }: { id: string; dto: UpdateActivityDto }) =>
      activitiesApi.update(id, dto),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ACTIVITIES_KEY })
      void qc.invalidateQueries({ queryKey: TODAY_KEY })
    },
  })
}

export function useDeleteActivity() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => activitiesApi.delete(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ACTIVITIES_KEY })
      void qc.invalidateQueries({ queryKey: TODAY_KEY })
    },
  })
}
