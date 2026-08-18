import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore, type Role } from '@/store/authStore'
import { defaultRouteFor } from './defaultRoute'

interface Props {
  roles: Role[]
  // Admin-grantable module key (see usePermissionsApi/AllModules on the
  // backend) — when set, a non-admin role in `roles` also needs this module
  // enabled in their store's `modules` list. Admin bypasses this check
  // entirely (never gated by module grants, matches the backend's
  // permissions.RequireModule). Omit for routes with no module concept
  // (still role-gated as before) — db-manager/configuration always omit it,
  // since neither is ever grantable (see AllModules' comment).
  module?: string
  children: ReactNode
}

export default function RequireRole({ roles, module, children }: Props) {
  const user = useAuthStore((s) => s.user)
  const modules = useAuthStore((s) => s.modules)
  const allowed =
    !!user &&
    roles.includes(user.role) &&
    (user.role === 'admin' || !module || modules.includes(module))
  if (!allowed) {
    // Not a hardcoded '/dashboard' — Dashboard is itself role-gated now, so
    // bouncing a blocked guest there would just bounce them right back out.
    return <Navigate to={defaultRouteFor(user?.role)} replace />
  }
  return <>{children}</>
}
