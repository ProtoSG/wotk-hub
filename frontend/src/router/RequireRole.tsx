import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore, type Role } from '@/store/authStore'
import { defaultRouteFor } from './defaultRoute'

interface Props {
  roles: Role[]
  children: ReactNode
}

export default function RequireRole({ roles, children }: Props) {
  const user = useAuthStore((s) => s.user)
  if (!user || !roles.includes(user.role)) {
    // Not a hardcoded '/dashboard' — Dashboard is itself role-gated now, so
    // bouncing a blocked guest there would just bounce them right back out.
    return <Navigate to={defaultRouteFor(user?.role)} replace />
  }
  return <>{children}</>
}
