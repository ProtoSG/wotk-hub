import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type Role = 'admin' | 'guest'

export interface AuthUser {
  id: number
  name: string
  email: string
  role: Role
}

interface AuthStore {
  user: AuthUser | null
  // Admin-granted module access (see usePermissionsApi) — [] until fetched.
  // Only meaningful for a guest; admin isn't gated by this at all (see
  // AuthGuard/Sidebar/RequireModule, which all special-case role === 'admin'
  // rather than relying on this list being populated for them).
  modules: string[]
  // Tracks whether AuthGuard has already resolved the initial /api/auth/me
  // check for this app load, so it only runs once (not on every navigation).
  hasHydrated: boolean
  setUser: (user: AuthUser | null) => void
  setModules: (modules: string[]) => void
  setHasHydrated: (hasHydrated: boolean) => void
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      user: null,
      modules: [],
      hasHydrated: false,
      setUser: (user) => set({ user }),
      setModules: (modules) => set({ modules }),
      setHasHydrated: (hasHydrated) => set({ hasHydrated }),
    }),
    {
      name: 'work-hub-auth',
      // Tokens live in httpOnly cookies, invisible to JS by design — never
      // persist them here. Only the display-only user profile is cached.
      partialize: (s) => ({ user: s.user }),
    }
  )
)
