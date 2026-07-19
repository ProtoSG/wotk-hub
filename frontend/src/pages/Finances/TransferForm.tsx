import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import type { Card } from '@/types/finance.types'

interface TransferFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  fromCard: Card
  cards: Card[]
}

// A card with a credit limit is a credit account with no spendable balance
// to move — filter it from the transfer destinations list.
export default function TransferForm({ open, onClose, onSaved, fromCard, cards }: TransferFormProps) {
  const { createCardTransfer } = useFinanceApi()
  const [saving, setSaving] = useState(false)

  const transferSchema = z
    .object({
      toCardId: z.string().min(1, 'Elegí una tarjeta destino'),
      amount: z.number().positive('Debe ser mayor a 0'),
      date: z.string().min(1, 'Requerido'),
    })
    .refine((data) => fromCard.id !== Number(data.toCardId), {
      message: 'No puedes transferir a la misma tarjeta',
      path: ['toCardId'],
    })

  type TransferFormValues = z.infer<typeof transferSchema>

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors },
  } = useForm<TransferFormValues>({
    resolver: zodResolver(transferSchema),
    defaultValues: {
      toCardId: '',
      amount: 0,
      date: new Date().toISOString().split('T')[0],
    },
  })

  useEffect(() => {
    if (open) reset({ toCardId: '', amount: 0, date: new Date().toISOString().split('T')[0] })
  }, [open, reset])

  const destinations = cards.filter((c) => c.id !== fromCard.id && c.creditLimitCents === 0)

  const onSubmit: SubmitHandler<TransferFormValues> = async (values) => {
    setSaving(true)
    try {
      await createCardTransfer({
        fromCardId: fromCard.id,
        toCardId: Number(values.toCardId),
        amountCents: Math.round(values.amount * 100),
        date: values.date,
        note: '',
      })
      toast.success('Transferencia registrada')
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Error al transferir')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Transferir desde {fromCard.name}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Tarjeta destino</Label>
            <Select value={watch('toCardId')} onValueChange={(v) => setValue('toCardId', v)}>
              <SelectTrigger>
                <SelectValue placeholder="Elegí una tarjeta" />
              </SelectTrigger>
              <SelectContent>
                {destinations.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>
                    {c.name} ({c.last4})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {errors.toCardId && <p className="text-xs text-destructive">{errors.toCardId.message}</p>}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Monto (PEN)</Label>
              <Input
                type="number"
                step="0.01"
                min="0.01"
                {...register('amount', { valueAsNumber: true })}
                placeholder="0.00"
              />
              {errors.amount && <p className="text-xs text-destructive">{errors.amount.message}</p>}
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Fecha</Label>
              <Input type="date" {...register('date')} />
              {errors.date && <p className="text-xs text-destructive">{errors.date.message}</p>}
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Transfiriendo...' : 'Transferir'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
