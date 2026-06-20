/**
 * Auth state provider.
 *
 * Exposes { user, isAuthenticated, isLoading, login, register, logout }.
 * User data is hydrated via TanStack Query GET /me (enabled only when a
 * token exists), so it is always in sync with the server.
 *
 * Must be rendered inside both QueryClientProvider and a Router so that
 * useQueryClient() and useNavigate() are available.
 */

import { createContext, useContext, useState } from 'react'
import type { ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { authApi } from './api'
import type { UserDto } from './api'
import { tokenStore } from './tokenStore'

// ── Context shape ─────────────────────────────────────────────────────────────

interface AuthContextValue {
  /** Null while loading or unauthenticated. */
  user: UserDto | null
  isAuthenticated: boolean
  /** True only while a token exists but the /me fetch hasn't resolved yet. */
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  register: (name: string, username: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

// ── Provider ──────────────────────────────────────────────────────────────────

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  // Initialise from localStorage so the session survives a hard refresh.
  const [hasToken, setHasToken] = useState(() => !!tokenStore.getAccess())

  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: () => authApi.me(),
    enabled: hasToken,
    // Don't throw to an error boundary — auth errors are handled here.
    throwOnError: false,
    // Let the apiClient refresh flow handle 401; don't let TQ retry on top.
    retry: false,
  })

  // ── Auth actions ───────────────────────────────────────────────────────────

  const login = async (username: string, password: string): Promise<void> => {
    const { access, refresh } = await authApi.login(username, password)
    tokenStore.set(access, refresh)
    setHasToken(true)
    // Mark /me stale so it refetches after enabled flips to true.
    void queryClient.invalidateQueries({ queryKey: ['me'] })
  }

  const register = async (
    name: string,
    username: string,
    password: string
  ): Promise<void> => {
    await authApi.register({ name, username, password })
    // Auto-login: registration doesn't return tokens, so follow up with login.
    const { access, refresh } = await authApi.login(username, password)
    tokenStore.set(access, refresh)
    setHasToken(true)
    void queryClient.invalidateQueries({ queryKey: ['me'] })
  }

  const logout = async (): Promise<void> => {
    const refresh = tokenStore.getRefresh()
    if (refresh) {
      try {
        await authApi.logout(refresh)
      } catch {
        // Server-side revocation failed — still clear locally.
      }
    }
    tokenStore.clear()
    setHasToken(false)
    queryClient.clear()
    navigate('/login')
  }

  // ── Context value ──────────────────────────────────────────────────────────

  const value: AuthContextValue = {
    user: meQuery.data ?? null,
    isAuthenticated: !!meQuery.data,
    // Show loading spinner only when we have a token but no user data yet.
    isLoading: hasToken && meQuery.isPending,
    login,
    register,
    logout,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// ── Hook ──────────────────────────────────────────────────────────────────────

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within <AuthProvider>')
  }
  return ctx
}
