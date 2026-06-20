import type { OccurrenceState } from '@/features/today/types'

export type { OccurrenceState }

export interface DaySummary {
  date: string   // YYYY-MM-DD
  pending: number
  partial: number
  done: number
  total: number
}

export interface ActivityHistoryEntry {
  date: string         // YYYY-MM-DD
  state: OccurrenceState
}
