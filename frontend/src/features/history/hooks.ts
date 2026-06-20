import { useQuery } from '@tanstack/react-query'
import { historyApi } from './api'

export function useCalendar(from: string, to: string) {
  return useQuery({
    queryKey: ['history', 'calendar', from, to],
    queryFn: () => historyApi.calendar(from, to),
    throwOnError: false,
    enabled: !!from && !!to,
  })
}

export function useCalendarDay(date: string | null) {
  return useQuery({
    queryKey: ['history', 'day', date],
    queryFn: () => historyApi.calendarDay(date!),
    throwOnError: false,
    enabled: !!date,
  })
}

export function useActivityHistory(id: string | null, from: string, to: string) {
  return useQuery({
    queryKey: ['history', 'activity', id, from, to],
    queryFn: () => historyApi.activityHistory(id!, from, to),
    throwOnError: false,
    enabled: !!id && !!from && !!to,
  })
}
