/**
 * /history — Two-tab history page: Calendar and By Activity.
 */

import { useState } from 'react'
import { CalendarView } from './CalendarView'
import { ByActivityView } from './ByActivityView'
import { cn } from '@/lib/utils'

type Tab = 'calendar' | 'by-activity'

const TABS: { id: Tab; label: string }[] = [
  { id: 'calendar', label: 'Calendar' },
  { id: 'by-activity', label: 'By Activity' },
]

export function HistoryPage() {
  const [activeTab, setActiveTab] = useState<Tab>('calendar')

  return (
    <div className="space-y-5">
      <h1 className="text-xl font-semibold">History</h1>

      {/* Tab bar */}
      <div className="flex gap-0.5 border-b border-border" role="tablist" aria-label="History views">
        {TABS.map(({ id, label }) => (
          <button
            key={id}
            role="tab"
            type="button"
            aria-selected={activeTab === id}
            aria-controls={`tabpanel-${id}`}
            id={`tab-${id}`}
            onClick={() => setActiveTab(id)}
            className={cn(
              'px-4 py-2.5 text-sm transition-colors',
              activeTab === id
                ? 'border-b-2 border-primary font-medium text-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Panels */}
      <div
        id="tabpanel-calendar"
        role="tabpanel"
        aria-labelledby="tab-calendar"
        hidden={activeTab !== 'calendar'}
      >
        {activeTab === 'calendar' && <CalendarView />}
      </div>
      <div
        id="tabpanel-by-activity"
        role="tabpanel"
        aria-labelledby="tab-by-activity"
        hidden={activeTab !== 'by-activity'}
      >
        {activeTab === 'by-activity' && <ByActivityView />}
      </div>
    </div>
  )
}
