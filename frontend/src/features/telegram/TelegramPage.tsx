/**
 * /telegram — Telegram connection page.
 *
 * States:
 *  1. Connected  — user.telegram_chat_id is set → show confirmation + linked_at
 *  2. Not linked — show "Connect Telegram" button
 *     a. POST succeeds → show deep-link URL + instructions; poll /me
 *     b. POST returns 404 → "Telegram not configured on this server"
 *     c. Other error → display error message
 */

import { useAuth } from '@/auth/AuthContext'
import { Button } from '@/components/ui/button'
import { useTelegramLink } from './hooks'
import { ApiError } from '@/api/apiClient'
import { formatDisplayDate } from '@/lib/dateUtils'

export function TelegramPage() {
  const { user } = useAuth()
  const linkMutation = useTelegramLink()

  const isConnected = !!user?.telegram_chat_id
  const is404 = linkMutation.error instanceof ApiError && linkMutation.error.status === 404

  // Format the linked_at date for display
  const linkedAtDisplay = user?.telegram_linked_at
    ? formatDisplayDate(user.telegram_linked_at.slice(0, 10))
    : null

  if (isConnected) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">Telegram</h1>
        <div className="rounded-xl border border-green-200 bg-green-50 px-5 py-4 dark:border-green-900 dark:bg-green-950/30">
          <p className="text-sm font-medium text-green-800 dark:text-green-300">Connected</p>
          {linkedAtDisplay && (
            <p className="mt-1 text-xs text-green-700 dark:text-green-400">
              Linked on {linkedAtDisplay}
            </p>
          )}
          <p className="mt-2 text-xs text-muted-foreground">
            You will receive activity reminders and updates via Telegram.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Telegram</h1>

      {/* Not yet linked + no pending link */}
      {!linkMutation.data && !linkMutation.isPending && (
        <div className="rounded-xl border border-border bg-card px-5 py-5 space-y-3">
          <p className="text-sm text-muted-foreground">
            Connect your Telegram account to receive daily reminders and activity updates.
          </p>

          {/* 404: bot not configured */}
          {is404 && (
            <p role="alert" className="rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
              Telegram is not configured on this server. Contact your administrator.
            </p>
          )}

          {/* Other errors */}
          {linkMutation.error && !is404 && (
            <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {linkMutation.error.message}
            </p>
          )}

          <Button
            onClick={() => linkMutation.mutate()}
            disabled={linkMutation.isPending}
          >
            Connect Telegram
          </Button>
        </div>
      )}

      {/* Generating link */}
      {linkMutation.isPending && (
        <p className="text-sm text-muted-foreground">Generating link…</p>
      )}

      {/* Link ready */}
      {linkMutation.data && (
        <div className="rounded-xl border border-border bg-card px-5 py-5 space-y-4">
          <p className="text-sm font-medium">Almost there — open the link below in Telegram:</p>

          <a
            href={linkMutation.data.url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 rounded-lg border border-primary bg-primary/5 px-4 py-2.5 text-sm font-medium text-primary hover:bg-primary/10 transition-colors"
          >
            Open DailyChecker Bot in Telegram
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
            </svg>
          </a>

          <ol className="list-decimal list-inside space-y-1 text-sm text-muted-foreground">
            <li>Click the link above to open Telegram.</li>
            <li>Send <code className="rounded bg-muted px-1 py-0.5 text-xs">/start</code> to the bot.</li>
            <li>Return here — this page will update automatically once linked.</li>
          </ol>

          <Button
            variant="outline"
            size="sm"
            onClick={() => linkMutation.reset()}
          >
            Generate new link
          </Button>
        </div>
      )}
    </div>
  )
}
