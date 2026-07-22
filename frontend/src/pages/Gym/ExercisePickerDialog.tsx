import { useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { Exercise } from '@/types/gym.types'
import ExerciseCatalog from './ExerciseCatalog'
import ExerciseDetail from './ExerciseDetail'

interface ExercisePickerDialogProps {
  open: boolean
  onClose: () => void
  onSelect: (exercise: Exercise) => void
}

/**
 * Catalog in picking mode. Clicking a row opens the exercise detail as a
 * second view of this same dialog — never a nested one, which Radix would
 * treat as an outside press and use to dismiss this dialog.
 */
export default function ExercisePickerDialog({
  open,
  onClose,
  onSelect,
}: ExercisePickerDialogProps) {
  const [detail, setDetail] = useState<Exercise | null>(null)

  const close = () => {
    setDetail(null)
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && close()}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-4 sm:max-w-2xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            {detail && (
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setDetail(null)}
                aria-label="Volver al catálogo"
                className="-ml-2 h-9 w-9"
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
            )}
            <DialogTitle className="text-left">{detail ? detail.name : 'Agregar ejercicio'}</DialogTitle>
          </div>
        </DialogHeader>

        <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
          {detail ? (
            <ExerciseDetail
              exercise={detail}
              onSelect={(exercise) => {
                onSelect(exercise)
                close()
              }}
            />
          ) : (
            <ExerciseCatalog onOpen={setDetail} />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
