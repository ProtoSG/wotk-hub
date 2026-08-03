import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import type { SavingsGoal, SavingsGoalInput } from '@/types/finance.types'
import { GOAL_COLORS } from '@/types/finance.types'
import { cardsKey } from './financeKeys'

const schema = z.object({
  name: z.string().min(1, 'El nombre es requerido'),
  targetCents: z.number().positive('Debe ser mayor a 0'),
  deadline: z.string().optional(),
  icon: z.string(),
  color: z.string(),
  defaultCardId: z.string().min(1, 'Elegí una tarjeta'),
})

type FormValues = z.infer<typeof schema>

function defaults(editGoal?: SavingsGoal): FormValues {
  return {
    name: editGoal?.name ?? '',
    targetCents: editGoal ? editGoal.targetCents / 100 : 0,
    deadline: editGoal?.deadline ?? '',
    icon: editGoal?.icon ?? 'piggy-bank',
    color: editGoal?.color ?? GOAL_COLORS[0],
    defaultCardId: editGoal ? String(editGoal.defaultCardId) : '',
  }
}

const GOAL_ICON_OPTIONS = [
  { value: 'piggy-bank', label: 'Ahorro' },
  { value: 'target', label: 'Meta' },
  { value: 'plane', label: 'Viaje' },
  { value: 'home', label: 'Casa' },
  { value: 'car', label: 'Auto' },
  { value: 'graduation-cap', label: 'Educación' },
]

interface GoalFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editGoal?: SavingsGoal
}

export default function GoalForm({ open, onClose, onSaved, editGoal }: GoalFormProps) {
  const { createGoal, updateGoal, listCards } = useFinanceApi()
  const { data: cards = [] } = useQuery({
    queryKey: cardsKey(),
    queryFn: () => listCards(),
  })
  const [saving, setSaving] = useState(false)

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaults(editGoal),
  })

  const color = watch('color')

  useEffect(() => {
    if (open) reset(defaults(editGoal))
  }, [open, editGoal, reset])

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    const input: SavingsGoalInput = {
      name: values.name,
      targetCents: Math.round(values.targetCents * 100),
      deadline: values.deadline || undefined,
      icon: values.icon,
      color: values.color,
      defaultCardId: parseInt(values.defaultCardId, 10),
    }
    setSaving(true)
    try {
      if (editGoal) {
        await updateGoal(editGoal.id, input)
        toast.success('Meta actualizada')
      } else {
        await createGoal(input)
        toast.success('Meta creada')
      }
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Error al guardar')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editGoal ? 'Editar meta' : 'Nueva meta de ahorro'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Nombre</Label>
            <Input {...register('name')} placeholder="Ej: Viaje a Cusco, Fondo emergencial" />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Monto objetivo (PEN)</Label>
              <Input
                type="number"
                step="0.01"
                min="0.01"
                {...register('targetCents', { valueAsNumber: true })}
                placeholder="500.00"
              />
              {errors.targetCents && (
                <p className="text-xs text-destructive">{errors.targetCents.message}</p>
              )}
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Fecha límite (opcional)</Label>
              <Input type="date" {...register('deadline')} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Icono</Label>
              <Select value={watch('icon')} onValueChange={(v) => setValue('icon', v)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {GOAL_ICON_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Tarjeta predeterminada</Label>
              <Select
                value={watch('defaultCardId')}
                onValueChange={(v) => setValue('defaultCardId', v)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Elegí una tarjeta" />
                </SelectTrigger>
                <SelectContent>
                  {cards
                    .filter((card) => card.creditLimitCents === 0)
                    .map((card) => (
                      <SelectItem key={card.id} value={card.id.toString()}>
                        {card.name} ({card.last4})
                      </SelectItem>
                    ))}
                </SelectContent>
              </Select>
              {errors.defaultCardId && (
                <p className="text-xs text-destructive">{errors.defaultCardId.message}</p>
              )}
            </div>
          </div>
          <p className="-mt-2 text-xs text-muted-foreground">
            Las aportaciones descontarán de esta tarjeta automáticamente
          </p>
          <div className="space-y-1">
            <Label>Color</Label>
            <div className="mt-1 flex flex-wrap gap-2">
              {GOAL_COLORS.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setValue('color', c)}
                  aria-label={c}
                  className="flex h-11 w-11 items-center justify-center rounded-full transition-transform hover:scale-110"
                >
                  <span
                    className="h-8 w-8 rounded-full border-2"
                    style={{ backgroundColor: c, borderColor: color === c ? '#000' : 'transparent' }}
                  />
                </button>
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Guardando...' : editGoal ? 'Actualizar' : 'Crear'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
