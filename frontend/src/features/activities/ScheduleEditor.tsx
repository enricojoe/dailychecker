/**
 * Controlled schedule editor: freq toggle, weekday multiselect (weekly only),
 * time picker, notes, is_active, sort_order.
 */

import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

const WEEKDAYS = [
  { label: 'Sun', value: 0 },
  { label: 'Mon', value: 1 },
  { label: 'Tue', value: 2 },
  { label: 'Wed', value: 3 },
  { label: 'Thu', value: 4 },
  { label: 'Fri', value: 5 },
  { label: 'Sat', value: 6 },
] as const

export interface ScheduleValues {
  freq: 'daily' | 'weekly'
  days_of_week: number[]
  time_of_day: string  // "HH:MM"
  notes: string
  is_active: boolean
  sort_order: number
}

interface ScheduleEditorProps {
  values: ScheduleValues
  onChange: (next: ScheduleValues) => void
  disabled?: boolean
}

export function ScheduleEditor({ values, onChange, disabled = false }: ScheduleEditorProps) {
  function set<K extends keyof ScheduleValues>(key: K, val: ScheduleValues[K]) {
    onChange({ ...values, [key]: val })
  }

  function toggleDay(day: number) {
    const next = values.days_of_week.includes(day)
      ? values.days_of_week.filter((d) => d !== day)
      : [...values.days_of_week, day]
    set('days_of_week', next)
  }

  function handleFreqChange(freq: 'daily' | 'weekly') {
    onChange({
      ...values,
      freq,
      // Clear days when switching to daily; reset to empty when switching to weekly
      days_of_week: freq === 'daily' ? [] : values.days_of_week,
    })
  }

  return (
    <div className="space-y-4">
      {/* Frequency toggle */}
      <div className="space-y-1.5">
        <Label>Frequency</Label>
        <div className="flex gap-2">
          {(['daily', 'weekly'] as const).map((f) => (
            <button
              key={f}
              type="button"
              disabled={disabled}
              onClick={() => handleFreqChange(f)}
              className={cn(
                'rounded-lg border px-4 py-1.5 text-sm font-medium transition-colors',
                values.freq === f
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'border-border bg-background text-muted-foreground hover:text-foreground'
              )}
            >
              {f.charAt(0).toUpperCase() + f.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Weekday picker — only for weekly */}
      {values.freq === 'weekly' && (
        <div className="space-y-1.5">
          <Label>Days of week</Label>
          <div className="flex flex-wrap gap-1.5" role="group" aria-label="Days of week">
            {WEEKDAYS.map(({ label, value }) => {
              const checked = values.days_of_week.includes(value)
              return (
                <button
                  key={value}
                  type="button"
                  disabled={disabled}
                  aria-pressed={checked}
                  onClick={() => toggleDay(value)}
                  className={cn(
                    'h-8 w-10 rounded-md border text-xs font-medium transition-colors',
                    checked
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-border bg-background text-muted-foreground hover:text-foreground'
                  )}
                >
                  {label}
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Time of day */}
      <div className="space-y-1.5">
        <Label htmlFor="schedule-time">Time of day</Label>
        <Input
          id="schedule-time"
          type="time"
          value={values.time_of_day}
          onChange={(e) => set('time_of_day', e.target.value)}
          disabled={disabled}
          className="w-36"
        />
      </div>

      {/* Notes */}
      <div className="space-y-1.5">
        <Label htmlFor="schedule-notes">Notes (optional)</Label>
        <textarea
          id="schedule-notes"
          value={values.notes}
          onChange={(e) => set('notes', e.target.value)}
          disabled={disabled}
          rows={2}
          placeholder="Any extra notes…"
          className={cn(
            'flex w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground shadow-sm transition-colors',
            'placeholder:text-muted-foreground',
            'focus-visible:outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
            'disabled:cursor-not-allowed disabled:opacity-50',
            'resize-none'
          )}
        />
      </div>

      {/* Sort order */}
      <div className="space-y-1.5">
        <Label htmlFor="schedule-sort">Sort order</Label>
        <Input
          id="schedule-sort"
          type="number"
          min={0}
          value={values.sort_order}
          onChange={(e) => set('sort_order', Number(e.target.value))}
          disabled={disabled}
          className="w-24"
        />
      </div>

      {/* is_active */}
      <div className="flex items-center gap-2">
        <input
          id="schedule-active"
          type="checkbox"
          checked={values.is_active}
          onChange={(e) => set('is_active', e.target.checked)}
          disabled={disabled}
          className="h-4 w-4 rounded border-border accent-primary"
        />
        <Label htmlFor="schedule-active">Active (included in Today)</Label>
      </div>
    </div>
  )
}
