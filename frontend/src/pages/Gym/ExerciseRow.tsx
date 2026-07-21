import { useState } from 'react'
import { Dumbbell } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { Exercise } from '@/types/gym.types'

interface ExerciseRowProps {
  exercise: Exercise
  /** When set, the row becomes a button that reports the picked exercise. */
  onSelect?: (exercise: Exercise) => void
}

export default function ExerciseRow({ exercise, onSelect }: ExerciseRowProps) {
  const selectable = Boolean(onSelect)

  return (
    <li>
      <div
        role={selectable ? 'button' : undefined}
        tabIndex={selectable ? 0 : undefined}
        onClick={() => onSelect?.(exercise)}
        onKeyDown={(e) => {
          if (!selectable) return
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onSelect?.(exercise)
          }
        }}
        className={cn(
          'flex min-h-[44px] items-center gap-3 rounded-lg border bg-card p-3 text-left',
          selectable &&
            'cursor-pointer transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        )}
      >
        <ExerciseThumb exercise={exercise} />

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

/**
 * Catalog media is hosted on a third-party bucket, so it is treated as
 * best-effort: a missing, empty or failing URL falls back to the icon instead
 * of leaving a broken image in the row. Videos get no poster frame, so they
 * use the same fallback.
 */
function ExerciseThumb({ exercise }: { exercise: Exercise }) {
  const [failed, setFailed] = useState(false)
  const showImage = exercise.mediaType === 'image' && exercise.mediaUrl !== '' && !failed

  if (!showImage) {
    return (
      <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
        <Dumbbell className="h-5 w-5" />
      </div>
    )
  }

  return (
    <img
      src={exercise.mediaUrl}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
      className="h-11 w-11 shrink-0 rounded-md bg-muted object-cover"
    />
  )
}
