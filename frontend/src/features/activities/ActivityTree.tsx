/**
 * Renders the activity tree: top-level nodes with expandable children.
 * Each node has edit / delete (with inline confirm) / add-sub-activity actions.
 * Sub-activity button only appears on top-level nodes (single-level depth).
 */

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { ActivityForm } from './ActivityForm'
import { useDeleteActivity } from './hooks'
import type { ActivityNode } from './types'
import { cn } from '@/lib/utils'

interface ActivityTreeProps {
  nodes: ActivityNode[]
}

type FormMode =
  | { kind: 'create-child'; parentId: string }
  | { kind: 'edit'; node: ActivityNode }
  | null

export function ActivityTree({ nodes }: ActivityTreeProps) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [formMode, setFormMode] = useState<FormMode>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const deleteMutation = useDeleteActivity()

  function toggleExpand(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function handleDeleteConfirm(id: string) {
    deleteMutation.mutate(id, {
      onSuccess: () => setConfirmDelete(null),
      onError: () => setConfirmDelete(null),
    })
  }

  if (nodes.length === 0) {
    return (
      <div className="flex h-24 items-center justify-center rounded-lg border border-dashed border-border">
        <p className="text-sm text-muted-foreground">No activities yet. Create one above.</p>
      </div>
    )
  }

  return (
    <ul className="space-y-2">
      {nodes.map((node) => {
        const isExpanded = expanded.has(node.id)
        const hasChildren = node.children.length > 0
        const isEditing = formMode?.kind === 'edit' && formMode.node.id === node.id
        const isAddingChild = formMode?.kind === 'create-child' && formMode.parentId === node.id
        const isDeleting = confirmDelete === node.id

        return (
          <li key={node.id}>
            <div className="rounded-lg border border-border bg-card px-4 py-3">
              {/* Node header */}
              <div className="flex items-start justify-between gap-3">
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  {hasChildren && (
                    <button
                      type="button"
                      onClick={() => toggleExpand(node.id)}
                      aria-expanded={isExpanded}
                      aria-label={isExpanded ? 'Collapse children' : 'Expand children'}
                      className="flex-shrink-0 text-muted-foreground hover:text-foreground"
                    >
                      <svg
                        className={cn('h-4 w-4 transition-transform', isExpanded && 'rotate-90')}
                        fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
                      >
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                      </svg>
                    </button>
                  )}
                  <div className="min-w-0">
                    <p className={cn('truncate text-sm font-medium', !node.is_active && 'text-muted-foreground line-through')}>
                      {node.title}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {node.freq === 'weekly'
                        ? `Weekly · ${node.days_of_week.map(d => ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'][d]).join(', ')}`
                        : 'Daily'
                      }
                      {' · '}{node.time_of_day.slice(0, 5)}
                    </p>
                  </div>
                </div>

                {/* Actions */}
                <div className="flex flex-shrink-0 items-center gap-1">
                  {!node.parent_id && (
                    <Button
                      size="xs"
                      variant="ghost"
                      type="button"
                      onClick={() => setFormMode({ kind: 'create-child', parentId: node.id })}
                      disabled={deleteMutation.isPending}
                    >
                      + Sub
                    </Button>
                  )}
                  <Button
                    size="xs"
                    variant="outline"
                    type="button"
                    onClick={() => setFormMode({ kind: 'edit', node })}
                    disabled={deleteMutation.isPending}
                  >
                    Edit
                  </Button>
                  {isDeleting ? (
                    <span className="flex items-center gap-1">
                      <Button
                        size="xs"
                        variant="destructive"
                        type="button"
                        onClick={() => handleDeleteConfirm(node.id)}
                        disabled={deleteMutation.isPending}
                      >
                        {deleteMutation.isPending ? '…' : 'Yes, delete'}
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        type="button"
                        onClick={() => setConfirmDelete(null)}
                        disabled={deleteMutation.isPending}
                      >
                        Cancel
                      </Button>
                    </span>
                  ) : (
                    <Button
                      size="xs"
                      variant="ghost"
                      type="button"
                      onClick={() => setConfirmDelete(node.id)}
                      className="text-destructive hover:text-destructive"
                      disabled={deleteMutation.isPending}
                    >
                      Delete
                    </Button>
                  )}
                </div>
              </div>

              {/* Inline edit form */}
              {isEditing && (
                <div className="mt-4 border-t border-border pt-4">
                  <ActivityForm
                    initial={formMode.kind === 'edit' ? formMode.node : undefined}
                    onDone={() => setFormMode(null)}
                  />
                </div>
              )}

              {/* Inline add-child form */}
              {isAddingChild && (
                <div className="mt-4 border-t border-border pt-4">
                  <p className="mb-3 text-xs font-medium text-muted-foreground">
                    New sub-activity under "{node.title}"
                  </p>
                  <ActivityForm
                    parentId={node.id}
                    onDone={() => setFormMode(null)}
                  />
                </div>
              )}

              {/* Children */}
              {hasChildren && isExpanded && (
                <ul className="mt-3 space-y-1.5 border-t border-border pt-3 pl-4">
                  {node.children.map((child) => {
                    const isEditingChild = formMode?.kind === 'edit' && formMode.node.id === child.id
                    const isDeletingChild = confirmDelete === child.id

                    return (
                      <li key={child.id} className="rounded-md border border-border/60 bg-background px-3 py-2">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className={cn('truncate text-sm', !child.is_active && 'text-muted-foreground line-through')}>
                              {child.title}
                            </p>
                            <p className="mt-0.5 text-xs text-muted-foreground">
                              {child.freq === 'weekly'
                                ? `Weekly · ${child.days_of_week.map(d => ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'][d]).join(', ')}`
                                : 'Daily'
                              }
                              {' · '}{child.time_of_day.slice(0, 5)}
                            </p>
                          </div>
                          <div className="flex flex-shrink-0 items-center gap-1">
                            <Button
                              size="xs"
                              variant="outline"
                              type="button"
                              onClick={() => setFormMode({ kind: 'edit', node: child })}
                              disabled={deleteMutation.isPending}
                            >
                              Edit
                            </Button>
                            {isDeletingChild ? (
                              <span className="flex items-center gap-1">
                                <Button
                                  size="xs"
                                  variant="destructive"
                                  type="button"
                                  onClick={() => handleDeleteConfirm(child.id)}
                                  disabled={deleteMutation.isPending}
                                >
                                  {deleteMutation.isPending ? '…' : 'Yes, delete'}
                                </Button>
                                <Button
                                  size="xs"
                                  variant="outline"
                                  type="button"
                                  onClick={() => setConfirmDelete(null)}
                                  disabled={deleteMutation.isPending}
                                >
                                  Cancel
                                </Button>
                              </span>
                            ) : (
                              <Button
                                size="xs"
                                variant="ghost"
                                type="button"
                                onClick={() => setConfirmDelete(child.id)}
                                className="text-destructive hover:text-destructive"
                                disabled={deleteMutation.isPending}
                              >
                                Delete
                              </Button>
                            )}
                          </div>
                        </div>
                        {isEditingChild && (
                          <div className="mt-3 border-t border-border pt-3">
                            <ActivityForm
                              initial={formMode.kind === 'edit' ? formMode.node : undefined}
                              onDone={() => setFormMode(null)}
                            />
                          </div>
                        )}
                      </li>
                    )
                  })}
                </ul>
              )}
            </div>
          </li>
        )
      })}
    </ul>
  )
}
