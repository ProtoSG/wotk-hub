import { useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import type { Card } from '@/types/finance.types'

// Derived from the app's --chart-1..8 tokens (index.css) so card swatches
// stay in the same warm terracotta family as the rest of the UI instead of
// the stock Tailwind rainbow. chart-1/3/4/6/7 were darkened just enough to
// clear 4.5:1 contrast against the white overlay text used on card faces
// (CardCarousel/TarjetasTab/MovimientosTab chip) — the chart tokens were
// only validated for contrast against the app background, not white text.
const CARD_COLORS = [
  '#b95c38', '#3d4f99', '#93702b', '#07819e',
  '#8b4aa6', '#49844b', '#b8586a', '#a6512a',
]

const cardSchema = z.object({
  name: z.string().min(1, 'El nombre es requerido'),
  bank: z.string(),
  last4: z.string().length(4, 'Debe tener 4 dígitos'),
  color: z.string(),
  icon: z.string(),
  initialBalance: z.number().min(0, 'No puede ser negativo').optional(),
  creditLimit: z.number().min(0, 'No puede ser negativo').optional(),
})

type CardFormValues = z.infer<typeof cardSchema>

function cardDefaults(editCard?: Card): CardFormValues {
  return {
    name: editCard?.name ?? '',
    bank: editCard?.bank ?? '',
    last4: editCard?.last4 ?? '',
    color: editCard?.color ?? CARD_COLORS[0],
    icon: editCard?.icon ?? 'credit-card',
    initialBalance: 0,
    creditLimit: editCard ? editCard.creditLimitCents / 100 : 0,
  }
}

interface CardFormFieldsProps {
  editCard?: Card
  onSaved: () => void
  onClose?: () => void
}

// Reusable card form body. Rendered inside the CardForm Dialog (TarjetasTab)
// and inline in the FinancesPage onboarding gate. Each open is a fresh mount
// (see `key` in the Dialog wrapper) so defaultValues apply cleanly and no
// manual reset effect is needed.
export function CardFormFields({ editCard, onSaved, onClose }: CardFormFieldsProps) {
  const { createCard, updateCard } = useFinanceApi()
  const [saving, setSaving] = useState(false)

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = useForm<CardFormValues>({
    resolver: zodResolver(cardSchema),
    defaultValues: cardDefaults(editCard),
  })

  const color = watch('color')

  const onSubmit: SubmitHandler<CardFormValues> = async (values) => {
    setSaving(true)
    try {
      const input = {
        name: values.name,
        bank: values.bank,
        last4: values.last4,
        color: values.color,
        icon: values.icon,
        creditLimitCents: Math.round((values.creditLimit ?? 0) * 100),
      }
      if (editCard) {
        // Balance isn't editable here — it's derived from transactions
        // (gastos/transferencias), not a field you overwrite.
        await updateCard(editCard.id, input)
        toast.success('Tarjeta actualizada')
      } else {
        await createCard({
          ...input,
          initialBalanceCents: Math.round((values.initialBalance ?? 0) * 100),
        })
        toast.success('Tarjeta creada')
      }
      onSaved()
      onClose?.()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Error al guardar')
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div className="space-y-1">
        <Label>Nombre</Label>
        <Input {...register('name')} placeholder="Ej: STM Lima, BCP Débito" />
        {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div className="min-w-0 space-y-1">
          <Label>Banco</Label>
          <Input {...register('bank')} placeholder="Ej: BCP, Interbank" />
        </div>
        <div className="min-w-0 space-y-1">
          <Label>Últimos 4 dígitos</Label>
          <Input {...register('last4')} placeholder="1234" maxLength={4} />
          {errors.last4 && <p className="text-xs text-destructive">{errors.last4.message}</p>}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        {!editCard && (
          <div className="min-w-0 space-y-1">
            <Label>Saldo inicial (opcional)</Label>
            <Input
              type="number"
              step="0.01"
              min="0"
              {...register('initialBalance', { valueAsNumber: true })}
              placeholder="0.00"
            />
            {errors.initialBalance && (
              <p className="text-xs text-destructive">{errors.initialBalance.message}</p>
            )}
          </div>
        )}
        <div className="min-w-0 space-y-1">
          <Label>Límite de crédito (opcional)</Label>
          <Input
            type="number"
            step="0.01"
            min="0"
            {...register('creditLimit', { valueAsNumber: true })}
            placeholder="0.00"
          />
          {errors.creditLimit && (
            <p className="text-xs text-destructive">{errors.creditLimit.message}</p>
          )}
        </div>
      </div>
      <div className="space-y-1">
        <Label>Color</Label>
        <div className="mt-1 flex flex-wrap gap-2">
          {CARD_COLORS.map((c) => (
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
        {errors.color && <p className="text-xs text-destructive">{errors.color.message}</p>}
      </div>
      <DialogFooter>
        {onClose && (
          <Button type="button" variant="outline" onClick={onClose}>
            Cancelar
          </Button>
        )}
        <Button type="submit" disabled={saving}>
          {saving ? 'Guardando...' : editCard ? 'Actualizar' : 'Crear'}
        </Button>
      </DialogFooter>
    </form>
  )
}

interface CardFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editCard?: Card
}

export default function CardForm({ open, onClose, onSaved, editCard }: CardFormProps) {
  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editCard ? 'Editar tarjeta' : 'Nueva tarjeta'}</DialogTitle>
        </DialogHeader>
        {/* key forces a fresh mount every time the dialog opens, so useForm
            defaultValues apply cleanly without a manual reset-on-open effect. */}
        {open && (
          <CardFormFields
            key="open"
            editCard={editCard}
            onSaved={onSaved}
            onClose={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
