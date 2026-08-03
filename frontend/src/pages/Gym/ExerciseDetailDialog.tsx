import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { Exercise } from '@/types/gym.types'
import ExerciseDetail from './ExerciseDetail'

interface ExerciseDetailDialogProps {
  exercise: Exercise | null
  onClose: () => void
  onEdit?: (exercise: Exercise) => void
  onDelete?: (exercise: Exercise) => void
}

/**
 * Dialog wrapper for the exercise detail. Only safe where the catalog is NOT
 * already inside a dialog — the picker and the routine builder show the same
 * content as another view of their own dialog instead.
 */
export default function ExerciseDetailDialog({
  exercise,
  onClose,
  onEdit,
  onDelete,
}: ExerciseDetailDialogProps) {
  return (
    <Dialog open={exercise !== null} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-md">
        {exercise && (
          <>
            <DialogHeader>
              <DialogTitle className="pr-6 text-left">{exercise.name}</DialogTitle>
            </DialogHeader>
            <ExerciseDetail exercise={exercise} onEdit={onEdit} onDelete={onDelete} />
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
