import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '@/components/ui/dialog'
import { formatPEN } from '@/lib/currency'
import type { SavingsGoal } from '@/types/finance.types'

interface DeleteGoalDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  goal: SavingsGoal | null
}

export default function DeleteGoalDialog({ open, onClose, onConfirm, goal }: DeleteGoalDialogProps) {
  const [loading, setLoading] = useState(false)

  const handleConfirm = async () => {
    setLoading(true)
    try {
      await onConfirm()
    } finally {
      setLoading(false)
      onClose()
    }
  }

  const showWarning = goal != null && goal.currentCents > 0 && goal.defaultCardId != null

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Eliminar meta</DialogTitle>
          <DialogDescription>Esta acción no se puede deshacer.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            ¿Estás seguro de que quieres eliminar la meta{' '}
            <span className="font-medium text-foreground">{goal?.name}</span>?
          </p>
          {showWarning && (
            <div className="flex items-start gap-2 rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
              <span className="text-base">⚠️</span>
              <span>
                Esta meta tiene <strong>{formatPEN(goal!.currentCents)}</strong> acumulados. Si la
                eliminas, <strong>NO</strong> se reintegrará a la tarjeta.
              </span>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            Cancelar
          </Button>
          <Button type="button" variant="destructive" onClick={handleConfirm} disabled={loading}>
            {loading ? 'Eliminando...' : 'Eliminar'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
