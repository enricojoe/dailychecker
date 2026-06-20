/**
 * Public registration page.
 * - Client validation: name required, username required (min 3 chars), password ≥ 8 chars.
 * - Surfaces backend errors: 409 "username already taken", 422 validation.
 * - On success: auto-logs in and navigates to /.
 * - Authenticated users are redirected to / immediately.
 */

import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LoadingScreen } from '@/components/LoadingScreen'

export function RegisterPage() {
  const { register, isAuthenticated, isLoading } = useAuth()
  const navigate = useNavigate()

  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [fieldError, setFieldError] = useState<{
    name?: string
    username?: string
    password?: string
  }>({})
  const [serverError, setServerError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (isLoading) return <LoadingScreen />
  if (isAuthenticated) return <Navigate to="/" replace />

  function validate(): boolean {
    const errors: { name?: string; username?: string; password?: string } = {}
    if (!name.trim()) errors.name = 'Name is required.'
    if (username.trim().length < 3) errors.username = 'Username must be at least 3 characters.'
    if (password.length < 8)
      errors.password = 'Password must be at least 8 characters.'
    setFieldError(errors)
    return Object.keys(errors).length === 0
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!validate()) return

    try {
      setIsSubmitting(true)
      setServerError(null)
      await register(name.trim(), username.trim(), password)
      navigate('/')
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Something went wrong.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm space-y-6">
        {/* Card */}
        <div className="rounded-xl border border-border bg-card px-6 py-8 shadow-sm">
          <div className="mb-6 space-y-1">
            <h1 className="text-xl font-semibold tracking-tight text-card-foreground">
              Create account
            </h1>
            <p className="text-sm text-muted-foreground">
              Start tracking your daily activities
            </p>
          </div>

          <form onSubmit={(e) => void handleSubmit(e)} noValidate className="space-y-4">
            {/* Name */}
            <div className="space-y-1.5">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                type="text"
                autoComplete="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                aria-describedby={fieldError.name ? 'name-error' : undefined}
                aria-invalid={!!fieldError.name}
                disabled={isSubmitting}
                placeholder="Jane Smith"
              />
              {fieldError.name && (
                <p id="name-error" role="alert" className="text-xs text-destructive">
                  {fieldError.name}
                </p>
              )}
            </div>

            {/* Username */}
            <div className="space-y-1.5">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                type="text"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                aria-describedby={fieldError.username ? 'username-error' : undefined}
                aria-invalid={!!fieldError.username}
                disabled={isSubmitting}
                placeholder="your_username"
              />
              {fieldError.username && (
                <p id="username-error" role="alert" className="text-xs text-destructive">
                  {fieldError.username}
                </p>
              )}
            </div>

            {/* Password */}
            <div className="space-y-1.5">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-describedby={fieldError.password ? 'password-error' : undefined}
                aria-invalid={!!fieldError.password}
                disabled={isSubmitting}
              />
              {fieldError.password && (
                <p id="password-error" role="alert" className="text-xs text-destructive">
                  {fieldError.password}
                </p>
              )}
            </div>

            {/* Server error */}
            {serverError && (
              <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {serverError}
              </p>
            )}

            <Button
              type="submit"
              className="w-full"
              disabled={isSubmitting}
            >
              {isSubmitting ? 'Creating account…' : 'Create account'}
            </Button>
          </form>
        </div>

        <p className="text-center text-sm text-muted-foreground">
          Already have an account?{' '}
          <Link to="/login" className="font-medium text-foreground underline-offset-4 hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  )
}
