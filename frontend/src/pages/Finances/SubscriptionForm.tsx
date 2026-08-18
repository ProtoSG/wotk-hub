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
import { solesToCents, centsToSoles } from '@/lib/currency'
import { cn } from '@/lib/utils'
import { FREQUENCY_LABELS, type Subscription, type Frequency, type Card } from '@/types/finance.types'

const schema = z.object({
  name: z.string().min(1, 'Requerido'),
  amount: z.number().positive('Debe ser mayor a 0'),
  frequency: z.enum(['weekly', 'monthly', 'yearly']),
  type: z.enum(['income', 'expense']),
  category: z.string().min(1, 'Requerido'),
  nextBillingOn: z.string().min(1, 'Requerido'),
  active: z.boolean(),
  cardId: z.string().min(1, 'Elegí una tarjeta'),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editing?: Subscription | null
}

function defaults(editing?: Subscription | null): Partial<FormValues> {
  return editing
    ? {
        name: editing.name,
        amount: centsToSoles(editing.amountCents),
        frequency: editing.frequency,
        type: editing.type,
        category: editing.category,
        nextBillingOn: editing.nextBillingOn,
        active: editing.active,
        cardId: editing.cardId != null ? String(editing.cardId) : '',
      }
    : {
        name: '',
        frequency: 'monthly',
        type: 'expense',
        category: 'suscripciones',
        nextBillingOn: new Date().toISOString().slice(0, 10),
        active: true,
        cardId: '',
      }
}

export default function SubscriptionForm({ open, onClose, onSaved, editing }: Props) {
  const [saving, setSaving] = useState(false)
  const [cards, setCards] = useState<Card[]>([])
  const { createSubscription, updateSubscription, listCards } = useFinanceApi()
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
    listCards().then(setCards).catch(() => setCards([]))
  }, [])

  useEffect(() => {
    if (open) reset(defaults(editing))
  }, [open, editing, reset])

  const frequency = watch('frequency')
  const type = watch('type')
  const category = watch('category')
  const cardId = watch('cardId')
  const categories = type === 'income' ? categoriesByKind.income : categoriesByKind.expense

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSaving(true)
    try {
      const input = {
        name: values.name,
        amountCents: solesToCents(values.amount),
        frequency: values.frequency,
        type: values.type,
        category: values.category,
        nextBillingOn: values.nextBillingOn,
        active: values.active,
        cardId: Number(values.cardId),
      }
      if (editing) {
        await updateSubscription(editing.id, input)
      } else {
        await createSubscription(input)
      }
      toast.success(editing ? 'Suscripción actualizada' : 'Suscripción creada')
      reset()
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar la suscripción')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? 'Editar suscripción' : 'Nueva suscripción'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="flex rounded-lg bg-muted p-1">
            {(['expense', 'income'] as const).map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => {
                  setValue('type', v)
                  setValue('category', v === 'income' ? 'sueldo' : 'suscripciones')
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
              <Label>Nombre</Label>
              <Input placeholder="Netflix, sueldo, renta…" {...register('name')} />
              {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
            </div>
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
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Categoría</Label>
              {/* key={type}: see TransactionForm.tsx's identical comment —
                  Radix Select races changing `value` and `<SelectItem>`
                  list together, self-correcting the value to '' for real.
                  Remounting on type change sidesteps it. */}
              <Select
                key={type}
                value={category}
                onValueChange={(v) => setValue('category', v)}
                disabled={categoriesLoading}
              >
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
            <div className="min-w-0 space-y-1">
              <Label>Tarjeta</Label>
              <Select value={cardId} onValueChange={(v) => setValue('cardId', v)}>
                <SelectTrigger>
                  <SelectValue placeholder="Elegí una tarjeta" />
                </SelectTrigger>
                <SelectContent>
                  {cards.map((card) => (
                    <SelectItem key={card.id} value={card.id.toString()}>
                      {card.name} ({card.last4})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {type === 'income' ? 'Se deposita acá' : 'Se descuenta de acá'}
              </p>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Frecuencia</Label>
              <Select value={frequency} onValueChange={(v) => setValue('frequency', v as Frequency)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(FREQUENCY_LABELS) as Frequency[]).map((f) => (
                    <SelectItem key={f} value={f}>
                      {FREQUENCY_LABELS[f]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Próximo cobro</Label>
              <DateField value={watch('nextBillingOn')} onChange={(v) => setValue('nextBillingOn', v)} />
              {errors.nextBillingOn && (
                <p className="text-xs text-destructive">{errors.nextBillingOn.message}</p>
              )}
            </div>
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
