/**
 * Create / edit form for an Activity.
 * Pass `initial` to edit; omit for create.
 * Pass `parentId` to create a sub-activity under a top-level parent.
 */

import { useState } from 'react'
import type { FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScheduleEditor } from './ScheduleEditor'
import type { ScheduleValues } from './ScheduleEditor'
import { useCreateActivity, useUpdateActivity } from './hooks'
import type { Activity } from './types'

interface ActivityFormProps {
  /** Existing activity being edited. Omit for create. */
  initial?: Activity
  /** Pre-set parent_id for sub-activity creation. */
  parentId?: string
  onDone: () => void
}

function defaultSchedule(a?: Activity): ScheduleValues {
  return {
    freq: a?.freq ?? 'daily',
    days_of_week: a?.days_of_week ?? [],
    // Strip seconds if present (backend returns HH:MM:SS, input needs HH:MM)
    time_of_day: (a?.time_of_day ?? '08:00').slice(0, 5),
    notes: a?.notes ?? '',
    is_active: a?.is_active ?? true,
    sort_order: a?.sort_order ?? 0,
  }
}

export function ActivityForm({ initial, parentId, onDone }: ActivityFormProps) {
  const [title, setTitle] = useState(initial?.title ?? '')
  const [titleError, setTitleError] = useState<string | null>(null)
  const [schedule, setSchedule] = useState<ScheduleValues>(() => defaultSchedule(initial))
  const [serverError, setServerError] = useState<string | null>(null)

  const createMutation = useCreateActivity()
  const updateMutation = useUpdateActivity()
  const isPending = createMutation.isPending || updateMutation.isPending

  function validate(): boolean {
    if (!title.trim()) {
      setTitleError('Title is required.')
      return false
    }
    if (schedule.freq === 'weekly' && schedule.days_of_week.length === 0) {
      setServerError('Select at least one day for weekly activities.')
      return false
    }
    setTitleError(null)
    setServerError(null)
    return true
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!validate()) return

    const dto = {
      title: title.trim(),
      notes: schedule.notes.trim() || undefined,
      freq: schedule.freq,
      days_of_week: schedule.freq === 'weekly' ? schedule.days_of_week : [],
      time_of_day: schedule.time_of_day,
      sort_order: schedule.sort_order,
      is_active: schedule.is_active,
    }

    try {
      if (initial) {
        await updateMutation.mutateAsync({ id: initial.id, dto })
      } else {
        await createMutation.mutateAsync({ ...dto, parent_id: parentId })
      }
      onDone()
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Something went wrong.')
    }
  }

  return (
    <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4" noValidate>
      <div className="space-y-1.5">
        <Label htmlFor="activity-title">Title</Label>
        <Input
          id="activity-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          disabled={isPending}
          aria-invalid={!!titleError}
          aria-describedby={titleError ? 'title-error' : undefined}
          placeholder="e.g. Morning run"
        />
        {titleError && (
          <p id="title-error" role="alert" className="text-xs text-destructive">
            {titleError}
          </p>
        )}
      </div>

      <ScheduleEditor values={schedule} onChange={setSchedule} disabled={isPending} />

      {serverError && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {serverError}
        </p>
      )}

      <div className="flex gap-2 pt-1">
        <Button type="submit" disabled={isPending}>
          {isPending ? 'Saving…' : initial ? 'Save changes' : 'Create activity'}
        </Button>
        <Button type="button" variant="outline" onClick={onDone} disabled={isPending}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
