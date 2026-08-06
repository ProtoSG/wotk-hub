import { Navigate } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { defaultRouteFor } from './defaultRoute'

// Redirects '/' and any unmatched path to the signed-in user's default
// landing route (Dashboard for admin, Citas for guest — see
// defaultRouteFor) instead of a hardcoded '/dashboard', which is itself
// role-gated now and would just bounce a guest straight back out.
export default function DefaultRedirect() {
  const role = useAuthStore((s) => s.user?.role)
  return <Navigate to={defaultRouteFor(role)} replace />
}
