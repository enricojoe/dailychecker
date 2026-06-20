/**
 * Today hooks.
 *
 * useSetOccurrenceState optimistic strategy:
 *   onMutate  — snapshot cache + optimistically update only the clicked node
 *   onSuccess — reconcile with the authoritative group tree returned by the server
 *               (server applies rollup: parent flips when all/some children done)
 *   onError   — rollback to snapshot + refetch
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { todayApi } from './api'
import type { OccurrenceNode, OccurrenceState } from './types'

export const TODAY_KEY = ['today'] as const

// ── Cache helpers ─────────────────────────────────────────────────────────────

/** Apply a single-node optimistic state change, walking the tree. */
function applyOptimistic(
  nodes: OccurrenceNode[],
  targetId: string,
  state: OccurrenceState
): OccurrenceNode[] {
  return nodes.map((node) => ({
    ...node,
    state: node.id === targetId ? state : node.state,
    children: applyOptimistic(node.children, targetId, state),
  }))
}

/** Flatten a returned group tree into a map of id → { state, completed_at }. */
function flattenGroup(
  nodes: OccurrenceNode[]
): Map<string, Pick<OccurrenceNode, 'state' | 'completed_at'>> {
  const map = new Map<string, Pick<OccurrenceNode, 'state' | 'completed_at'>>()
  function walk(node: OccurrenceNode) {
    map.set(node.id, { state: node.state, completed_at: node.completed_at })
    node.children.forEach(walk)
  }
  nodes.forEach(walk)
  return map
}

/** Apply the authoritative server updates to the cached tree. */
function reconcile(
  cached: OccurrenceNode[],
  updates: Map<string, Pick<OccurrenceNode, 'state' | 'completed_at'>>
): OccurrenceNode[] {
  return cached.map((node) => {
    const upd = updates.get(node.id)
    return {
      ...node,
      ...(upd ?? {}),
      children: reconcile(node.children, updates),
    }
  })
}

// ── Next state cycle: pending → partial → done → pending ──────────────────────

export function nextOccurrenceState(current: OccurrenceState): OccurrenceState {
  if (current === 'pending') return 'partial'
  if (current === 'partial') return 'done'
  return 'pending'
}

// ── Queries & mutations ───────────────────────────────────────────────────────

export function useToday() {
  return useQuery({
    queryKey: TODAY_KEY,
    queryFn: () => todayApi.list(),
    throwOnError: false,
  })
}

export function useSetOccurrenceState() {
  const qc = useQueryClient()

  return useMutation({
    mutationFn: ({ id, state }: { id: string; state: OccurrenceState }) =>
      todayApi.setState(id, state),

    onMutate: async ({ id, state }) => {
      // Cancel in-flight fetches so they don't stomp our optimistic update
      await qc.cancelQueries({ queryKey: TODAY_KEY })

      const previous = qc.getQueryData<OccurrenceNode[]>(TODAY_KEY)

      // Optimistically update only the clicked node
      qc.setQueryData<OccurrenceNode[]>(TODAY_KEY, (old) => {
        if (!old) return old
        return applyOptimistic(old, id, state)
      })

      return { previous }
    },

    onSuccess: (returnedGroup) => {
      // Reconcile with the authoritative server rollup
      const updates = flattenGroup(returnedGroup)
      qc.setQueryData<OccurrenceNode[]>(TODAY_KEY, (old) => {
        if (!old) return old
        return reconcile(old, updates)
      })
    },

    onError: (_err, _vars, context) => {
      // Rollback to the pre-optimistic snapshot
      if (context?.previous !== undefined) {
        qc.setQueryData(TODAY_KEY, context.previous)
      }
      // Ensure cache is fresh after rollback
      void qc.invalidateQueries({ queryKey: TODAY_KEY })
    },
  })
}
