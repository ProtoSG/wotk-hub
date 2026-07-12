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
  // Tracks whether AuthGuard has already resolved the initial /api/auth/me
  // check for this app load, so it only runs once (not on every navigation).
  hasHydrated: boolean
  setUser: (user: AuthUser | null) => void
  setHasHydrated: (hasHydrated: boolean) => void
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      user: null,
      hasHydrated: false,
      setUser: (user) => set({ user }),
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
