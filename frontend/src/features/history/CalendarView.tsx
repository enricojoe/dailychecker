/**
 * Calendar history view.
 * Month grid with prev/next navigation.
 * Each day cell shows pending/partial/done counts from DaySummary.
 * Clicking a day shows that day's occurrence tree read-only.
 */

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { OccurrenceTree } from '@/features/today/OccurrenceTree'
import { useCalendar, useCalendarDay } from './hooks'
import {
  buildMonthGrid,
  prevMonth,
  nextMonth,
  MONTH_NAMES,
  DAY_LABELS_SHORT,
  todayJakarta,
  formatDisplayDate,
} from '@/lib/dateUtils'
import type { DaySummary } from './types'
import { cn } from '@/lib/utils'

function daySummaryColor(summary: DaySummary | undefined): string {
  if (!summary || summary.total === 0) return ''
  if (summary.done === summary.total) return 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300'
  if (summary.partial > 0 || summary.done > 0) return 'bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300'
  return 'bg-muted text-muted-foreground'
}

function DayDetail({ date }: { date: string }) {
  const { data, isPending, isError, error } = useCalendarDay(date)

  return (
    <div className="mt-4 rounded-xl border border-border bg-card px-5 py-4">
      <h3 className="mb-3 text-sm font-semibold">{formatDisplayDate(date)}</h3>
      {isPending && <p className="text-sm text-muted-foreground">Loading…</p>}
      {isError && (
        <p className="text-sm text-destructive">
          {error instanceof Error ? error.message : 'Failed to load.'}
        </p>
      )}
      {data && data.length === 0 && (
        <p className="text-sm text-muted-foreground">No occurrences on this day.</p>
      )}
      {data && data.length > 0 && <OccurrenceTree nodes={data} />}
    </div>
  )
}

export function CalendarView() {
  const today = todayJakarta()
  const nowDate = new Date(today + 'T00:00:00Z')
  const [year, setYear] = useState(nowDate.getUTCFullYear())
  const [month, setMonth] = useState(nowDate.getUTCMonth())
  const [selectedDate, setSelectedDate] = useState<string | null>(null)

  const grid = buildMonthGrid(year, month)
  const { data: summaries, isPending, isError } = useCalendar(grid.from, grid.to)

  const summaryMap = new Map<string, DaySummary>()
  summaries?.forEach((s) => summaryMap.set(s.date, s))

  function handlePrev() {
    const p = prevMonth(year, month)
    setYear(p.year)
    setMonth(p.month)
    setSelectedDate(null)
  }
  function handleNext() {
    const n = nextMonth(year, month)
    setYear(n.year)
    setMonth(n.month)
    setSelectedDate(null)
  }

  return (
    <div className="space-y-4">
      {/* Month nav */}
      <div className="flex items-center gap-3">
        <Button variant="outline" size="sm" onClick={handlePrev} aria-label="Previous month">
          ‹
        </Button>
        <span className="min-w-36 text-center text-sm font-semibold">
          {MONTH_NAMES[month]} {year}
        </span>
        <Button variant="outline" size="sm" onClick={handleNext} aria-label="Next month">
          ›
        </Button>
        {isPending && <span className="text-xs text-muted-foreground">Loading…</span>}
        {isError && <span className="text-xs text-destructive">Failed to load.</span>}
      </div>

      {/* Grid header */}
      <div className="grid grid-cols-7 gap-1 text-center">
        {DAY_LABELS_SHORT.map((d) => (
          <div key={d} className="py-1 text-xs font-medium text-muted-foreground">
            {d}
          </div>
        ))}
      </div>

      {/* Day cells */}
      <div className="grid grid-cols-7 gap-1">
        {grid.days.map(({ date, inMonth }) => {
          const summary = summaryMap.get(date)
          const isToday = date === today
          const isSelected = date === selectedDate

          return (
            <button
              key={date}
              type="button"
              onClick={() => setSelectedDate(date === selectedDate ? null : date)}
              disabled={!inMonth}
              aria-label={`${date}${summary ? `: ${summary.done}/${summary.total} done` : ''}`}
              aria-pressed={isSelected}
              className={cn(
                'flex min-h-12 flex-col items-center justify-start rounded-md px-1 pt-1.5 text-xs transition-colors',
                !inMonth && 'invisible',
                inMonth && 'hover:bg-muted cursor-pointer',
                isToday && 'ring-2 ring-primary ring-offset-1',
                isSelected && 'bg-muted',
                summary ? daySummaryColor(summary) : ''
              )}
            >
              <span className={cn('font-medium', isToday && 'text-primary')}>
                {Number(date.split('-')[2])}
              </span>
              {summary && (
                <span className="mt-0.5 text-[10px]">
                  {summary.done}/{summary.total}
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* Day detail */}
      {selectedDate && <DayDetail date={selectedDate} />}
    </div>
  )
}
