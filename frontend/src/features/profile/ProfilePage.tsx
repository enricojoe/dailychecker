/**
 * /profile — account settings.
 *
 * Two independent forms:
 *   1. Profile  — full name + username (with live availability check)
 *   2. Password — current + new + confirm
 *
 * Only changed fields are sent. On success the ['me'] cache is updated so the
 * header reflects the change immediately (see useUpdateProfile).
 */

import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useAuth } from '@/auth/AuthContext'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LoadingScreen } from '@/components/LoadingScreen'
import { profileApi } from './api'
import { useUpdateProfile } from './hooks'

type UsernameStatus =
  | 'unchanged' // equals current username — no check needed, valid
  | 'invalid' // too short
  | 'checking'
  | 'available'
  | 'taken'
  | 'error'

export function ProfilePage() {
  const { user, isLoading } = useAuth()

  if (isLoading || !user) return <LoadingScreen />

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Profile</h1>
      <ProfileDetailsForm currentName={user.name} currentUsername={user.username} />
      <PasswordForm />
    </div>
  )
}

// ── Name + username ─────────────────────────────────────────────────────────

function ProfileDetailsForm({
  currentName,
  currentUsername,
}: {
  currentName: string
  currentUsername: string
}) {
  const [name, setName] = useState(currentName)
  const [username, setUsername] = useState(currentUsername)
  // Async availability result, tagged with the username it applies to so a
  // stale (debounced) result is never shown for the current input.
  const [check, setCheck] = useState<{
    name: string
    status: 'checking' | 'available' | 'taken' | 'error'
  } | null>(null)
  const [serverError, setServerError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const mutation = useUpdateProfile()

  const trimmedName = name.trim()
  const trimmedUsername = username.trim()
  const nameChanged = trimmedName !== currentName
  const usernameChanged = trimmedUsername !== currentUsername
  const nothingChanged = !nameChanged && !usernameChanged

  // Debounced username availability check. setState happens only inside async
  // callbacks (never synchronously in the effect body).
  useEffect(() => {
    if (!usernameChanged || trimmedUsername.length < 3) return
    let cancelled = false
    const handle = setTimeout(() => {
      setCheck({ name: trimmedUsername, status: 'checking' })
      profileApi
        .checkUsername(trimmedUsername)
        .then((res) => {
          if (!cancelled)
            setCheck({ name: trimmedUsername, status: res.available ? 'available' : 'taken' })
        })
        .catch(() => {
          if (!cancelled) setCheck({ name: trimmedUsername, status: 'error' })
        })
    }, 400)

    return () => {
      cancelled = true
      clearTimeout(handle)
    }
  }, [trimmedUsername, usernameChanged])

  // Derive the displayed status synchronously from the inputs + async result.
  let usernameStatus: UsernameStatus
  if (!usernameChanged) usernameStatus = 'unchanged'
  else if (trimmedUsername.length < 3) usernameStatus = 'invalid'
  else if (check && check.name === trimmedUsername) usernameStatus = check.status
  else usernameStatus = 'checking' // within the debounce window

  const usernameBlocks =
    usernameChanged &&
    (usernameStatus === 'checking' ||
      usernameStatus === 'taken' ||
      usernameStatus === 'invalid')

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setServerError(null)
    setSuccess(false)

    if (!trimmedName) {
      setServerError('Name is required.')
      return
    }
    if (nothingChanged) return

    try {
      await mutation.mutateAsync({
        ...(nameChanged ? { name: trimmedName } : {}),
        ...(usernameChanged ? { username: trimmedUsername } : {}),
      })
      setSuccess(true)
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Something went wrong.')
    }
  }

  return (
    <form
      onSubmit={(e) => void handleSubmit(e)}
      noValidate
      className="space-y-4 rounded-xl border border-border bg-card px-5 py-5"
    >
      <h2 className="text-sm font-semibold">Account details</h2>

      <div className="space-y-1.5">
        <Label htmlFor="profile-name">Full name</Label>
        <Input
          id="profile-name"
          value={name}
          autoComplete="name"
          onChange={(e) => setName(e.target.value)}
          disabled={mutation.isPending}
          placeholder="Jane Smith"
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="profile-username">Username</Label>
        <Input
          id="profile-username"
          value={username}
          autoComplete="username"
          onChange={(e) => setUsername(e.target.value)}
          disabled={mutation.isPending}
          aria-invalid={usernameStatus === 'taken' || usernameStatus === 'invalid'}
          placeholder="your_username"
        />
        <UsernameHint status={usernameStatus} />
      </div>

      {serverError && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {serverError}
        </p>
      )}
      {success && (
        <p role="status" className="text-sm text-green-600 dark:text-green-400">
          Profile updated.
        </p>
      )}

      <Button type="submit" disabled={mutation.isPending || nothingChanged || usernameBlocks}>
        {mutation.isPending ? 'Saving…' : 'Save changes'}
      </Button>
    </form>
  )
}

function UsernameHint({ status }: { status: UsernameStatus }) {
  switch (status) {
    case 'invalid':
      return <p className="text-xs text-destructive">Username must be at least 3 characters.</p>
    case 'checking':
      return <p className="text-xs text-muted-foreground">Checking availability…</p>
    case 'available':
      return <p className="text-xs text-green-600 dark:text-green-400">Username is available.</p>
    case 'taken':
      return <p className="text-xs text-destructive">Username is already taken.</p>
    case 'error':
      return <p className="text-xs text-muted-foreground">Could not check availability.</p>
    default:
      return null
  }
}

// ── Password ────────────────────────────────────────────────────────────────

function PasswordForm() {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldError, setFieldError] = useState<string | null>(null)
  const [serverError, setServerError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const mutation = useUpdateProfile()

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFieldError(null)
    setServerError(null)
    setSuccess(false)

    if (!current) {
      setFieldError('Enter your current password.')
      return
    }
    if (next.length < 8) {
      setFieldError('New password must be at least 8 characters.')
      return
    }
    if (next !== confirm) {
      setFieldError('New password and confirmation do not match.')
      return
    }

    try {
      await mutation.mutateAsync({ current_password: current, new_password: next })
      setSuccess(true)
      setCurrent('')
      setNext('')
      setConfirm('')
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Something went wrong.')
    }
  }

  return (
    <form
      onSubmit={(e) => void handleSubmit(e)}
      noValidate
      className="space-y-4 rounded-xl border border-border bg-card px-5 py-5"
    >
      <h2 className="text-sm font-semibold">Change password</h2>

      <div className="space-y-1.5">
        <Label htmlFor="current-password">Current password</Label>
        <Input
          id="current-password"
          type="password"
          autoComplete="current-password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          disabled={mutation.isPending}
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="new-password">New password</Label>
        <Input
          id="new-password"
          type="password"
          autoComplete="new-password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          disabled={mutation.isPending}
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="confirm-password">Confirm new password</Label>
        <Input
          id="confirm-password"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          disabled={mutation.isPending}
        />
      </div>

      {fieldError && (
        <p role="alert" className="text-xs text-destructive">
          {fieldError}
        </p>
      )}
      {serverError && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {serverError}
        </p>
      )}
      {success && (
        <p role="status" className="text-sm text-green-600 dark:text-green-400">
          Password changed.
        </p>
      )}

      <Button type="submit" disabled={mutation.isPending}>
        {mutation.isPending ? 'Saving…' : 'Update password'}
      </Button>
    </form>
  )
}
