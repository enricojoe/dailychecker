/**
 * Application router.
 *
 * Route table:
 *   /login        — public; authenticated users redirect to /
 *   /register     — public; authenticated users redirect to /
 *   /             — protected; Today page
 *   /activities   — protected; Activity CRUD
 *   /history      — protected; Calendar + By-Activity history
 *   /telegram     — protected; Telegram connection
 *   *             — catch-all; redirects to /
 */

import { createBrowserRouter, Navigate } from 'react-router-dom'
import { RootLayout } from './RootLayout'
import { ProtectedRoute } from './ProtectedRoute'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { AppShell } from '@/components/AppShell'
import { TodayPage } from '@/features/today/TodayPage'
import { ActivitiesPage } from '@/features/activities/ActivitiesPage'
import { HistoryPage } from '@/features/history/HistoryPage'
import { TelegramPage } from '@/features/telegram/TelegramPage'

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
            children: [
              { path: '/', element: <TodayPage /> },
              { path: '/activities', element: <ActivitiesPage /> },
              { path: '/history', element: <HistoryPage /> },
              { path: '/telegram', element: <TelegramPage /> },
            ],
          },
        ],
      },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])
