/**
 * /  — Today page.
 * Replaces TodayPlaceholder; renders today's occurrence tree.
 */

import { OccurrenceTree } from './OccurrenceTree'
import { useToday } from './hooks'
import { todayJakarta } from '@/lib/dateUtils'

export function TodayPage() {
  const { data, isPending, isError, error } = useToday()

  const today = todayJakarta()
  const [year, month, day] = today.split('-')
  const displayDate = `${day} ${['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][Number(month) - 1]} ${year}`

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">Today</h1>
        <p className="text-sm text-muted-foreground">{displayDate}</p>
      </div>

      {isPending && (
        <div className="flex h-24 items-center justify-center">
          <p className="text-sm text-muted-foreground">Loading…</p>
        </div>
      )}

      {isError && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error instanceof Error ? error.message : 'Failed to load today’s activities.'}
        </p>
      )}

      {data && <OccurrenceTree nodes={data} />}
    </div>
  )
}
