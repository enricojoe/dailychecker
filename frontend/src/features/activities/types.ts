export interface Activity {
  id: string
  user_id: string
  parent_id?: string
  title: string
  notes?: string
  freq: 'daily' | 'weekly'
  days_of_week: number[]  // 0=Sun..6=Sat
  time_of_day: string     // "HH:MM:SS"
  sort_order: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface ActivityNode extends Activity {
  children: ActivityNode[]
}

export interface CreateActivityDto {
  parent_id?: string
  title: string
  notes?: string
  freq: 'daily' | 'weekly'
  days_of_week?: number[]
  time_of_day: string
  sort_order?: number
  is_active?: boolean
}

export interface UpdateActivityDto {
  title?: string
  notes?: string
  freq?: 'daily' | 'weekly'
  days_of_week?: number[]
  time_of_day?: string
  sort_order?: number
  is_active?: boolean
  parent_id?: string
}
