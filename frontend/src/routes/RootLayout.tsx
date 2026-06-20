/**
 * Root layout rendered by the router as the top-level element.
 * Mounts AuthProvider inside the router so that useNavigate() is available
 * to the provider (required for SPA-style logout navigation).
 */

import { Outlet } from 'react-router-dom'
import { AuthProvider } from '@/auth/AuthContext'

export function RootLayout() {
  return (
    <AuthProvider>
      <Outlet />
    </AuthProvider>
  )
}
