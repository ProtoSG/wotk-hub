import { useCallback } from 'react'
import api from '@/lib/axios'

// Same useCallback-empty-deps pattern as usePetApi.ts — api is a stable
// module import, nothing here legitimately changes across renders.
export function usePetChat() {
  // Rethrows on failure rather than swallowing — api's axios interceptor
  // already normalizes any failure into an ApiError with a Spanish
  // user-facing .message (see lib/axios.ts's ERROR_MESSAGES map), same
  // pattern PetChat.tsx's caller uses to show a toast via
  // `err instanceof Error ? err.message : fallback`.
  const sendMessage = useCallback(async (text: string): Promise<string> => {
    const res = await api.post<{ response: string }>('/api/pet/chat', { message: text })
    return res.data.response
  }, [])

  // responseType: 'blob' overrides axios's default JSON parsing for just
  // this request — the backend streams raw MP3 bytes (Content-Type:
  // audio/mpeg), not a JSON body, so decoding it as JSON would corrupt it.
  const speak = useCallback(async (text: string): Promise<Blob> => {
    const res = await api.post<Blob>('/api/pet/speak', { text }, { responseType: 'blob' })
    return res.data
  }, [])

  return { sendMessage, speak }
}
