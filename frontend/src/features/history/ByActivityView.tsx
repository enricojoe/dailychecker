/**
 * By-Activity history view.
 * Pick an activity + date range → state timeline.
 */

import { useState } from 'react'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { useActivities } from '@/features/activities/hooks'
import { useActivityHistory } from './hooks'
import type { OccurrenceState } from './types'
import { todayJakarta } from '@/lib/dateUtils'
import { cn } from '@/lib/utils'

const STATE_BADGE: Record<OccurrenceState, string> = {
  done: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  partial: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
  pending: 'bg-muted text-muted-foreground',
}

/** Default date range: first of current month → today */
function defaultRange(): { from: string; to: string } {
  const today = todayJakarta()
  const [year, month] = today.split('-')
  return { from: `${year}-${month}-01`, to: today }
}

export function ByActivityView() {
  const { data: activityTree, isPending: activitiesPending } = useActivities()

  // Flatten tree to a single list for the select
  const activityList =
    activityTree?.flatMap((node) => [
      { id: node.id, title: node.title },
      ...node.children.map((c) => ({ id: c.id, title: `  ${node.title} › ${c.title}` })),
    ]) ?? []

  const [selectedId, setSelectedId] = useState<string>('')
  const range = defaultRange()
  const [from, setFrom] = useState(range.from)
  const [to, setTo] = useState(range.to)

  const { data: entries, isPending, isError, error } = useActivityHistory(
    selectedId || null,
    from,
    to
  )

  return (
    <div className="space-y-5">
      {/* Filters */}
      <div className="grid gap-4 sm:grid-cols-3">
        <div className="space-y-1.5">
          <Label htmlFor="by-activity-select">Activity</Label>
          <select
            id="by-activity-select"
            value={selectedId}
            onChange={(e) => setSelectedId(e.target.value)}
            disabled={activitiesPending}
            className={cn(
              'flex h-9 w-full rounded-lg border border-input bg-background px-3 py-1 text-sm text-foreground shadow-sm',
              'focus-visible:outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
              'disabled:cursor-not-allowed disabled:opacity-50'
            )}
          >
            <option value="">Select activity…</option>
            {activityList.map(({ id, title }) => (
              <option key={id} value={id}>{title}</option>
            ))}
          </select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="by-activity-from">From</Label>
          <Input
            id="by-activity-from"
            type="date"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="by-activity-to">To</Label>
          <Input
            id="by-activity-to"
            type="date"
            value={to}
            onChange={(e) => setTo(e.target.value)}
          />
        </div>
      </div>

      {/* Results */}
      {!selectedId && (
        <p className="text-sm text-muted-foreground">Select an activity to see its history.</p>
      )}

      {selectedId && isPending && (
        <p className="text-sm text-muted-foreground">Loading…</p>
      )}

      {selectedId && isError && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error instanceof Error ? error.message : 'Failed to load history.'}
        </p>
      )}

      {entries && entries.length === 0 && (
        <p className="text-sm text-muted-foreground">No history in this range.</p>
      )}

      {entries && entries.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">{entries.length} entries</p>
          <div className="divide-y divide-border rounded-lg border border-border">
            {entries.map(({ date, state }) => (
              <div key={date} className="flex items-center justify-between px-4 py-2.5">
                <span className="text-sm">{date}</span>
                <span
                  className={cn(
                    'rounded-full px-2 py-0.5 text-xs font-medium capitalize',
                    STATE_BADGE[state]
                  )}
                >
                  {state}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Clear button for convenience */}
      {selectedId && (
        <Button variant="ghost" size="sm" onClick={() => setSelectedId('')}>
          Clear selection
        </Button>
      )}
    </div>
  )
}
