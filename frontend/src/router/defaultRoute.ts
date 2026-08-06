import type { Role } from '@/store/authStore'

// Where a role lands after login, on '/', or when bounced off a route it
// can't access (see RequireRole). Guest's module access is restricted to
// Citas + Juegos (Sidebar's roles filter + RequireRole on the other
// routes) — Dashboard is now admin-only too, so it's not a valid fallback
// for guest anymore: redirecting there would just immediately bounce back
// out via RequireRole.
export function defaultRouteFor(role: Role | undefined): string {
  return role === 'guest' ? '/citas' : '/dashboard'
}
