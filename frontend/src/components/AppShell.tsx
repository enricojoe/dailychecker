/**
 * Authenticated app shell: sticky header + top nav + content outlet.
 * Nav links for Activities / History / Telegram are placeholders wired in M8.
 */

import { NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const NAV_ITEMS = [
  { label: 'Today', to: '/', m8: false },
  { label: 'Activities', to: '/activities', m8: true },
  { label: 'History', to: '/history', m8: true },
  { label: 'Telegram', to: '/telegram', m8: true },
] as const

export function AppShell() {
  const { user, logout } = useAuth()

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      {/* ── Header ───────────────────────────────────────────────────────── */}
      <header className="sticky top-0 z-10 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
          <span className="text-base font-semibold tracking-tight">
            DailyChecker
          </span>
          <div className="flex items-center gap-3">
            {user && (
              <span className="hidden text-sm text-muted-foreground sm:inline">
                {user.name}
              </span>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => void logout()}
            >
              Logout
            </Button>
          </div>
        </div>
      </header>

      {/* ── Nav ──────────────────────────────────────────────────────────── */}
      <nav aria-label="Main navigation" className="border-b border-border">
        <div className="mx-auto flex max-w-5xl gap-0.5 px-4">
          {NAV_ITEMS.map(({ label, to, m8 }) =>
            m8 ? (
              <span
                key={to}
                className="cursor-not-allowed px-3 py-2.5 text-sm text-muted-foreground/50"
                aria-disabled="true"
                title="Coming in M8"
              >
                {label}
              </span>
            ) : (
              <NavLink
                key={to}
                to={to}
                end
                className={({ isActive }) =>
                  cn(
                    'px-3 py-2.5 text-sm transition-colors',
                    isActive
                      ? 'border-b-2 border-primary font-medium text-foreground'
                      : 'text-muted-foreground hover:text-foreground'
                  )
                }
              >
                {label}
              </NavLink>
            )
          )}
        </div>
      </nav>

      {/* ── Content ──────────────────────────────────────────────────────── */}
      <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
