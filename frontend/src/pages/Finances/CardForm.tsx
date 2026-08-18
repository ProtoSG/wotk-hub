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
import { CARD_ICON_MAP, CARD_ICON_OPTIONS } from './cardIcons'

const CARD_COLORS = [
  '#b95c38', '#3d4f99', '#93702b', '#07819e',
  '#8b4aa6', '#49844b', '#b8586a', '#a6512a', '#7c6a5b',
]

const cardSchema = z.object({
  name: z.string().min(1, 'El nombre es requerido'),
  bank: z.string(),
  last4: z.string().length(4, 'Debe tener 4 dígitos'),
  color: z.string(),
  icon: z.string(),
  initialBalance: z.number().min(0, 'No puede ser negativo').optional(),
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
  }
}

interface CardFormFieldsProps {
  editCard?: Card
  onSaved: () => void
  onClose?: () => void
}

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
  const icon = watch('icon')

  const onSubmit: SubmitHandler<CardFormValues> = async (values) => {
    setSaving(true)
    try {
      const input = {
        name: values.name,
        bank: values.bank,
        last4: values.last4,
        color: values.color,
        icon: values.icon,
        creditLimitCents: 0,
      }
      if (editCard) {
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
      {!editCard && (
        <div className="space-y-1">
          <Label>Saldo inicial (opcional)</Label>
          <div className="relative">
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
              S/
            </span>
            <Input
              type="number"
              step="0.01"
              min="0"
              className="pl-8"
              {...register('initialBalance', { valueAsNumber: true })}
              placeholder="0.00"
            />
          </div>
          {errors.initialBalance && (
            <p className="text-xs text-destructive">{errors.initialBalance.message}</p>
          )}
        </div>
      )}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1">
          <Label>Icono</Label>
          <div className="flex flex-wrap gap-1.5">
            {CARD_ICON_OPTIONS.map((opt) => {
              const Icon = CARD_ICON_MAP[opt.value]
              const selected = icon === opt.value
              return (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => setValue('icon', opt.value)}
                  aria-label={opt.label}
                  title={opt.label}
                  className={`flex h-9 w-9 items-center justify-center rounded-full border-2 transition-transform hover:scale-110 ${
                    selected ? 'border-primary bg-primary/10 text-primary' : 'border-transparent bg-muted text-muted-foreground'
                  }`}
                >
                  <Icon className="h-4 w-4" />
                </button>
              )
            })}
          </div>
          {errors.icon && <p className="text-xs text-destructive">{errors.icon.message}</p>}
        </div>
        <div className="space-y-1">
          <Label>Color</Label>
          <div className="flex flex-wrap gap-1.5">
            {CARD_COLORS.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => setValue('color', c)}
                aria-label={c}
                className="flex h-9 w-9 items-center justify-center rounded-full transition-transform hover:scale-110"
              >
                <span
                  className="h-7 w-7 rounded-full border-2"
                  style={{ backgroundColor: c, borderColor: color === c ? '#000' : 'transparent' }}
                />
              </button>
            ))}
          </div>
          {errors.color && <p className="text-xs text-destructive">{errors.color.message}</p>}
        </div>
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
