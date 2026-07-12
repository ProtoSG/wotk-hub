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
import { solesToCents, centsToSoles } from '@/lib/currency'
import {
  EXPENSE_CATEGORIES,
  CATEGORY_LABELS,
  FREQUENCY_LABELS,
  type Subscription,
  type Frequency,
} from '@/types/finance.types'

const schema = z.object({
  name: z.string().min(1, 'Requerido'),
  amount: z.number().positive('Debe ser mayor a 0'),
  frequency: z.enum(['weekly', 'monthly', 'yearly']),
  category: z.string().min(1, 'Requerido'),
  nextBillingOn: z.string().min(1, 'Requerido'),
  active: z.boolean(),
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
      }
    : {
        name: '',
        amount: 0,
        frequency: 'monthly',
        category: 'suscripciones',
        nextBillingOn: new Date().toISOString().slice(0, 10),
        active: true,
      }
}

export default function SubscriptionForm({ open, onClose, onSaved, editing }: Props) {
  const [saving, setSaving] = useState(false)
  const { createSubscription, updateSubscription } = useFinanceApi()

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
    if (open) reset(defaults(editing))
  }, [open, editing, reset])

  const frequency = watch('frequency')
  const category = watch('category')
  const active = watch('active')

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
            <Select value={category} onValueChange={(v) => setValue('category', v)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {EXPENSE_CATEGORIES.map((c) => (
                  <SelectItem key={c} value={c}>
                    {CATEGORY_LABELS[c] ?? c}
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
