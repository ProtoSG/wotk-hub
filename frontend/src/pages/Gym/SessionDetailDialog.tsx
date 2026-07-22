import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useGymApi } from '@/hooks/useGymApi'
import { formatKg, formatVolume } from '@/lib/weight'
import { formatDistance, formatDuration } from '@/lib/duration'
import type { TrackingType } from '@/types/gym.types'
import { sessionsKey } from './gymKeys'

interface SessionDetailDialogProps {
  sessionId: number | null
  onClose: () => void
}

export default function SessionDetailDialog({ sessionId, onClose }: SessionDetailDialogProps) {
  const queryClient = useQueryClient()
  const { getSession, deleteSession } = useGymApi()

  const { data: session, isPending } = useQuery({
    queryKey: ['gym', 'sessions', sessionId] as const,
    queryFn: () => getSession(sessionId!),
    enabled: sessionId !== null,
  })

  const remove = useMutation({
    mutationFn: () => deleteSession(sessionId!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: sessionsKey() })
      toast.success('Entrenamiento eliminado')
      onClose()
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el entrenamiento')
    },
  })

  return (
    <Dialog open={sessionId !== null} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-4 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {session ? session.name || 'Entrenamiento libre' : 'Entrenamiento'}
          </DialogTitle>
        </DialogHeader>

        <div className="-mx-1 min-h-0 flex-1 space-y-4 overflow-y-auto px-1">
          {isPending || !session ? (
            <>
              <Skeleton className="h-24 w-full rounded-lg" />
              <Skeleton className="h-24 w-full rounded-lg" />
            </>
          ) : session.exercises.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              Este entrenamiento no registró ejercicios.
            </p>
          ) : (
            session.exercises.map((sessionExercise) => {
              const trackingType = sessionExercise.exercise.trackingType
              const working = sessionExercise.sets.filter((set) => !set.isWarmup)
              const done = working.filter((set) => set.completed)
              const total =
                trackingType === 'weight_reps'
                  ? formatVolume(done.reduce((sum, set) => sum + set.reps * set.weightGrams, 0))
                  : formatDuration(done.reduce((sum, set) => sum + set.durationSeconds, 0))

              return (
                <section key={sessionExercise.id} className="rounded-lg border bg-card px-4 py-3">
                  <div className="flex items-baseline justify-between gap-3">
                    <h3 className="truncate font-medium">{sessionExercise.exercise.name}</h3>
                    {done.length > 0 && (
                      <span className="shrink-0 text-sm text-muted-foreground tabular-nums">
                        {total}
                      </span>
                    )}
                  </div>

                  <ul className="mt-2 space-y-1 text-sm tabular-nums">
                    {sessionExercise.sets.map((set, index) => (
                      <li
                        key={set.id}
                        className="flex items-center gap-3 text-muted-foreground"
                      >
                        <span className="w-6 shrink-0">
                          {set.isWarmup ? 'W' : countWorking(sessionExercise.sets, index)}
                        </span>
                        <span className="text-foreground">{describeSet(set, trackingType)}</span>
                        {!set.completed && <span className="text-xs">no completada</span>}
                      </li>
                    ))}
                  </ul>
                </section>
              )
            })
          )}
        </div>

        <div className="flex justify-end border-t pt-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => remove.mutate()}
            disabled={remove.isPending}
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="mr-1 h-4 w-4" />
            Eliminar
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

/** Renders a logged set the way its tracking type is measured. */
function describeSet(
  set: { reps: number; weightGrams: number; durationSeconds: number; distanceMeters: number },
  trackingType: TrackingType,
): string {
  if (trackingType === 'weight_reps') return `${formatKg(set.weightGrams)} kg × ${set.reps}`
  if (trackingType === 'duration') return formatDuration(set.durationSeconds)
  return [formatDuration(set.durationSeconds), formatDistance(set.distanceMeters)]
    .filter((part) => part !== '0s' && part !== '0 m')
    .join(' · ')
}

/** Working-set number at `index`, ignoring warmups above it. */
function countWorking(sets: { isWarmup: boolean }[], index: number): number {
  return sets.slice(0, index + 1).filter((set) => !set.isWarmup).length
}
