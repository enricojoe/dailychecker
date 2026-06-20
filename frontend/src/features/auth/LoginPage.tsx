/**
 * Public login page.
 * - Client validation: phone required, password ≥ 8 chars.
 * - Surfaces backend error messages (401 "invalid credentials", etc.).
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

export function LoginPage() {
  const { login, isAuthenticated, isLoading } = useAuth()
  const navigate = useNavigate()

  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [fieldError, setFieldError] = useState<{ phone?: string; password?: string }>({})
  const [serverError, setServerError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  // While /me is still resolving for an existing session, show a spinner.
  if (isLoading) return <LoadingScreen />
  // Already authenticated — go to app.
  if (isAuthenticated) return <Navigate to="/" replace />

  function validate(): boolean {
    const errors: { phone?: string; password?: string } = {}
    if (!phone.trim()) errors.phone = 'Phone number is required.'
    if (password.length < 8) errors.password = 'Password must be at least 8 characters.'
    setFieldError(errors)
    return Object.keys(errors).length === 0
  }

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!validate()) return

    try {
      setIsSubmitting(true)
      setServerError(null)
      await login(phone, password)
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
              Sign in
            </h1>
            <p className="text-sm text-muted-foreground">
              Welcome back to DailyChecker
            </p>
          </div>

          <form onSubmit={(e) => void handleSubmit(e)} noValidate className="space-y-4">
            {/* Phone */}
            <div className="space-y-1.5">
              <Label htmlFor="phone">Phone number</Label>
              <Input
                id="phone"
                type="tel"
                autoComplete="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                aria-describedby={fieldError.phone ? 'phone-error' : undefined}
                aria-invalid={!!fieldError.phone}
                disabled={isSubmitting}
                placeholder="+1 555 000 0000"
              />
              {fieldError.phone && (
                <p id="phone-error" role="alert" className="text-xs text-destructive">
                  {fieldError.phone}
                </p>
              )}
            </div>

            {/* Password */}
            <div className="space-y-1.5">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
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
              {isSubmitting ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </div>

        <p className="text-center text-sm text-muted-foreground">
          Don&apos;t have an account?{' '}
          <Link to="/register" className="font-medium text-foreground underline-offset-4 hover:underline">
            Register
          </Link>
        </p>
      </div>
    </div>
  )
}
