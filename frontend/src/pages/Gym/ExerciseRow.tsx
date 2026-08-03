import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { Exercise } from '@/types/gym.types'
import ExerciseMedia from './ExerciseMedia'

interface ExerciseRowProps {
  exercise: Exercise
  /** Opens the exercise detail. Rows are always clickable. */
  onOpen: (exercise: Exercise) => void
}

export default function ExerciseRow({ exercise, onOpen }: ExerciseRowProps) {

  return (
    <li>
      <div
        role="button"
        tabIndex={0}
        onClick={() => onOpen(exercise)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onOpen(exercise)
          }
        }}
        className={cn(
          'flex min-h-[44px] cursor-pointer items-center gap-3 rounded-lg border bg-card p-3 text-left',
          'transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        )}
      >
        <ExerciseMedia exercise={exercise} className="h-11 w-11" />

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <p className="truncate font-medium">{exercise.name}</p>
            {exercise.isCustom && <Badge variant="secondary">Propio</Badge>}
          </div>
          <p className="truncate text-sm text-muted-foreground">
            {[exercise.primaryMuscle, exercise.equipment].filter(Boolean).join(' · ')}
            {exercise.secondaryMuscle && ` · ${exercise.secondaryMuscle}`}
          </p>
        </div>
      </div>
    </li>
  )
}
