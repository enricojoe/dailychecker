/**
 * /  — Today dashboard.
 * Shows today's occurrence tree plus an inline "Add activity" panel so a new
 * activity (with optional sub-activities) can be created without leaving the
 * dashboard. Creating an activity invalidates both the activities and today
 * queries, so the tree refreshes automatically.
 */

import { useEffect, useState } from 'react'
import { OccurrenceTree } from './OccurrenceTree'
import { useToday } from './hooks'
import { todayJakarta } from '@/lib/dateUtils'
import { Button } from '@/components/ui/button'
import { ActivityForm } from '@/features/activities/ActivityForm'

function jakartaTime(): string {
  const now = new Date()
  const jakarta = new Date(now.getTime() + 7 * 60 * 60 * 1000)
  return jakarta.toISOString().slice(11, 19) // HH:MM:SS
}

export function TodayPage() {
  const { data, isPending, isError, error } = useToday()
  const [showCreate, setShowCreate] = useState(false)
  const [clock, setClock] = useState(jakartaTime)

  useEffect(() => {
    const id = setInterval(() => setClock(jakartaTime()), 1000)
    return () => clearInterval(id)
  }, [])

  const today = todayJakarta()
  const [year, month, day] = today.split('-')
  const displayDate = `${day} ${['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][Number(month) - 1]} ${year}`

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Today</h1>
          <p className="text-sm text-muted-foreground">{displayDate}</p>
          <p className="font-mono text-2xl font-semibold tabular-nums">{clock}</p>
        </div>
        {!showCreate && (
          <Button size="sm" onClick={() => setShowCreate(true)}>
            Add activity
          </Button>
        )}
      </div>

      {/* Inline create panel */}
      {showCreate && (
        <div className="rounded-xl border border-border bg-card px-5 py-5">
          <h2 className="mb-4 text-sm font-semibold">New activity</h2>
          <ActivityForm onDone={() => setShowCreate(false)} />
        </div>
      )}

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
