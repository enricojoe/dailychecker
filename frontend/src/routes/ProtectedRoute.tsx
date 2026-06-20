/**
 * Guard for authenticated routes.
 * - While /me is resolving: shows a full-page spinner (avoids flashing /login).
 * - Unauthenticated: redirects to /login.
 * - Authenticated: renders the child route via <Outlet />.
 */

import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext'
import { LoadingScreen } from '@/components/LoadingScreen'

export function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) return <LoadingScreen />
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <Outlet />
}
