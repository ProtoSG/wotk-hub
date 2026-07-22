import { Suspense } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import AppLayout from '@/layouts/AppLayout'
import AuthGuard from './AuthGuard'
import RequireRole from './RequireRole'
import RouteFallback from './RouteFallback'
import {
  DashboardPage,
  DbManagerPage,
  FinancesPage,
  CategoriesPage,
  CouplePage,
  GymPage,
  YtdlpPage,
  PublicYtdlpPage,
  ConfigurationPage,
  LoginPage,
} from './lazyPages'

export const router = createBrowserRouter([
  {
    path: '/login',
    element: (
      <Suspense fallback={<RouteFallback />}>
        <LoginPage />
      </Suspense>
    ),
  },
  {
    // Token-gated, not JWT — intentionally outside AuthGuard/AppLayout so it
    // works for someone with no account on this app (see usePublicYtdlpApi).
    path: '/publico/ytdlp',
    element: (
      <Suspense fallback={<RouteFallback />}>
        <PublicYtdlpPage />
      </Suspense>
    ),
  },
  {
    path: '/',
    element: (
      <AuthGuard>
        <AppLayout />
      </AuthGuard>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      {
        path: 'dashboard',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <DashboardPage />
          </Suspense>
        ),
      },
      {
        path: 'db-manager',
        element: (
          <RequireRole roles={['admin']}>
            <Suspense fallback={<RouteFallback />}>
              <DbManagerPage />
            </Suspense>
          </RequireRole>
        ),
      },
      {
        path: 'finances',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <FinancesPage />
          </Suspense>
        ),
      },
      {
        path: 'finances/categories',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <CategoriesPage />
          </Suspense>
        ),
      },
      {
        path: 'citas',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <CouplePage />
          </Suspense>
        ),
      },
      {
        path: 'gym',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <GymPage />
          </Suspense>
        ),
      },
      {
        path: 'ytdlp',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <YtdlpPage />
          </Suspense>
        ),
      },
      {
        path: 'configuration',
        element: (
          <Suspense fallback={<RouteFallback />}>
            <ConfigurationPage />
          </Suspense>
        ),
      },
      { path: '*', element: <Navigate to="/dashboard" replace /> },
    ],
  },
])
