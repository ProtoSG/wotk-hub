import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/store/authStore'

/** Normalized error thrown by the API client so callers can rely on a plain `message`. */
export class ApiError extends Error {
  status?: number

  constructor(message: string, status?: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:3001',
  timeout: 30000,
  // Auth now travels as httpOnly cookies (access_token/refresh_token), not
  // a static header — the browser needs permission to send them.
  withCredentials: true,
})

interface RetriableConfig extends InternalAxiosRequestConfig {
  _retried?: boolean
}

/** Normalizes any thrown error (axios or otherwise) into an ApiError. */
function toApiError(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<{ error?: string }>
    const message = axiosError.response?.data?.error ?? axiosError.message
    return new ApiError(message, axiosError.response?.status)
  }
  const message = error instanceof Error ? error.message : 'Unexpected error'
  return new ApiError(message)
}

// Coalesces concurrent 401s into a single /api/auth/refresh call instead of
// firing one refresh per failed request.
let refreshPromise: Promise<void> | null = null

function refreshSession(): Promise<void> {
  if (!refreshPromise) {
    refreshPromise = api
      .post('/api/auth/refresh')
      .then(() => undefined)
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

api.interceptors.response.use(
  (response) => response,
  async (error: unknown) => {
    if (axios.isAxiosError(error)) {
      const axiosError = error as AxiosError<{ error?: string }>
      const config = axiosError.config as RetriableConfig | undefined
      const isAuthEndpoint =
        config?.url === '/api/auth/refresh' || config?.url === '/api/auth/login' || config?.url === '/api/auth/register'

      if (axiosError.response?.status === 401 && config && !config._retried && !isAuthEndpoint) {
        config._retried = true

        try {
          await refreshSession()
        } catch (refreshErr) {
          // Only force logout when the refresh call itself came back a
          // genuine 401 (the refresh token is invalid/expired). A network
          // error, 5xx, or timeout means the refresh token might still be
          // valid server-side, so don't clear the session over a
          // transient blip — just reject this request.
          if (axios.isAxiosError(refreshErr) && refreshErr.response?.status === 401) {
            useAuthStore.getState().setUser(null)
            if (typeof window !== 'undefined') {
              window.location.href = '/login'
            }
          }
          return Promise.reject(toApiError(refreshErr))
        }

        try {
          return await api(config)
        } catch (retryErr) {
          // Refresh succeeded but the retried request still failed (e.g.
          // the session was revoked elsewhere) — clear the stale
          // "authenticated" state instead of surfacing a generic error.
          useAuthStore.getState().setUser(null)
          if (typeof window !== 'undefined') {
            window.location.href = '/login'
          }
          return Promise.reject(toApiError(retryErr))
        }
      }

      return Promise.reject(toApiError(axiosError))
    }
    return Promise.reject(toApiError(error))
  }
)

export default api
