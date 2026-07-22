import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ClipboardList, Dumbbell, MoreVertical, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { EmptyState } from '@/components/ui/empty-state'
import { Skeleton } from '@/components/ui/skeleton'
import { useGymApi } from '@/hooks/useGymApi'
import type { RoutineSummary } from '@/types/gym.types'
import { routinesKey } from './gymKeys'
import RoutineForm from './RoutineForm'

interface RutinasTabProps {
  formOpen: boolean
  onFormOpenChange: (open: boolean) => void
  onStart: (routine: RoutineSummary) => void
  starting: boolean
}

export default function RutinasTab({
  formOpen,
  onFormOpenChange,
  onStart,
  starting,
}: RutinasTabProps) {
  const [editingId, setEditingId] = useState<number | null>(null)
  const queryClient = useQueryClient()
  const { listRoutines, deleteRoutine } = useGymApi()

  const { data: routines = [], isPending } = useQuery({
    queryKey: routinesKey(),
    queryFn: () => listRoutines(),
  })

  const remove = useMutation({
    mutationFn: (id: number) => deleteRoutine(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: routinesKey() })
      toast.success('Rutina eliminada')
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la rutina')
    },
  })

  const openNew = () => {
    setEditingId(null)
    onFormOpenChange(true)
  }

  const openEdit = (id: number) => {
    setEditingId(id)
    onFormOpenChange(true)
  }

  if (isPending) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-20 w-full rounded-lg" />
        ))}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {routines.length === 0 ? (
        <EmptyState
          icon={<ClipboardList />}
          title="Sin rutinas todavía"
          description="Armá una plantilla con los ejercicios de un día y empezá desde ahí, sin volver a elegirlos uno por uno."
          action={{ label: 'Crear rutina', onClick: openNew }}
        />
      ) : (
        <ul className="space-y-2">
          {routines.map((routine) => (
            <li key={routine.id} className="flex items-center gap-3 rounded-lg border bg-card px-4 py-3">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                <Dumbbell className="h-5 w-5" />
              </div>

              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{routine.name}</p>
                <p className="truncate text-sm text-muted-foreground">
                  {routine.exerciseCount === 0
                    ? 'Sin ejercicios'
                    : `${routine.exerciseCount} ${routine.exerciseCount === 1 ? 'ejercicio' : 'ejercicios'}`}
                </p>
              </div>

              <Button
                size="sm"
                onClick={() => onStart(routine)}
                disabled={starting || routine.exerciseCount === 0}
              >
                <Play className="mr-1 h-4 w-4" />
                Empezar
              </Button>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" aria-label={`Opciones de ${routine.name}`}>
                    <MoreVertical className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => openEdit(routine.id)}>
                    <Pencil className="mr-2 h-4 w-4" />
                    Editar
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => remove.mutate(routine.id)}
                    className="text-destructive focus:text-destructive"
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    Eliminar
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </li>
          ))}
        </ul>
      )}

      {routines.length > 0 && (
        <Button variant="outline" className="hidden w-full sm:flex" onClick={openNew}>
          <Plus className="mr-1 h-4 w-4" />
          Nueva rutina
        </Button>
      )}

      <RoutineForm
        open={formOpen}
        routineId={editingId}
        onClose={() => onFormOpenChange(false)}
      />
    </div>
  )
}
