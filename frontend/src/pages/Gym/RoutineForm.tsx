import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowDown, ArrowLeft, ArrowUp, Loader2, Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { useGymApi } from '@/hooks/useGymApi'
import type { Exercise, RoutineInput } from '@/types/gym.types'
import { routineKey, routinesKey } from './gymKeys'
import ExerciseCatalog from './ExerciseCatalog'
import NumericField from './NumericField'

const DEFAULT_SETS = 3
const DEFAULT_REPS = 10

interface DraftExercise {
  /** Client-only, stable across reorders — see setRows.ts for the reasoning. */
  key: string
  exerciseId: number
  name: string
  detail: string
  targetSets: number
  targetReps: number
}

interface RoutineFormProps {
  open: boolean
  /** null creates a new routine; an id loads that one for editing. */
  routineId: number | null
  onClose: () => void
}

function draftFrom(exercise: Exercise): DraftExercise {
  return {
    // Picked in an event handler, so a random key is safe — and it can't
    // collide with the `saved-<id>` keys of an already-stored routine.
    key: `draft-${crypto.randomUUID()}`,
    exerciseId: exercise.id,
    name: exercise.name,
    detail: [exercise.primaryMuscle, exercise.equipment].filter(Boolean).join(' · '),
    targetSets: DEFAULT_SETS,
    targetReps: DEFAULT_REPS,
  }
}

export default function RoutineForm({ open, routineId, onClose }: RoutineFormProps) {
  const queryClient = useQueryClient()
  const { getRoutine, createRoutine, updateRoutine } = useGymApi()

  const [name, setName] = useState('')
  const [exercises, setExercises] = useState<DraftExercise[]>([])
  // The picker is a second view of THIS dialog, not a nested one: a Radix
  // dialog inside another renders in its own portal, so interacting with it
  // counts as an outside press for the parent and dismisses it — taking the
  // unsaved draft with it.
  const [picking, setPicking] = useState(false)
  const [loadedId, setLoadedId] = useState<number | null>(null)

  const { data: routine, isPending: loadingRoutine } = useQuery({
    queryKey: routineKey(routineId ?? 0),
    queryFn: () => getRoutine(routineId!),
    enabled: open && routineId !== null,
  })

  // Seeds the draft once per opened routine. Done during render rather than in
  // an effect so the fields never paint one frame empty; `loadedId` is the
  // guard that keeps it from clobbering edits on later renders.
  const wantedId = open ? routineId : null
  if (loadedId !== wantedId && (wantedId === null || routine?.id === wantedId)) {
    setLoadedId(wantedId)
    // Always reopen on the form, never on the picker the last edit left behind.
    setPicking(false)
    setName(routine && wantedId !== null ? routine.name : '')
    setExercises(
      routine && wantedId !== null
        ? routine.exercises.map((re) => ({
            // Seeded during render, so the key must be derived from the data
            // rather than a counter: routine_exercises ids are stable and
            // unique within the routine.
            key: `saved-${re.id}`,
            exerciseId: re.exerciseId,
            name: re.exercise.name,
            detail: [re.exercise.primaryMuscle, re.exercise.equipment].filter(Boolean).join(' · '),
            targetSets: re.targetSets,
            targetReps: re.targetReps,
          }))
        : [],
    )
  }

  const save = useMutation({
    mutationFn: () => {
      const input: RoutineInput = {
        name: name.trim(),
        notes: '',
        exercises: exercises.map((e) => ({
          exerciseId: e.exerciseId,
          targetSets: e.targetSets,
          targetReps: e.targetReps,
          notes: '',
        })),
      }
      return routineId === null ? createRoutine(input) : updateRoutine(routineId, input)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: routinesKey() })
      toast.success(routineId === null ? 'Rutina creada' : 'Rutina actualizada')
      onClose()
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar la rutina')
    },
  })

  const move = (index: number, delta: number) => {
    const target = index + delta
    if (target < 0 || target >= exercises.length) return
    const next = [...exercises]
    ;[next[index], next[target]] = [next[target], next[index]]
    setExercises(next)
  }

  const update = (key: string, patch: Partial<DraftExercise>) => {
    setExercises((current) => current.map((e) => (e.key === key ? { ...e, ...patch } : e)))
  }

  const loading = routineId !== null && loadingRoutine
  const canSave = name.trim() !== '' && !save.isPending

  if (picking) {
    return (
      <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
        <DialogContent className="flex max-h-[85vh] flex-col gap-4 sm:max-w-2xl">
          <DialogHeader>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setPicking(false)}
                aria-label="Volver a la rutina"
                className="-ml-2 h-9 w-9"
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
              <DialogTitle>Agregar ejercicio</DialogTitle>
            </div>
          </DialogHeader>

          <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
            <ExerciseCatalog
              onSelect={(exercise) => {
                setExercises((current) => [...current, draftFrom(exercise)])
                setPicking(false)
              }}
            />
          </div>
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-4 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{routineId === null ? 'Nueva rutina' : 'Editar rutina'}</DialogTitle>
        </DialogHeader>

        <div className="-mx-1 min-h-0 flex-1 space-y-4 overflow-y-auto px-1">
          <div className="space-y-1">
            <Label htmlFor="routine-name">Nombre</Label>
            <Input
              id="routine-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Día de pecho"
            />
          </div>

          {loading ? (
            <>
              <Skeleton className="h-16 w-full rounded-lg" />
              <Skeleton className="h-16 w-full rounded-lg" />
            </>
          ) : exercises.length === 0 ? (
            <p className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              Agregá los ejercicios que querés hacer en este día, con sus series y repeticiones
              objetivo.
            </p>
          ) : (
            <ul className="space-y-2">
              {exercises.map((exercise, index) => (
                <li key={exercise.key} className="rounded-lg border bg-card px-3 py-2">
                  <div className="flex items-start gap-2">
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium">{exercise.name}</p>
                      <p className="truncate text-sm text-muted-foreground">{exercise.detail}</p>
                    </div>
                    <div className="flex shrink-0">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-9 w-9"
                        disabled={index === 0}
                        onClick={() => move(index, -1)}
                        aria-label={`Subir ${exercise.name}`}
                      >
                        <ArrowUp className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-9 w-9"
                        disabled={index === exercises.length - 1}
                        onClick={() => move(index, 1)}
                        aria-label={`Bajar ${exercise.name}`}
                      >
                        <ArrowDown className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-9 w-9 text-muted-foreground"
                        onClick={() =>
                          setExercises((current) => current.filter((e) => e.key !== exercise.key))
                        }
                        aria-label={`Quitar ${exercise.name}`}
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>

                  <div className="mt-2 flex items-center gap-2">
                    <div className="flex flex-1 items-center gap-2">
                      <NumericField
                        value={String(exercise.targetSets)}
                        parse={parsePositive}
                        onCommit={(targetSets) => update(exercise.key, { targetSets })}
                        label={`Series objetivo de ${exercise.name}`}
                        className="h-10 w-16"
                      />
                      <span className="text-sm text-muted-foreground">series</span>
                    </div>
                    <span className="text-muted-foreground" aria-hidden>
                      ×
                    </span>
                    <div className="flex flex-1 items-center gap-2">
                      <NumericField
                        value={String(exercise.targetReps)}
                        parse={parsePositive}
                        onCommit={(targetReps) => update(exercise.key, { targetReps })}
                        label={`Repeticiones objetivo de ${exercise.name}`}
                        className="h-10 w-16"
                      />
                      <span className="text-sm text-muted-foreground">reps</span>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}

          <Button variant="outline" className="w-full" onClick={() => setPicking(true)}>
            <Plus className="mr-1 h-4 w-4" />
            Agregar ejercicio
          </Button>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancelar
          </Button>
          <Button onClick={() => save.mutate()} disabled={!canSave}>
            {save.isPending && <Loader2 className="mr-1 h-4 w-4 animate-spin" />}
            Guardar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Targets must stay above zero — the column CHECKs enforce the same. */
function parsePositive(raw: string): number | null {
  const value = Number(raw.trim())
  if (!Number.isFinite(value) || value < 1) return null
  return Math.round(value)
}
