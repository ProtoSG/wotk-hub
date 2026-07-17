import { useCallback, useEffect, useRef, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2, CreditCard, ArrowLeftRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
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
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import type { Card } from '@/types/finance.types'
import FloatingActionButton from './FloatingActionButton'

const UNDO_WINDOW_MS = 4500

const CARD_COLORS = [
  '#863bff', '#3b82f6', '#10b981', '#f59e0b',
  '#ef4444', '#ec4899', '#06b6d4', '#8b5cf6',
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
      <div className="space-y-1">
        <Label>Banco</Label>
        <Input {...register('bank')} placeholder="Ej: BCP, Interbank" />
      </div>
      <div className="space-y-1">
        <Label>Últimos 4 dígitos</Label>
        <Input {...register('last4')} placeholder="1234" maxLength={4} />
        {errors.last4 && <p className="text-xs text-destructive">{errors.last4.message}</p>}
      </div>
      {!editCard && (
        <div className="space-y-1">
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
      <div className="space-y-1">
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
      <div className="space-y-1">
        <Label>Color</Label>
        <div className="mt-1 flex flex-wrap gap-2">
          {CARD_COLORS.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => setValue('color', c)}
              className="h-8 w-8 rounded-full border-2 transition-transform hover:scale-110"
              style={{ backgroundColor: c, borderColor: color === c ? '#000' : 'transparent' }}
            />
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

function CardForm({ open, onClose, onSaved, editCard }: CardFormProps) {
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

const transferSchema = z.object({
  toCardId: z.string().min(1, 'Elegí una tarjeta destino'),
  amount: z.number().positive('Debe ser mayor a 0'),
  date: z.string().min(1, 'Requerido'),
})

type TransferFormValues = z.infer<typeof transferSchema>

function transferDefaults(): TransferFormValues {
  return {
    toCardId: '',
    amount: 0,
    date: new Date().toISOString().split('T')[0],
  }
}

interface TransferFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  fromCard: Card
  cards: Card[]
}

// A card with a credit limit is a credit account with no spendable balance
// to move — filter it from the transfer destinations list.
function TransferForm({ open, onClose, onSaved, fromCard, cards }: TransferFormProps) {
  const { createCardTransfer } = useFinanceApi()
  const [saving, setSaving] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors },
  } = useForm<TransferFormValues>({
    resolver: zodResolver(transferSchema),
    defaultValues: transferDefaults(),
  })

  useEffect(() => {
    if (open) reset(transferDefaults())
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
          <div className="space-y-1">
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
          <div className="space-y-1">
            <Label>Fecha</Label>
            <Input type="date" {...register('date')} />
            {errors.date && <p className="text-xs text-destructive">{errors.date.message}</p>}
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

export default function TarjetasTab() {
  const [cards, setCards] = useState<Card[]>([])
  const [formOpen, setFormOpen] = useState(false)
  const [editCard, setEditCard] = useState<Card | undefined>()
  const [transferOpen, setTransferOpen] = useState(false)
  const [selectedCard, setSelectedCard] = useState<Card | null>(null)
  const { listCards, deleteCard } = useFinanceApi()
  const pendingDeletes = useRef(new Map<number, number>())

  const load = useCallback(async () => {
    try {
      setCards(await listCards())
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las tarjetas')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

  async function commitDelete(id: number) {
    pendingDeletes.current.delete(id)
    try {
      await deleteCard(id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la tarjeta')
      load()
    }
  }

  function handleDelete(c: Card) {
    let removedIndex = -1
    setCards((prev) => {
      removedIndex = prev.findIndex((x) => x.id === c.id)
      return prev.filter((x) => x.id !== c.id)
    })

    const timer = window.setTimeout(() => commitDelete(c.id), UNDO_WINDOW_MS)
    pendingDeletes.current.set(c.id, timer)

    toast.success('Tarjeta eliminada', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(c.id)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(c.id)
          }
          setCards((prev) => {
            const next = [...prev]
            next.splice(Math.min(removedIndex, next.length), 0, c)
            return next
          })
        },
      },
    })
  }

  return (
    <div className="space-y-4">
      <div className="hidden justify-end sm:flex">
        <Button
          onClick={() => {
            setEditCard(undefined)
            setFormOpen(true)
          }}
        >
          <Plus size={14} />
          Nueva tarjeta
        </Button>
      </div>

      <FloatingActionButton
        label="Nueva tarjeta"
        onClick={() => {
          setEditCard(undefined)
          setFormOpen(true)
        }}
      />

      {cards.length === 0 ? (
        <CozyCard className="animate-card-in">
          <CardContent className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground">
            <CreditCard className="h-10 w-10 opacity-30" />
            <p>No tienes tarjetas registradas</p>
            <button
              onClick={() => {
                setEditCard(undefined)
                setFormOpen(true)
              }}
              className="mt-1 text-sm text-primary hover:underline"
            >
              Agregar primera tarjeta
            </button>
          </CardContent>
        </CozyCard>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {cards.map((card, i) => {
            const hasCreditLimit = card.creditLimitCents > 0
            const utilization =
              hasCreditLimit && card.creditLimitCents > 0
                ? card.usedCreditCents / card.creditLimitCents
                : 0
            const utilizationColor =
              utilization > 0.8 ? 'bg-destructive' : utilization > 0.5 ? 'bg-amber-500' : 'bg-emerald-500'
            // The backend rejects archiving your last active card with 409
            // (cards.go DeleteCard). Disable the affordance here too so the
            // user doesn't trip the failure — the helpful title explains why.
            const isLastCard = cards.length === 1

            return (
              <CozyCard
                key={card.id}
                className="animate-card-in"
                style={{ animationDelay: `${Math.min(i * 40, 320)}ms`, borderTop: `4px solid ${card.color}` }}
              >
                <CardHeader className="flex flex-row items-start justify-between pb-2">
                  <div>
                    <CardTitle className="text-sm font-medium">{card.name}</CardTitle>
                    <p className="text-xs text-muted-foreground">
                      {card.bank ? `${card.bank}` : ''}
                      {card.bank && card.last4 ? ` · ` : ''}
                      {card.last4 ? `${card.last4}` : ''}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Editar tarjeta ${card.name}`}
                      onClick={() => {
                        setEditCard(card)
                        setFormOpen(true)
                      }}
                    >
                      <Pencil size={14} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      disabled={isLastCard}
                      aria-label={
                        isLastCard
                          ? `No podés archivar tu última tarjeta activa`
                          : `Eliminar tarjeta ${card.name}`
                      }
                      title={
                        isLastCard ? 'No podés archivar tu última tarjeta activa' : undefined
                      }
                      onClick={() => !isLastCard && handleDelete(card)}
                    >
                      <Trash2 size={14} />
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="space-y-2">
                  <div className="flex items-end justify-between">
                    {hasCreditLimit ? (
                      <div className="flex flex-col gap-1">
                        <p className="text-sm text-muted-foreground">
                          Usado:{' '}
                          <span className="font-medium text-foreground">
                            {formatPEN(card.usedCreditCents)}
                          </span>
                        </p>
                        <p className="text-2xl font-bold">
                          {formatPEN(card.creditLimitCents - card.usedCreditCents)}
                          <span className="ml-1 text-xs font-normal text-muted-foreground">
                            disponible
                          </span>
                        </p>
                      </div>
                    ) : (
                      <p className="text-2xl font-bold">{formatPEN(card.balanceCents)}</p>
                    )}
                    <div className="flex gap-1">
                      {!hasCreditLimit && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-xs"
                          onClick={() => {
                            setSelectedCard(card)
                            setTransferOpen(true)
                          }}
                        >
                          <ArrowLeftRight className="mr-1 h-3 w-3" />
                          Transferir
                        </Button>
                      )}
                    </div>
                  </div>
                  {hasCreditLimit && card.creditLimitCents > 0 && (
                    <div className="space-y-1">
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span>Límite: {formatPEN(card.creditLimitCents)}</span>
                        <span>{Math.round(utilization * 100)}% usado</span>
                      </div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                        <div
                          className={`h-full rounded-full transition-all ${utilizationColor}`}
                          style={{ width: `${Math.min(utilization * 100, 100)}%` }}
                        />
                      </div>
                    </div>
                  )}
                </CardContent>
              </CozyCard>
            )
          })}
        </div>
      )}

      <CardForm open={formOpen} onClose={() => setFormOpen(false)} onSaved={load} editCard={editCard} />
      {selectedCard && (
        <TransferForm
          open={transferOpen}
          onClose={() => setTransferOpen(false)}
          onSaved={load}
          fromCard={selectedCard}
          cards={cards}
        />
      )}
    </div>
  )
}
