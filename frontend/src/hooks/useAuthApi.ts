import api from '@/lib/axios'
import type { AuthUser } from '@/store/authStore'

export function useAuthApi() {
  async function login(email: string, password: string): Promise<AuthUser> {
    const res = await api.post<AuthUser>('/api/auth/login', { email, password })
    return res.data
  }

  async function logout(): Promise<void> {
    await api.post('/api/auth/logout')
  }

  async function logoutAll(): Promise<void> {
    await api.post('/api/auth/logout-all')
  }

  async function me(): Promise<AuthUser> {
    const res = await api.get<AuthUser>('/api/auth/me')
    return res.data
  }

  return { login, logout, logoutAll, me }
}
