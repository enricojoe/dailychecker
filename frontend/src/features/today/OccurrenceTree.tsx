/**
 * Renders the occurrence tree with:
 * - Expand/collapse for nodes with children
 * - Accessible tri-state checkbox (pending/partial/done) per node
 *   role="checkbox" aria-checked: false|mixed|true
 * - Keyboard operable (Space/Enter cycle state)
 */

import { useState } from 'react'
import { cn } from '@/lib/utils'
import { useSetOccurrenceState, nextOccurrenceState } from './hooks'
import type { OccurrenceNode, OccurrenceState } from './types'

function ariaChecked(state: OccurrenceState): 'false' | 'mixed' | 'true' {
  if (state === 'done') return 'true'
  if (state === 'partial') return 'mixed'
  return 'false'
}

function stateLabel(state: OccurrenceState): string {
  if (state === 'done') return 'Done'
  if (state === 'partial') return 'Partial'
  return 'Pending'
}

const STATE_STYLES: Record<OccurrenceState, string> = {
  pending:
    'border-border bg-background text-muted-foreground hover:border-ring',
  partial:
    'border-amber-400 bg-amber-50 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
  done:
    'border-green-500 bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300',
}

interface OccurrenceRowProps {
  node: OccurrenceNode
  depth: number
  readOnly?: boolean
}

function OccurrenceRow({ node, depth }: OccurrenceRowProps) {
  const [expanded, setExpanded] = useState(true)
  const { mutate, isPending } = useSetOccurrenceState()
  const hasChildren = node.children.length > 0

  function cycleState() {
    mutate({ id: node.id, state: nextOccurrenceState(node.state) })
  }

  return (
    <li>
      <div
        className={cn(
          'flex items-center gap-3 rounded-lg px-3 py-2.5',
          depth > 0 && 'ml-6 border-l border-border pl-4',
          isPending && 'opacity-70'
        )}
      >
        {/* Tri-state checkbox */}
        <button
          role="checkbox"
          aria-checked={ariaChecked(node.state)}
          aria-label={`${node.title}: ${stateLabel(node.state)}. Click to set ${stateLabel(nextOccurrenceState(node.state))}`}
          onClick={cycleState}
          disabled={isPending}
          className={cn(
            'flex h-5 w-5 flex-shrink-0 items-center justify-center rounded border-2 transition-colors',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1',
            STATE_STYLES[node.state]
          )}
        >
          {node.state === 'done' && (
            <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none">
              <path d="M2 6l3 3 5-5" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          )}
          {node.state === 'partial' && (
            <span className="block h-0.5 w-2.5 rounded-full bg-current" />
          )}
        </button>

        {/* Title */}
        <span
          className={cn(
            'flex-1 text-sm',
            node.state === 'done' && 'text-muted-foreground line-through'
          )}
        >
          {node.title}
        </span>

        {/* Expand/collapse toggle for nodes with children */}
        {hasChildren && (
          <button
            type="button"
            onClick={() => setExpanded((p) => !p)}
            aria-expanded={expanded}
            aria-label={expanded ? 'Collapse sub-tasks' : 'Expand sub-tasks'}
            className="text-muted-foreground hover:text-foreground"
          >
            <svg
              className={cn('h-4 w-4 transition-transform', expanded && 'rotate-90')}
              fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
            </svg>
          </button>
        )}
      </div>

      {/* Children */}
      {hasChildren && expanded && (
        <ul>
          {node.children.map((child) => (
            <OccurrenceRow key={child.id} node={child} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  )
}

interface OccurrenceTreeProps {
  nodes: OccurrenceNode[]
}

export function OccurrenceTree({ nodes }: OccurrenceTreeProps) {
  if (nodes.length === 0) {
    return (
      <div className="flex h-32 items-center justify-center rounded-lg border border-dashed border-border">
        <p className="text-sm text-muted-foreground">Nothing due today.</p>
      </div>
    )
  }

  return (
    <ul className="space-y-1">
      {nodes.map((node) => (
        <OccurrenceRow key={node.id} node={node} depth={0} />
      ))}
    </ul>
  )
}
