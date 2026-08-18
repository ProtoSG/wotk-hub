import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import RouteFallback from './RouteFallback'
import { useAuthApi } from '@/hooks/useAuthApi'
import { usePermissionsApi } from '@/hooks/usePermissionsApi'
import { useAuthStore } from '@/store/authStore'

interface Props {
  children: ReactNode
}

/**
 * Resolves the session once per app load (gated by the store's hasHydrated
 * flag, not per navigation) by calling /api/auth/me, then either renders
 * children or redirects to /login.
 */
export default function AuthGuard({ children }: Props) {
  const { me } = useAuthApi()
  const { listMine } = usePermissionsApi()
  const user = useAuthStore((s) => s.user)
  const hasHydrated = useAuthStore((s) => s.hasHydrated)
  const setUser = useAuthStore((s) => s.setUser)
  const setModules = useAuthStore((s) => s.setModules)
  const setHasHydrated = useAuthStore((s) => s.setHasHydrated)
  const [checking, setChecking] = useState(!hasHydrated)

  const checkSession = useCallback(async () => {
    try {
      const u = await me()
      setUser(u)
      // Admin isn't gated by this list at all (every consumer special-cases
      // the role), so skip the extra round trip for them.
      setModules(u.role === 'admin' ? [] : await listMine())
    } catch {
      setUser(null)
      setModules([])
    } finally {
      setHasHydrated(true)
      setChecking(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (hasHydrated) return
    checkSession()
  }, [hasHydrated, checkSession])

  if (checking) return <RouteFallback />
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}
