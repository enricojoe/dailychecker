/**
 * Typed API client.
 *
 * Extended in M7:
 * - Attaches `Authorization: Bearer <access>` on every request.
 * - On 401, attempts a single-flight token refresh via POST /auth/refresh,
 *   persists the rotated pair, and retries the original request once.
 * - On refresh failure (or no refresh token) clears tokens and redirects to
 *   /login via window.location.replace (avoids a React Router dependency here).
 * - Parses the backend's `{ error: "..." }` envelope and surfaces it as
 *   ApiError.message so form components can display it directly.
 *
 * The existing `apiClient.{get,post,patch,delete}` surface is preserved —
 * all future feature hooks keep working without changes.
 *
 * XSS trade-off: tokens live in localStorage (required for hard-refresh
 * persistence). Revisit in M9 (httpOnly cookies / token binding).
 */

import { tokenStore } from '@/auth/tokenStore'

// Fall back to the documented dev default so a missing frontend/.env never
// produces request URLs like "undefined/auth/register".
const DEFAULT_BASE_URL = 'http://localhost:8080/api'
const BASE_URL =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? DEFAULT_BASE_URL

if (!import.meta.env.VITE_API_BASE_URL) {
  console.warn(
    `[apiClient] VITE_API_BASE_URL is not set — falling back to ${DEFAULT_BASE_URL}. ` +
      'Copy frontend/.env.example to frontend/.env to configure it.'
  )
}

// ── Error type ────────────────────────────────────────────────────────────────

/**
 * Structured error thrown for any non-2xx response.
 * `message` is the backend's `{ error }` string when available, or a fallback.
 * `status` is the HTTP status code.
 */
export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// ── Single-flight refresh ─────────────────────────────────────────────────────

/**
 * Paths that must NOT trigger the 401→refresh flow.
 * The refresh endpoint itself, and auth endpoints that legitimately return 401.
 */
const SKIP_REFRESH = ['/auth/refresh', '/auth/login', '/auth/register']

/**
 * One in-flight refresh promise shared by all concurrent 401s.
 * Refresh tokens ROTATE on each use — two parallel refreshes would invalidate
 * each other, so we funnel them through a single promise.
 */
let refreshPromise: Promise<void> | null = null

async function doRefresh(): Promise<void> {
  const refresh = tokenStore.getRefresh()
  if (!refresh) {
    tokenStore.clear()
    window.location.replace('/login')
    throw new ApiError(401, 'No refresh token')
  }

  const res = await fetch(`${BASE_URL}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh }),
  })

  if (!res.ok) {
    tokenStore.clear()
    window.location.replace('/login')
    throw new ApiError(401, 'Session expired')
  }

  const data = (await res.json()) as { access: string; refresh: string }
  tokenStore.set(data.access, data.refresh)
}

// ── Core request wrapper ──────────────────────────────────────────────────────

async function request<T>(
  path: string,
  init?: RequestInit,
  isRetry = false
): Promise<T> {
  const access = tokenStore.getAccess()

  const callerHeaders = (init?.headers ?? {}) as Record<string, string>
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...callerHeaders,
  }
  if (access) {
    headers['Authorization'] = `Bearer ${access}`
  }

  const url = `${BASE_URL}${path}`
  const response = await fetch(url, { ...init, headers })

  // ── 401 → single-flight refresh + retry ──────────────────────────────────
  if (
    response.status === 401 &&
    !isRetry &&
    !SKIP_REFRESH.some((p) => path.startsWith(p))
  ) {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => {
        refreshPromise = null
      })
    }
    try {
      await refreshPromise
    } catch {
      // doRefresh already redirected; rethrow so callers can handle
      throw new ApiError(401, 'Session expired')
    }
    return request<T>(path, init, true)
  }

  // ── Non-2xx ──────────────────────────────────────────────────────────────
  if (!response.ok) {
    let message = `${response.status}: ${response.statusText}`
    try {
      const body = (await response.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // body is not JSON — use the default message above
    }
    throw new ApiError(response.status, message)
  }

  // ── 204 No Content ────────────────────────────────────────────────────────
  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}

// ── Convenience helpers (same surface as M0) ──────────────────────────────────

export const apiClient = {
  get: <T>(path: string, init?: RequestInit) =>
    request<T>(path, { method: 'GET', ...init }),

  post: <T>(path: string, body: unknown, init?: RequestInit) =>
    request<T>(path, {
      method: 'POST',
      body: JSON.stringify(body),
      ...init,
    }),

  patch: <T>(path: string, body: unknown, init?: RequestInit) =>
    request<T>(path, {
      method: 'PATCH',
      body: JSON.stringify(body),
      ...init,
    }),

  delete: <T>(path: string, init?: RequestInit) =>
    request<T>(path, { method: 'DELETE', ...init }),
}
