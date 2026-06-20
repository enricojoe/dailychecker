/**
 * Jakarta (WIB, UTC+7) date helpers.
 * All API date params are YYYY-MM-DD strings in the Jakarta calendar.
 */

export function todayJakarta(): string {
  const now = new Date()
  const jakarta = new Date(now.getTime() + 7 * 60 * 60 * 1000)
  return jakarta.toISOString().slice(0, 10)
}

export const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
] as const

export const DAY_LABELS_SHORT = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] as const

export interface DayCell {
  date: string      // YYYY-MM-DD
  inMonth: boolean
}

export interface MonthGrid {
  year: number
  month: number   // 0-indexed
  days: DayCell[]
  from: string    // first day of grid
  to: string      // last day of grid
}

/**
 * Builds a Mon-anchored 6-week (at most) calendar grid for the given month.
 * Returned `from`/`to` are the full grid bounds for use as API query params.
 */
export function buildMonthGrid(year: number, month: number): MonthGrid {
  const firstOfMonth = new Date(Date.UTC(year, month, 1))
  const lastOfMonth = new Date(Date.UTC(year, month + 1, 0))

  // Shift so Mon = 0 (getUTCDay: 0=Sun,1=Mon..6=Sat)
  const startDow = (firstOfMonth.getUTCDay() + 6) % 7
  const gridStart = new Date(firstOfMonth)
  gridStart.setUTCDate(gridStart.getUTCDate() - startDow)

  const endDow = (lastOfMonth.getUTCDay() + 6) % 7
  const gridEnd = new Date(lastOfMonth)
  gridEnd.setUTCDate(gridEnd.getUTCDate() + (6 - endDow))

  const days: DayCell[] = []
  const cursor = new Date(gridStart)
  while (cursor <= gridEnd) {
    days.push({
      date: cursor.toISOString().slice(0, 10),
      inMonth: cursor.getUTCMonth() === month,
    })
    cursor.setUTCDate(cursor.getUTCDate() + 1)
  }

  return {
    year,
    month,
    days,
    from: gridStart.toISOString().slice(0, 10),
    to: gridEnd.toISOString().slice(0, 10),
  }
}

export function prevMonth(year: number, month: number): { year: number; month: number } {
  if (month === 0) return { year: year - 1, month: 11 }
  return { year, month: month - 1 }
}

export function nextMonth(year: number, month: number): { year: number; month: number } {
  if (month === 11) return { year: year + 1, month: 0 }
  return { year, month: month + 1 }
}

/** Format a YYYY-MM-DD string for display (e.g. "June 20, 2026"). */
export function formatDisplayDate(date: string): string {
  const [year, month, day] = date.split('-').map(Number)
  return `${MONTH_NAMES[(month ?? 1) - 1]} ${day}, ${year}`
}
