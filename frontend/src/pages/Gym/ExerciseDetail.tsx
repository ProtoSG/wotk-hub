import { Pencil, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { Exercise } from '@/types/gym.types'
import ExerciseMedia from './ExerciseMedia'

interface ExerciseDetailProps {
  exercise: Exercise
  /** When set, an action button offers to add this exercise. */
  onSelect?: (exercise: Exercise) => void
  selectLabel?: string
  /** Editing and deleting are offered for custom exercises only. */
  onEdit?: (exercise: Exercise) => void
  onDelete?: (exercise: Exercise) => void
}

/**
 * Detail content without a container. The catalog is rendered both standalone
 * and inside dialogs, so this cannot own a Dialog of its own — a Radix dialog
 * nested in another dismisses its parent (see RoutineForm). Callers wrap it:
 * in a dialog when the catalog is standalone, or as another view of the dialog
 * they already have open.
 */
export default function ExerciseDetail({
  exercise,
  onSelect,
  selectLabel = 'Agregar ejercicio',
  onEdit,
  onDelete,
}: ExerciseDetailProps) {
  const secondary = exercise.secondaryMuscle
    .split(',')
    .map((muscle) => muscle.trim())
    .filter(Boolean)

  return (
    <div className="space-y-4">
      {/* Large enough to actually read the movement — the same media is a
          44px thumbnail in the list, where it only serves recognition.
          object-contain (overriding the thumbnail's cover) shows the whole
          frame: the source clips are wider than this box, so cropping to fill
          would cut off the limbs the demo is about. */}
      <ExerciseMedia
        exercise={exercise}
        // White, not the app surface: the source clips are rendered on a baked-in
        // white background, so any other colour would frame a hard white
        // rectangle inside it. Matching it makes the letterboxing disappear —
        // including in dark mode, where a tinted box would look like an error.
        className="h-56 w-full bg-white object-contain"
        autoPlay
      />

      <div className="flex flex-wrap gap-1.5">
        {exercise.primaryMuscle && <Badge>{exercise.primaryMuscle}</Badge>}
        {secondary.map((muscle) => (
          <Badge key={muscle} variant="secondary">
            {muscle}
          </Badge>
        ))}
        {exercise.equipment && <Badge variant="outline">{exercise.equipment}</Badge>}
      </div>

      {exercise.description ? (
        <p className="text-sm leading-relaxed text-muted-foreground">{exercise.description}</p>
      ) : (
        <p className="text-sm text-muted-foreground">
          Este ejercicio todavía no tiene descripción.
        </p>
      )}

      {onSelect && (
        <Button className="w-full" onClick={() => onSelect(exercise)}>
          {selectLabel}
        </Button>
      )}

      {exercise.isCustom && (onEdit || onDelete) && (
        <div className="flex gap-2 border-t pt-3">
          {onEdit && (
            <Button variant="outline" size="sm" onClick={() => onEdit(exercise)}>
              <Pencil className="mr-1 h-4 w-4" />
              Editar
            </Button>
          )}
          {onDelete && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onDelete(exercise)}
              className="text-destructive hover:text-destructive"
            >
              <Trash2 className="mr-1 h-4 w-4" />
              Eliminar
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
