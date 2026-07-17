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
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { useCategories } from '@/hooks/useCategories'
import { solesToCents, centsToSoles } from '@/lib/currency'
import {
  FREQUENCY_LABELS,
  type Subscription,
  type Frequency,
  type Card,
} from '@/types/finance.types'

const schema = z.object({
  name: z.string().min(1, 'Requerido'),
  amount: z.number().positive('Debe ser mayor a 0'),
  frequency: z.enum(['weekly', 'monthly', 'yearly']),
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

function defaults(editing?: Subscription | null): FormValues {
  return editing
    ? {
        name: editing.name,
        amount: centsToSoles(editing.amountCents),
        frequency: editing.frequency,
        category: editing.category,
        nextBillingOn: editing.nextBillingOn,
        active: editing.active,
        cardId: editing.cardId != null ? String(editing.cardId) : '',
      }
    : {
        name: '',
        amount: 0,
        frequency: 'monthly',
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
  const category = watch('category')
  const active = watch('active')
  const cardId = watch('cardId')

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSaving(true)
    try {
      const input = {
        name: values.name,
        amountCents: solesToCents(values.amount),
        frequency: values.frequency,
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
          <div className="space-y-1">
            <Label>Nombre</Label>
            <Input placeholder="Netflix, renta, gimnasio…" {...register('name')} />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Monto (S/)</Label>
              <Input type="number" step="0.01" min="0" {...register('amount', { valueAsNumber: true })} />
              {errors.amount && <p className="text-xs text-destructive">{errors.amount.message}</p>}
            </div>
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
          </div>
          <div className="space-y-1">
            <Label>Categoría</Label>
            <Select value={category} onValueChange={(v) => setValue('category', v)} disabled={categoriesLoading}>
              <SelectTrigger>
                <SelectValue placeholder={categoriesLoading ? 'Cargando…' : undefined} />
              </SelectTrigger>
              <SelectContent>
                {categoriesByKind.expense.map((c) => (
                  <SelectItem key={c.id} value={c.name}>
                    {c.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Próximo cobro</Label>
            <div className="w-1/2">
              <Input type="date" {...register('nextBillingOn')} />
            </div>
            {errors.nextBillingOn && (
              <p className="text-xs text-destructive">{errors.nextBillingOn.message}</p>
            )}
          </div>
          <div className="space-y-1">
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
            <p className="mt-1 text-xs text-muted-foreground">
              Los cobros automáticos descontarán de esta tarjeta
            </p>
          </div>
          <div className="flex items-center justify-between">
            <Label>Activa</Label>
            <Switch checked={active} onCheckedChange={(v) => setValue('active', v)} />
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
