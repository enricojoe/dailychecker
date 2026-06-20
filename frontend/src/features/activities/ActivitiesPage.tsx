/**
 * /activities — Activity CRUD page.
 * Shows a create form (collapsible) + the activity tree.
 */

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { ActivityForm } from './ActivityForm'
import { ActivityTree } from './ActivityTree'
import { useActivities } from './hooks'

export function ActivitiesPage() {
  const { data: nodes, isPending, isError, error } = useActivities()
  const [showCreate, setShowCreate] = useState(false)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Activities</h1>
        {!showCreate && (
          <Button size="sm" onClick={() => setShowCreate(true)}>
            New activity
          </Button>
        )}
      </div>

      {/* Create form */}
      {showCreate && (
        <div className="rounded-xl border border-border bg-card px-5 py-5">
          <h2 className="mb-4 text-sm font-semibold">New activity</h2>
          <ActivityForm onDone={() => setShowCreate(false)} />
        </div>
      )}

      {/* States */}
      {isPending && (
        <div className="flex h-24 items-center justify-center">
          <p className="text-sm text-muted-foreground">Loading activities…</p>
        </div>
      )}

      {isError && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error instanceof Error ? error.message : 'Failed to load activities.'}
        </p>
      )}

      {nodes && <ActivityTree nodes={nodes} />}
    </div>
  )
}
