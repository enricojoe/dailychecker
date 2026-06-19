/**
 * Typed API client stub.
 *
 * The base URL is read from the VITE_API_BASE_URL environment variable,
 * which must be defined in frontend/.env (gitignored) or frontend/.env.example.
 * Vite only exposes VITE_-prefixed variables to the browser bundle — never put
 * backend secrets here.
 *
 * Real request methods and typed response DTOs will be added in Milestone 7.
 */

const BASE_URL = import.meta.env.VITE_API_BASE_URL as string

if (!BASE_URL) {
  console.warn(
    '[apiClient] VITE_API_BASE_URL is not set. ' +
      'Copy frontend/.env.example to frontend/.env and fill in the value.'
  )
}

/** Shared fetch wrapper — all feature hooks will call this. */
async function request<T>(
  path: string,
  init?: RequestInit
): Promise<T> {
  const url = `${BASE_URL}${path}`
  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
    ...init,
  })

  if (!response.ok) {
    throw new Error(`API ${response.status}: ${response.statusText} — ${url}`)
  }

  return response.json() as Promise<T>
}

/** Convenience helpers — auth header injection added in Milestone 7. */
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
