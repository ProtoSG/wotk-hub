import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import type { SavingsGoal } from '@/types/finance.types'

interface ContributionFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  goal: SavingsGoal
}

const contributionSchema = z.object({
  amount: z.number().positive('Debe ser mayor a 0'),
  date: z.string().min(1, 'La fecha es requerida'),
})

type ContributionFormValues = z.infer<typeof contributionSchema>

export default function ContributionForm({ open, onClose, onSaved, goal }: ContributionFormProps) {
  const { createContribution } = useFinanceApi()
  const [saving, setSaving] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ContributionFormValues>({
    resolver: zodResolver(contributionSchema),
    defaultValues: {
      amount: 0,
      date: new Date().toISOString().split('T')[0],
    },
  })

  useEffect(() => {
    if (open) reset({ amount: 0, date: new Date().toISOString().split('T')[0] })
  }, [open, reset])

  const onSubmit: SubmitHandler<ContributionFormValues> = async (values) => {
    setSaving(true)
    try {
      await createContribution(goal.id, {
        amountCents: Math.round(values.amount * 100),
        date: values.date,
        note: '',
      })
      toast.success('Ahorro registrado')
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Error al agregar')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Agregar ahorro a {goal.name}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Monto (PEN)</Label>
            <Input
              type="number"
              step="0.01"
              min="0.01"
              {...register('amount', { valueAsNumber: true })}
              placeholder="0.00"
            />
            {errors.amount && (
              <p className="text-xs text-destructive">{errors.amount.message}</p>
            )}
          </div>
          <div className="space-y-1">
            <Label>Fecha</Label>
            <Input type="date" {...register('date')} />
            {errors.date && (
              <p className="text-xs text-destructive">{errors.date.message}</p>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Registrando...' : 'Agregar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
