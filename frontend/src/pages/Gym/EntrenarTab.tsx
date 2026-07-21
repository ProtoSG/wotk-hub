import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Dumbbell, Flag, Loader2, Plus, Timer, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGymApi } from '@/hooks/useGymApi'
import { todayISO } from '@/lib/date'
import { formatVolume } from '@/lib/weight'
import type { Exercise, Session } from '@/types/gym.types'
import { activeSessionKey, sessionsKey } from './gymKeys'
import { formatDuration, useElapsedMinutes } from './useElapsed'
import ExercisePickerDialog from './ExercisePickerDialog'
import SessionExerciseCard from './SessionExerciseCard'

interface EntrenarTabProps {
  /** Set by the FAB and the desktop button; cleared once handled. */
  pickerOpen: boolean
  onPickerOpenChange: (open: boolean) => void
}

export default function EntrenarTab({ pickerOpen, onPickerOpenChange }: EntrenarTabProps) {
  const queryClient = useQueryClient()
  const {
    activeSession,
    createSession,
    finishSession,
    deleteSession,
    addSessionExercise,
    removeSessionExercise,
  } = useGymApi()

  const { data: session, isPending } = useQuery({
    queryKey: activeSessionKey(),
    queryFn: () => activeSession(),
  })

  const applySession = (next: Session) => {
    queryClient.setQueryData(activeSessionKey(), next)
  }

  const start = useMutation({
    mutationFn: () => createSession({ name: '', occurredOn: todayISO(), notes: '' }),
    onSuccess: (next) => {
      applySession(next)
      onPickerOpenChange(true)
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo empezar el entrenamiento')
    },
  })

  const addExercise = useMutation({
    mutationFn: (exercise: Exercise) => addSessionExercise(session!.id, exercise.id),
    onSuccess: applySession,
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo agregar el ejercicio')
    },
  })

  const removeExercise = useMutation({
    mutationFn: (sessionExerciseId: number) => removeSessionExercise(session!.id, sessionExerciseId),
    onSuccess: applySession,
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo quitar el ejercicio')
    },
  })

  const finish = useMutation({
    mutationFn: () => finishSession(session!.id),
    onSuccess: () => {
      queryClient.setQueryData(activeSessionKey(), null)
      queryClient.invalidateQueries({ queryKey: sessionsKey() })
      toast.success('Entrenamiento terminado')
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo terminar el entrenamiento')
    },
  })

  const discard = useMutation({
    mutationFn: () => deleteSession(session!.id),
    onSuccess: () => {
      queryClient.setQueryData(activeSessionKey(), null)
      queryClient.invalidateQueries({ queryKey: sessionsKey() })
      toast.success('Entrenamiento descartado')
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo descartar el entrenamiento')
    },
  })

  if (isPending) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-20 w-full rounded-lg" />
        <Skeleton className="h-40 w-full rounded-lg" />
      </div>
    )
  }

  if (!session) {
    return (
      <StartPrompt onStart={() => start.mutate()} starting={start.isPending} />
    )
  }

  return (
    <div className="space-y-4">
      <SessionHeader
        session={session}
        onFinish={() => finish.mutate()}
        finishing={finish.isPending}
        onDiscard={() => discard.mutate()}
      />

      {session.exercises.length === 0 ? (
        <div className="rounded-lg border border-dashed px-6 py-10 text-center">
          <Dumbbell className="mx-auto mb-3 h-7 w-7 text-muted-foreground" />
          <p className="font-medium">Elegí el primer ejercicio</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Buscá en el catálogo y las series se registran acá.
          </p>
          <Button className="mt-4" onClick={() => onPickerOpenChange(true)}>
            <Plus className="mr-1 h-4 w-4" />
            Agregar ejercicio
          </Button>
        </div>
      ) : (
        <>
          <div className="space-y-3">
            {session.exercises.map((sessionExercise) => (
              <SessionExerciseCard
                key={sessionExercise.id}
                sessionId={session.id}
                sessionExercise={sessionExercise}
                onRemove={() => removeExercise.mutate(sessionExercise.id)}
              />
            ))}
          </div>

          <Button
            variant="outline"
            className="hidden w-full sm:flex"
            onClick={() => onPickerOpenChange(true)}
          >
            <Plus className="mr-1 h-4 w-4" />
            Agregar ejercicio
          </Button>
        </>
      )}

      <ExercisePickerDialog
        open={pickerOpen}
        onClose={() => onPickerOpenChange(false)}
        onSelect={(exercise) => addExercise.mutate(exercise)}
      />
    </div>
  )
}

function StartPrompt({ onStart, starting }: { onStart: () => void; starting: boolean }) {
  return (
    <div className="rounded-lg border bg-card px-6 py-12 text-center">
      <Dumbbell className="mx-auto mb-4 h-8 w-8 text-primary" />
      <h2 className="text-lg font-semibold">No hay entrenamiento en curso</h2>
      <p className="mx-auto mt-1 max-w-sm text-sm text-muted-foreground">
        Empezá uno y andá agregando ejercicios sobre la marcha. Cada serie se guarda apenas la
        anotás.
      </p>
      <Button className="mt-5" onClick={onStart} disabled={starting}>
        {starting ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Plus className="mr-1 h-4 w-4" />}
        Empezar entrenamiento
      </Button>
    </div>
  )
}

interface SessionHeaderProps {
  session: Session
  onFinish: () => void
  finishing: boolean
  onDiscard: () => void
}

function SessionHeader({ session, onFinish, finishing, onDiscard }: SessionHeaderProps) {
  const minutes = useElapsedMinutes(session.startedAt)

  const completedSets = session.exercises.reduce(
    (sum, se) => sum + se.sets.filter((set) => set.completed && !set.isWarmup).length,
    0,
  )
  const volume = session.exercises.reduce(
    (sum, se) =>
      sum +
      se.sets
        .filter((set) => set.completed && !set.isWarmup)
        .reduce((setSum, set) => setSum + set.reps * set.weightGrams, 0),
    0,
  )
  const empty = session.exercises.length === 0

  return (
    <header className="rounded-lg border bg-card px-4 py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="font-semibold">{session.name || 'Entrenamiento libre'}</h2>
          <p className="mt-0.5 flex items-center gap-1.5 text-sm text-muted-foreground tabular-nums">
            <Timer className="h-3.5 w-3.5" />
            {formatDuration(minutes)}
            {completedSets > 0 && (
              <>
                <span aria-hidden>·</span>
                {completedSets} {completedSets === 1 ? 'serie' : 'series'}
                <span aria-hidden>·</span>
                {formatVolume(volume)}
              </>
            )}
          </p>
        </div>

        {/* Discarding is only offered while the session is still empty —
            after that, finishing keeps the record and deleting belongs in the
            history, behind a confirmation. */}
        {empty ? (
          <Button variant="ghost" size="sm" onClick={onDiscard} className="text-muted-foreground">
            <Trash2 className="mr-1 h-4 w-4" />
            Descartar
          </Button>
        ) : (
          <Button size="sm" onClick={onFinish} disabled={finishing}>
            {finishing ? (
              <Loader2 className="mr-1 h-4 w-4 animate-spin" />
            ) : (
              <Flag className="mr-1 h-4 w-4" />
            )}
            Terminar
          </Button>
        )}
      </div>
    </header>
  )
}
