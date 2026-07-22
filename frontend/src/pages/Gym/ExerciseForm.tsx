import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useGymApi } from '@/hooks/useGymApi'
import type { Exercise, TrackingType } from '@/types/gym.types'
import { exerciseFiltersKey } from './gymKeys'

interface ExerciseFormProps {
  open: boolean
  /** null creates a new exercise; an exercise edits it. */
  editing: Exercise | null
  onClose: () => void
  onSaved: (exercise: Exercise) => void
}

/** Offered when the catalog has no equipment value that fits. */
const NO_EQUIPMENT = 'Sin equipo'

const TRACKING_LABELS: Record<TrackingType, string> = {
  weight_reps: 'Peso y repeticiones',
  duration_distance: 'Tiempo y distancia',
  duration: 'Solo tiempo',
}

export default function ExerciseForm({ open, editing, onClose, onSaved }: ExerciseFormProps) {
  const queryClient = useQueryClient()
  const { listExerciseFilters, createExercise, updateExercise } = useGymApi()

  const [name, setName] = useState('')
  const [primaryMuscle, setPrimaryMuscle] = useState('')
  const [equipment, setEquipment] = useState(NO_EQUIPMENT)
  const [description, setDescription] = useState('')
  const [trackingType, setTrackingType] = useState<TrackingType>('weight_reps')
  const [loadedId, setLoadedId] = useState<number | null | undefined>(undefined)

  const { data: filterValues } = useQuery({
    queryKey: exerciseFiltersKey(),
    queryFn: () => listExerciseFilters(),
    staleTime: 5 * 60_000,
  })

  // Seeds the fields once per opened exercise, during render rather than in an
  // effect so they never paint one frame stale. See RoutineForm for the same
  // pattern and the reasoning behind the guard.
  const wantedId = open ? (editing?.id ?? null) : undefined
  if (loadedId !== wantedId) {
    setLoadedId(wantedId)
    setName(editing?.name ?? '')
    setPrimaryMuscle(editing?.primaryMuscle ?? '')
    setEquipment(editing?.equipment || NO_EQUIPMENT)
    setDescription(editing?.description ?? '')
    setTrackingType(editing?.trackingType ?? 'weight_reps')
  }

  const save = useMutation({
    mutationFn: () => {
      const input = {
        name: name.trim(),
        // The sentinel is a UI affordance, not a stored value: the column's
        // "no equipment" is an empty string.
        equipment: equipment === NO_EQUIPMENT ? '' : equipment,
        primaryMuscle,
        secondaryMuscle: editing?.secondaryMuscle ?? '',
        description: description.trim(),
        trackingType,
      }
      return editing ? updateExercise(editing.id, input) : createExercise(input)
    },
    onSuccess: (exercise) => {
      // Filter dropdowns are derived from the catalog, so a new muscle or
      // equipment value has to invalidate them.
      queryClient.invalidateQueries({ queryKey: ['gym', 'exercises'] })
      queryClient.invalidateQueries({ queryKey: exerciseFiltersKey() })
      toast.success(editing ? 'Ejercicio actualizado' : 'Ejercicio creado')
      onSaved(exercise)
      onClose()
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar el ejercicio')
    },
  })

  const canSave = name.trim() !== '' && primaryMuscle !== '' && !save.isPending

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? 'Editar ejercicio' : 'Nuevo ejercicio'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1">
            <Label htmlFor="exercise-name">Nombre</Label>
            <Input
              id="exercise-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Remo con toalla"
            />
          </div>

          <div className="space-y-1">
            <Label>Músculo principal</Label>
            <Select value={primaryMuscle} onValueChange={setPrimaryMuscle}>
              <SelectTrigger aria-label="Músculo principal">
                <SelectValue placeholder="Elegí un músculo" />
              </SelectTrigger>
              <SelectContent>
                {filterValues?.muscles.map((muscle) => (
                  <SelectItem key={muscle} value={muscle}>
                    {muscle}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label>Equipo</Label>
            <Select value={equipment} onValueChange={setEquipment}>
              <SelectTrigger aria-label="Equipo">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NO_EQUIPMENT}>{NO_EQUIPMENT}</SelectItem>
                {filterValues?.equipment.map((item) => (
                  <SelectItem key={item} value={item}>
                    {item}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label>Cómo se mide</Label>
            <Select
              value={trackingType}
              onValueChange={(value) => setTrackingType(value as TrackingType)}
            >
              <SelectTrigger aria-label="Cómo se mide">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(TRACKING_LABELS) as TrackingType[]).map((type) => (
                  <SelectItem key={type} value={type}>
                    {TRACKING_LABELS[type]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label htmlFor="exercise-description">Descripción</Label>
            <Textarea
              id="exercise-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Cómo se ejecuta el movimiento"
              rows={4}
            />
          </div>
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
