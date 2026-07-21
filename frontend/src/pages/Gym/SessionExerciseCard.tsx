import { useEffect, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Check, Loader2, MoreVertical, Plus, RotateCcw, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useGymApi } from '@/hooks/useGymApi'
import { formatVolume } from '@/lib/weight'
import type { SessionExercise } from '@/types/gym.types'
import { activeSessionKey } from './gymKeys'
import ExerciseMedia from './ExerciseMedia'
import SetGrid from './SetGrid'
import { nextRow, rowsEqual, toInputs, toRows, type SetRow } from './setRows'

interface SessionExerciseCardProps {
  sessionId: number
  sessionExercise: SessionExercise
  onRemove: () => void
}

export default function SessionExerciseCard({
  sessionId,
  sessionExercise,
  onRemove,
}: SessionExerciseCardProps) {
  const [rows, setRows] = useState<SetRow[]>(() => toRows(sessionExercise.sets))
  const queryClient = useQueryClient()
  const { replaceSets, lastSets } = useGymApi()

  // Rows are local while editing. The server copy only reclaims them when it
  // actually differs from what is on screen, so a save round-trip can't
  // clobber a number typed while it was in flight.
  const serverSets = sessionExercise.sets
  const lastServerRows = useRef(toRows(serverSets))
  useEffect(() => {
    const incoming = toRows(serverSets)
    if (!rowsEqual(incoming, lastServerRows.current)) {
      lastServerRows.current = incoming
      setRows((current) => (rowsEqual(current, incoming) ? current : incoming))
    }
  }, [serverSets])

  const save = useMutation({
    mutationFn: (next: SetRow[]) => replaceSets(sessionId, sessionExercise.id, toInputs(next)),
    onSuccess: (session) => {
      queryClient.setQueryData(activeSessionKey(), session)
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudieron guardar las series')
    },
  })

  const commit = (next: SetRow[]) => {
    setRows(next)
    save.mutate(next)
  }

  const prefill = async () => {
    try {
      const result = await lastSets(sessionExercise.exerciseId)
      if (result.sets.length === 0) {
        toast.info('Todavía no hay registros de este ejercicio')
        return
      }
      // Copied as pending, not completed: the sets are a target to hit, not
      // work already done.
      commit(toRows(result.sets).map((row) => ({ ...row, completed: false })))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo copiar la última sesión')
    }
  }

  const completed = rows.filter((row) => row.completed && !row.isWarmup)
  const volume = completed.reduce((sum, row) => sum + row.reps * row.weightGrams, 0)
  const working = rows.filter((row) => !row.isWarmup)

  return (
    <section className="rounded-lg border bg-card">
      <header className="flex items-start gap-3 border-b px-4 py-3">
        <ExerciseMedia exercise={sessionExercise.exercise} className="h-10 w-10" />

        <div className="min-w-0 flex-1">
          <h3 className="truncate font-semibold">{sessionExercise.exercise.name}</h3>
          <p className="truncate text-sm text-muted-foreground">
            {[sessionExercise.exercise.primaryMuscle, sessionExercise.exercise.equipment]
              .filter(Boolean)
              .join(' · ')}
          </p>
        </div>

        {save.isPending && <Loader2 className="mt-1 h-4 w-4 animate-spin text-muted-foreground" />}

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Opciones del ejercicio">
              <MoreVertical className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={prefill}>
              <RotateCcw className="mr-2 h-4 w-4" />
              Copiar última sesión
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onRemove} className="text-destructive focus:text-destructive">
              <Trash2 className="mr-2 h-4 w-4" />
              Quitar del entrenamiento
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </header>

      <div className="px-3 py-3">
        {rows.length === 0 ? (
          <div className="px-1 pb-2 text-sm text-muted-foreground">
            Sin series todavía. Agregá la primera o copiá la última sesión.
          </div>
        ) : (
          <SetGrid rows={rows} onChange={commit} />
        )}

        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => commit([...rows, nextRow(rows)])}>
            <Plus className="mr-1 h-4 w-4" />
            Serie
          </Button>
          {rows.length === 0 && (
            <Button variant="ghost" size="sm" onClick={prefill}>
              <RotateCcw className="mr-1 h-4 w-4" />
              Copiar última
            </Button>
          )}

          {completed.length > 0 && (
            <p className="ml-auto flex items-center gap-1.5 text-sm text-muted-foreground tabular-nums">
              <Check className="h-3.5 w-3.5 text-success" />
              {completed.length}/{working.length} · {formatVolume(volume)}
            </p>
          )}
        </div>
      </div>
    </section>
  )
}
