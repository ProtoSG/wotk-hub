import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { Exercise } from '@/types/gym.types'
import ExerciseCatalog from './ExerciseCatalog'

interface ExercisePickerDialogProps {
  open: boolean
  onClose: () => void
  onSelect: (exercise: Exercise) => void
}

/**
 * Catalog in picking mode. Kept as a dialog rather than a route so choosing an
 * exercise never unmounts the session being logged underneath.
 */
export default function ExercisePickerDialog({
  open,
  onClose,
  onSelect,
}: ExercisePickerDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-4 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Agregar ejercicio</DialogTitle>
        </DialogHeader>
        <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
          <ExerciseCatalog
            onSelect={(exercise) => {
              onSelect(exercise)
              onClose()
            }}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
