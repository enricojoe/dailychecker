/**
 * Authenticated app shell: sticky header + top nav + content outlet.
 * Nav links for Activities / History / Telegram are placeholders wired in M8.
 */

import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext'
import { cn } from '@/lib/utils'

const NAV_ITEMS = [
  { label: 'Today', to: '/' },
  { label: 'Activities', to: '/activities' },
  { label: 'History', to: '/history' },
  { label: 'Telegram', to: '/telegram' },
] as const

function UserMenu() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [open])

  if (!user) return null

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(v => !v)}
        className="text-sm text-muted-foreground hover:text-foreground"
      >
        {user.name}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-2 w-36 rounded-md border border-border bg-background shadow-md">
          <button
            className="w-full px-4 py-2 text-left text-sm hover:bg-muted"
            onClick={() => { setOpen(false); navigate('/profile') }}
          >
            Profile
          </button>
          <button
            className="w-full px-4 py-2 text-left text-sm hover:bg-muted"
            onClick={() => { setOpen(false); void logout() }}
          >
            Logout
          </button>
        </div>
      )}
    </div>
  )
}

export function AppShell() {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      {/* ── Header ───────────────────────────────────────────────────────── */}
      <header className="sticky top-0 z-10 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
          <span className="text-base font-semibold tracking-tight">
            DailyChecker
          </span>
          <UserMenu />
        </div>
      </header>

      {/* ── Nav ──────────────────────────────────────────────────────────── */}
      <nav aria-label="Main navigation" className="border-b border-border">
        <div className="mx-auto flex max-w-5xl gap-0.5 px-4">
          {NAV_ITEMS.map(({ label, to }) => (
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
          ))}
        </div>
      </nav>

      {/* ── Content ──────────────────────────────────────────────────────── */}
      <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
