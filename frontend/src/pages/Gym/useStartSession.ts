import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useGymApi } from '@/hooks/useGymApi'
import { todayISO } from '@/lib/date'
import { activeSessionKey } from './gymKeys'

/**
 * Starts a workout, freestyle or from a routine. Shared by the Entrenar tab's
 * start prompt and the routine list, so both paths write the active session
 * into the same cache entry and land on an identical state.
 */
export function useStartSession(onStarted?: () => void) {
  const queryClient = useQueryClient()
  const { createSession } = useGymApi()

  return useMutation({
    mutationFn: (routineId?: number) =>
      createSession({ routineId, name: '', occurredOn: todayISO(), notes: '' }),
    onSuccess: (session) => {
      queryClient.setQueryData(activeSessionKey(), session)
      onStarted?.()
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo empezar el entrenamiento')
    },
  })
}
