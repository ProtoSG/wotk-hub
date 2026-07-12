import { lazy } from 'react'

export const DashboardPage = lazy(() => import('@/pages/Dashboard/DashboardPage'))
export const DbManagerPage = lazy(() => import('@/pages/DbManager/DbManagerPage'))
export const FinancesPage = lazy(() => import('@/pages/Finances/FinancesPage'))
export const CouplePage = lazy(() => import('@/pages/Couple/CouplePage'))
export const ConfigurationPage = lazy(() => import('@/pages/Configuration/ConfigurationPage'))
export const LoginPage = lazy(() => import('@/pages/Login/LoginPage'))
