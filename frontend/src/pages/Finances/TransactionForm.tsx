import { Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DateField } from '@/components/ui/date-field'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { useCategories } from '@/hooks/useCategories'
import { cn } from '@/lib/utils'
import { solesToCents, centsToSoles } from '@/lib/currency'
import { todayISO } from '@/lib/date'
import type { Transaction, Card } from '@/types/finance.types'

const schema = z.object({
  type: z.enum(['income', 'expense']),
  amount: z.number().positive('Debe ser mayor a 0'),
  category: z.string().min(1, 'Requerido'),
  date: z.string().min(1, 'Requerido'),
  description: z.string(),
  cardId: z.string().min(1, 'Elegí una tarjeta'),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editing?: Transaction | null
  defaultCardId?: number | null
}

function defaults(editing?: Transaction | null, defaultCardId?: number | null): Partial<FormValues> {
  return editing
    ? {
        // editing always comes from Movimientos, which never lists transfer
        // rows — this narrows TransactionKind back to the form's type.
        type: editing.type === 'income' ? 'income' : 'expense',
        amount: centsToSoles(editing.amountCents),
        category: editing.category,
        date: editing.date,
        description: editing.description,
        cardId: editing.cardId != null ? String(editing.cardId) : '',
      }
    : {
        type: 'expense',
        category: 'comida',
        // Not user-editable on create — the date field is only shown when
        // `editing`, so a new transaction always dates to today.
        date: todayISO(),
        description: '',
        cardId: defaultCardId != null ? String(defaultCardId) : '',
      }
}

export default function TransactionForm({ open, onClose, onSaved, editing, defaultCardId }: Props) {
  const [saving, setSaving] = useState(false)
  const [cards, setCards] = useState<Card[]>([])
  const { createTransaction, updateTransaction, listCards } = useFinanceApi()
  const { data: categoriesByKind, isLoading: categoriesLoading } = useCategories()

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
    if (open) reset(defaults(editing, defaultCardId))
  }, [open, editing, defaultCardId, reset])

  const type = watch('type')
  const category = watch('category')
  const cardId = watch('cardId')
  const categories = type === 'income' ? categoriesByKind.income : categoriesByKind.expense

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSaving(true)
    try {
      const input = {
        type: values.type,
        amountCents: solesToCents(values.amount),
        category: values.category,
        description: values.description,
        date: values.date,
        cardId: Number(values.cardId),
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
          <div className="flex rounded-lg bg-muted p-1">
            {(['expense', 'income'] as const).map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => {
                  setValue('type', v)
                  setValue('category', v === 'income' ? 'sueldo' : 'comida')
                }}
                className={cn(
                  'flex-1 rounded-md py-1.5 text-sm font-medium transition-colors',
                  type === v ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {v === 'expense' ? 'Gasto' : 'Ingreso'}
              </button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Monto</Label>
              <div className="relative">
                <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  S/
                </span>
                <Input
                  type="number"
                  step="0.01"
                  min="0"
                  placeholder="0.00"
                  className="pl-8"
                  {...register('amount', { valueAsNumber: true })}
                />
              </div>
              {errors.amount && <p className="text-xs text-destructive">{errors.amount.message}</p>}
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Categoría</Label>
              <Select value={category} onValueChange={(v) => setValue('category', v)} disabled={categoriesLoading}>
                <SelectTrigger>
                  <SelectValue placeholder={categoriesLoading ? 'Cargando…' : undefined} />
                </SelectTrigger>
                <SelectContent>
                  {categories.map((c) => (
                    <SelectItem key={c.id} value={c.name}>
                      {c.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          {editing && (
            <div className="space-y-1">
              <Label>Fecha</Label>
              <DateField value={watch('date')} onChange={(v) => setValue('date', v)} />
              {errors.date && <p className="text-xs text-destructive">{errors.date.message}</p>}
            </div>
          )}
          <div className="space-y-1">
            <Label>Descripción</Label>
            <Input placeholder="Almuerzo, taxi, etc." {...register('description')} />
          </div>
          <div className="space-y-1">
            <Label>Tarjeta</Label>
            <Select value={cardId} onValueChange={(v) => setValue('cardId', v)}>
              <SelectTrigger>
                <SelectValue placeholder="Elegí una tarjeta" />
              </SelectTrigger>
              <SelectContent>
                {cards.map((card) => (
                  <SelectItem key={card.id} value={String(card.id)}>
                    {card.name} ({card.last4})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {errors.cardId && <p className="text-xs text-destructive">{errors.cardId.message}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              {saving ? 'Guardando…' : 'Guardar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
