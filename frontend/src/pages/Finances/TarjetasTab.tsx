import { useState, useEffect } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { Card as UICard } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { CreditCard, RefreshCw, Trash2, Pencil } from 'lucide-react'
import { toast } from 'sonner'
import type { Card, CardType } from '@/types/finance.types'

const CARD_COLORS = [
  '#863bff', '#3b82f6', '#10b981', '#f59e0b',
  '#ef4444', '#ec4899', '#06b6d4', '#8b5cf6',
]

const CARD_TYPES = [
  { value: 'debito', label: 'Débito' },
  { value: 'credito', label: 'Crédito' },
  { value: 'prepago', label: 'Prepago' },
]

const cardSchema = z.object({
  name: z.string().min(1, 'El nombre es requerido'),
  type: z.enum(['debito', 'credito', 'prepago']),
  bank: z.string(),
  last4: z.string().length(4, 'Debe tener 4 dígitos'),
  color: z.string(),
  icon: z.string(),
})

type CardFormValues = z.infer<typeof cardSchema>

interface CardFormProps {
  open: boolean
  onClose: () => void
  onSuccess: (card: Card) => void
  editCard?: Card
}

function cardDefaults(editCard?: Card): CardFormValues {
  return {
    name: editCard?.name ?? '',
    type: editCard?.type ?? 'debito',
    bank: editCard?.bank ?? '',
    last4: editCard?.last4 ?? '',
    color: editCard?.color ?? CARD_COLORS[0],
    icon: editCard?.icon ?? 'credit-card',
  }
}

export function CardForm({ open, onClose, onSuccess, editCard }: CardFormProps) {
  const { createCard, updateCard } = useFinanceApi()
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors },
  } = useForm<CardFormValues>({
    resolver: zodResolver(cardSchema),
    defaultValues: cardDefaults(editCard),
  })

  const color = watch('color')
  const type = watch('type')

  useEffect(() => {
    if (open) reset(cardDefaults(editCard))
  }, [open, editCard, reset])

  const onSubmit: SubmitHandler<CardFormValues> = async (values) => {
    setLoading(true)
    try {
      let card: Card
      if (editCard) {
        card = await updateCard(editCard.id, values)
        toast.success('Tarjeta actualizada')
      } else {
        card = await createCard(values)
        toast.success('Tarjeta creada')
      }
      onSuccess(card)
      onClose()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Error al guardar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editCard ? 'Editar tarjeta' : 'Nueva tarjeta'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Nombre</Label>
            <Input {...register('name')} placeholder="Ej: STM Lima, BCP Débito" />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Tipo</Label>
            <Select value={type} onValueChange={(v) => setValue('type', v as CardType)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {CARD_TYPES.map(t => (
                  <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            {errors.type && <p className="text-xs text-destructive">{errors.type.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Banco</Label>
            <Input {...register('bank')} placeholder="Ej: BCP, Interbank" />
          </div>
          <div className="space-y-1">
            <Label>Últimos 4 dígitos</Label>
            <Input {...register('last4')} placeholder="1234" maxLength={4} />
            {errors.last4 && <p className="text-xs text-destructive">{errors.last4.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Color</Label>
            <div className="flex gap-2 flex-wrap mt-1">
              {CARD_COLORS.map(c => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setValue('color', c)}
                  className="w-8 h-8 rounded-full border-2 transition-transform hover:scale-110"
                  style={{ backgroundColor: c, borderColor: color === c ? '#000' : 'transparent' }}
                />
              ))}
            </div>
            {errors.color && <p className="text-xs text-destructive">{errors.color.message}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Guardando...' : editCard ? 'Actualizar' : 'Crear'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface TarjetasTabProps {
  cards: Card[]
  onRefresh: () => void
}

export function TarjetasTab({ cards, onRefresh }: TarjetasTabProps) {
  const { deleteCard } = useFinanceApi()
  const [formOpen, setFormOpen] = useState(false)
  const [reloadOpen, setReloadOpen] = useState(false)
  const [selectedCard, setSelectedCard] = useState<Card | null>(null)
  const [editCard, setEditCard] = useState<Card | undefined>()

  const handleDelete = async (id: number) => {
    if (!confirm('¿Eliminar esta tarjeta?')) return
    try {
      await deleteCard(id)
      toast.success('Tarjeta eliminada')
      onRefresh()
    } catch {
      toast.error('Error al eliminar')
    }
  }

  const formatBalance = (cents: number) =>
    new Intl.NumberFormat('es-PE', { style: 'currency', currency: 'PEN' }).format(cents / 100)

  const typeLabel: Record<string, string> = {
    debito: 'Débito',
    credito: 'Crédito',
    prepago: 'Prepago',
  }

  return (
    <div className="space-y-4">
      {cards.length === 0 && (
        <UICard className="p-6 flex flex-col items-center gap-2 text-center text-muted-foreground">
          <CreditCard className="w-10 h-10 opacity-30" />
          <p>No tienes tarjetas registradas</p>
          <button
            onClick={() => { setEditCard(undefined); setFormOpen(true) }}
            className="mt-1 text-sm text-primary hover:underline"
          >
            Agregar primera tarjeta
          </button>
        </UICard>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {cards.map(card => (
          <UICard
            key={card.id}
            className="p-4 flex flex-col gap-3"
            style={{ borderTop: `4px solid ${card.color}` }}
          >
            <div className="flex justify-between items-start">
              <div>
                <p className="font-semibold">{card.name}</p>
                <p className="text-xs text-muted-foreground">
                  {typeLabel[card.type] ?? card.type}
                  {card.bank ? ` · ${card.bank}` : ''}
                  {card.last4 ? ` · ${card.last4}` : ''}
                </p>
              </div>
              <div className="flex gap-1">
                <button onClick={() => { setEditCard(card); setFormOpen(true) }} className="p-1 hover:bg-accent rounded">
                  <Pencil className="w-3 h-3 text-muted-foreground" />
                </button>
                <button onClick={() => handleDelete(card.id)} className="p-1 hover:bg-accent rounded">
                  <Trash2 className="w-3 h-3 text-destructive" />
                </button>
              </div>
            </div>
            <div className="flex justify-between items-end">
              <p className="text-2xl font-bold">{formatBalance(card.balanceCents)}</p>
              <Button size="sm" variant="ghost" className="text-xs" onClick={() => { setSelectedCard(card); setReloadOpen(true) }}>
                <RefreshCw className="w-3 h-3 mr-1" /> Recargar
              </Button>
            </div>
          </UICard>
        ))}
      </div>

      <CardForm open={formOpen} onClose={() => setFormOpen(false)} onSuccess={() => { setFormOpen(false); onRefresh() }} editCard={editCard} />
      {selectedCard && (
        <ReloadForm open={reloadOpen} onClose={() => setReloadOpen(false)} onSuccess={() => { setReloadOpen(false); onRefresh() }} card={selectedCard} />
      )}
    </div>
  )
}

interface ReloadFormProps {
  open: boolean
  onClose: () => void
  onSuccess: () => void
  card: Card
}

const reloadSchema = z.object({
  amount: z.number().positive('Debe ser mayor a 0'),
  date: z.string().min(1, 'Requerido'),
})

type ReloadFormValues = z.infer<typeof reloadSchema>

function reloadDefaults(): ReloadFormValues {
  return {
    amount: 0,
    date: new Date().toISOString().split('T')[0],
  }
}

function ReloadForm({ open, onClose, onSuccess, card }: ReloadFormProps) {
  const { createReload } = useFinanceApi()
  const [loading, setLoading] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ReloadFormValues>({
    resolver: zodResolver(reloadSchema),
    defaultValues: reloadDefaults(),
  })

  useEffect(() => {
    if (open) reset(reloadDefaults())
  }, [open, reset])

  const onSubmit: SubmitHandler<ReloadFormValues> = async (values) => {
    const amountCents = Math.round(values.amount * 100)
    setLoading(true)
    try {
      await createReload(card.id, { amountCents, date: values.date, note: '' })
      toast.success('Recarga registrada')
      onSuccess()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Error al recargar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Recargar {card.name}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Monto (PEN)</Label>
            <Input type="number" step="0.01" min="0.01" {...register('amount', { valueAsNumber: true })} placeholder="0.00" />
            {errors.amount && <p className="text-xs text-destructive">{errors.amount.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Fecha</Label>
            <Input type="date" {...register('date')} />
            {errors.date && <p className="text-xs text-destructive">{errors.date.message}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>Cancelar</Button>
            <Button type="submit" disabled={loading}>{loading ? 'Registrando...' : 'Recargar'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
