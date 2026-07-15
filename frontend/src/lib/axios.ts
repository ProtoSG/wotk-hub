import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/store/authStore'

/** Normalized error thrown by the API client so callers can rely on a plain `message`. */
export class ApiError extends Error {
  status?: number
  code?: string

  constructor(message: string, status?: number, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/** Maps backend error codes to user-facing messages in Spanish. */
const ERROR_MESSAGES: Record<string, string> = {
  AUTH_INVALID_CREDENTIALS: 'Email o contraseña incorrectos',
  AUTH_UNAUTHORIZED: 'No autorizado',
  AUTH_TOKEN_EXPIRED: 'Sesión expirada. Iniciá sesión de nuevo.',
  AUTH_TOKEN_INVALID: 'Sesión inválida. Iniciá sesión de nuevo.',
  AUTH_FORBIDDEN: 'No tenés permiso para hacer esto',
  VALIDATION_ERROR: 'Datos inválidos',
  BAD_REQUEST: 'Solicitud inválida',
  NOT_FOUND: 'No encontrado',
  CONFLICT: 'Conflicto con datos existentes',
  INTERNAL_ERROR: 'Error interno del servidor',
  SERVICE_UNAVAILABLE: 'Servicio temporalmente no disponible',
}

function mapMessage(code: string | undefined, fallback: string): string {
  if (!code) return fallback
  return ERROR_MESSAGES[code] ?? fallback
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:3001',
  timeout: 30000,
  withCredentials: true,
})

interface RetriableConfig extends InternalAxiosRequestConfig {
  _retried?: boolean
}

/** Normalizes any thrown error (axios or otherwise) into an ApiError. */
function toApiError(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<{ code?: string; message?: string; error?: string }>
    const data = axiosError.response?.data
    // New format: { code, message } — fall back to { error } for backwards compat
    const code = data?.code
    const rawMessage = data?.message ?? data?.error ?? axiosError.message
    const message = mapMessage(code, rawMessage)
    return new ApiError(message, axiosError.response?.status, code)
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
      const axiosError = error as AxiosError<{ code?: string; message?: string; error?: string }>
      const config = axiosError.config as RetriableConfig | undefined
      const isAuthEndpoint =
        config?.url === '/api/auth/refresh' || config?.url === '/api/auth/login' || config?.url === '/api/auth/register'

      if (axiosError.response?.status === 401 && config && !config._retried && !isAuthEndpoint) {
        config._retried = true

        try {
          await refreshSession()
        } catch (refreshErr) {
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
