import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore, type Role } from '@/store/authStore'

interface Props {
  roles: Role[]
  children: ReactNode
}

export default function RequireRole({ roles, children }: Props) {
  const user = useAuthStore((s) => s.user)
  if (!user || !roles.includes(user.role)) {
    return <Navigate to="/dashboard" replace />
  }
  return <>{children}</>
}
