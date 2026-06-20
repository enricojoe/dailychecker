export type OccurrenceState = 'pending' | 'partial' | 'done'

export interface OccurrenceNode {
  id: string
  activity_id: string
  title: string
  state: OccurrenceState
  completed_at?: string
  children: OccurrenceNode[]
}
