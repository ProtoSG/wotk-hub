import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { solesToCents, centsToSoles } from '@/lib/currency'
import {
  EXPENSE_CATEGORIES,
  INCOME_CATEGORIES,
  CATEGORY_LABELS,
  type Transaction,
  type TransactionType,
  type Card,
} from '@/types/finance.types'

const schema = z.object({
  type: z.enum(['income', 'expense']),
  amount: z.number().positive('Debe ser mayor a 0'),
  category: z.string().min(1, 'Requerido'),
  date: z.string().min(1, 'Requerido'),
  description: z.string(),
  cardId: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editing?: Transaction | null
}

function defaults(editing?: Transaction | null): FormValues {
  return editing
    ? {
        // editing always comes from Movimientos, which never lists transfer
        // rows — this narrows TransactionKind back to the form's type.
        type: editing.type === 'income' ? 'income' : 'expense',
        amount: centsToSoles(editing.amountCents),
        category: editing.category,
        date: editing.date,
        description: editing.description,
        cardId: editing.cardId != null ? String(editing.cardId) : undefined,
      }
    : {
        type: 'expense',
        amount: 0,
        category: 'comida',
        date: new Date().toISOString().slice(0, 10),
        description: '',
        cardId: undefined,
      }
}

export default function TransactionForm({ open, onClose, onSaved, editing }: Props) {
  const [saving, setSaving] = useState(false)
  const [cards, setCards] = useState<Card[]>([])
  const { createTransaction, updateTransaction, listCards } = useFinanceApi()

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaults(editing),
  })

  useEffect(() => {
    if (open) {
      listCards().then(setCards).catch(() => setCards([]))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  useEffect(() => {
    if (open) reset(defaults(editing))
  }, [open, editing, reset])

  const type = watch('type')
  const category = watch('category')
  const cardId = watch('cardId')
  const categories = type === 'income' ? INCOME_CATEGORIES : EXPENSE_CATEGORIES

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSaving(true)
    try {
      const input = {
        type: values.type,
        amountCents: solesToCents(values.amount),
        category: values.category,
        description: values.description,
        date: values.date,
        cardId: values.cardId ? Number(values.cardId) : undefined,
      }
      if (editing) {
        await updateTransaction(editing.id, input)
      } else {
        await createTransaction(input)
      }
      toast.success(editing ? 'Movimiento actualizado' : 'Movimiento registrado')
      reset()
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar el movimiento')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? 'Editar movimiento' : 'Nuevo movimiento'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Tipo</Label>
              <Select
                value={type}
                onValueChange={(v) => {
                  setValue('type', v as TransactionType)
                  setValue('category', v === 'income' ? 'sueldo' : 'comida')
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="expense">Gasto</SelectItem>
                  <SelectItem value="income">Ingreso</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Monto (S/)</Label>
              <Input type="number" step="0.01" min="0" {...register('amount', { valueAsNumber: true })} />
              {errors.amount && <p className="text-xs text-destructive">{errors.amount.message}</p>}
            </div>
          </div>
          <div className="space-y-1">
            <Label>Categoría</Label>
            <Select value={category} onValueChange={(v) => setValue('category', v)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {categories.map((c) => (
                  <SelectItem key={c} value={c}>
                    {CATEGORY_LABELS[c] ?? c}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Fecha</Label>
            <div className="w-1/2">
              <Input type="date" {...register('date')} />
            </div>
            {errors.date && <p className="text-xs text-destructive">{errors.date.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Descripción</Label>
            <Input placeholder="Almuerzo, taxi, etc." {...register('description')} />
          </div>
          <div className="space-y-1">
            <Label>Tarjeta (opcional)</Label>
            <Select value={cardId ?? ''} onValueChange={(v) => setValue('cardId', v || undefined)}>
              <SelectTrigger>
                <SelectValue placeholder="Sin tarjeta" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">Ninguna</SelectItem>
                {cards.map((card) => (
                  <SelectItem key={card.id} value={String(card.id)}>
                    {card.name} ({card.last4})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving && <Loader2 size={14} className="animate-spin" />}
              {saving ? 'Guardando…' : 'Guardar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
