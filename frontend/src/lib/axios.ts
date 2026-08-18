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

// Two different jobs, easy to conflate (this file used to): translating a
// KNOWN-VARIED backend message vs. supplying one when there isn't any.
//
// FIXED_MESSAGES always wins over whatever the backend wrote — for auth
// states specifically, where the UX copy needs to read the same everywhere
// it shows up, not because the backend's own text is bad.
//
// GENERIC_FALLBACKS only fills in when the backend genuinely sent nothing
// usable (a network failure, a 500 with no body) — it must NEVER override a
// real backend message. BAD_REQUEST/CONFLICT/NOT_FOUND/VALIDATION_ERROR in
// particular cover dozens of different validation rules across the API,
// each with its own specific reason ("saldo insuficiente en tarjeta", "ya
// existe un ejercicio con ese nombre", "no podés archivar tu última tarjeta
// activa") — collapsing all of them to one generic string was the actual
// bug: a card-balance rejection looked identical to every other 400 and
// told the user nothing about why.
const FIXED_MESSAGES: Record<string, string> = {
  AUTH_INVALID_CREDENTIALS: 'Email o contraseña incorrectos',
  AUTH_UNAUTHORIZED: 'No autorizado',
  AUTH_TOKEN_EXPIRED: 'Sesión expirada. Iniciá sesión de nuevo.',
  AUTH_TOKEN_INVALID: 'Sesión inválida. Iniciá sesión de nuevo.',
  AUTH_FORBIDDEN: 'No tenés permiso para hacer esto',
}

const GENERIC_FALLBACKS: Record<string, string> = {
  VALIDATION_ERROR: 'Datos inválidos',
  BAD_REQUEST: 'Solicitud inválida',
  NOT_FOUND: 'No encontrado',
  CONFLICT: 'Conflicto con datos existentes',
  INTERNAL_ERROR: 'Error interno del servidor',
  SERVICE_UNAVAILABLE: 'Servicio temporalmente no disponible',
}

/**
 * backendMessage is whatever the response body actually said (undefined if
 * the backend sent no body at all, e.g. a network-level failure never
 * reached a handler). axiosGeneric is axios's own technical fallback
 * ("Request failed with status code 400") — never shown to a user directly.
 */
function resolveMessage(code: string | undefined, backendMessage: string | undefined, axiosGeneric: string): string {
  if (code && FIXED_MESSAGES[code]) return FIXED_MESSAGES[code]
  if (backendMessage) return backendMessage
  if (code && GENERIC_FALLBACKS[code]) return GENERIC_FALLBACKS[code]
  return axiosGeneric
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
    const backendMessage = data?.message ?? data?.error
    const message = resolveMessage(code, backendMessage, axiosError.message)
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
