/**
 * Application router.
 *
 * Route table:
 *   /login      — public; authenticated users redirect to /
 *   /register   — public; authenticated users redirect to /
 *   /           — protected; renders AppShell + TodayPlaceholder (M8 wires content)
 *   *           — catch-all; redirects to /
 */

import { createBrowserRouter, Navigate } from 'react-router-dom'
import { RootLayout } from './RootLayout'
import { ProtectedRoute } from './ProtectedRoute'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { AppShell } from '@/components/AppShell'
import { TodayPlaceholder } from '@/features/today/TodayPlaceholder'

export const router = createBrowserRouter([
  {
    element: <RootLayout />,
    children: [
      { path: '/login', element: <LoginPage /> },
      { path: '/register', element: <RegisterPage /> },
      {
        element: <ProtectedRoute />,
        children: [
          {
            element: <AppShell />,
            children: [{ path: '/', element: <TodayPlaceholder /> }],
          },
        ],
      },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])
