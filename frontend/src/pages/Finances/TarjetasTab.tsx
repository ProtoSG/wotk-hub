import { useState } from 'react'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { Card as UICard } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import { CreditCard, Plus, RefreshCw, Trash2, Pencil } from 'lucide-react'
import { toast } from 'sonner'
import type { Card } from '@/types/finance.types'

const CARD_COLORS = [
  '#863bff', '#3b82f6', '#10b981', '#f59e0b',
  '#ef4444', '#ec4899', '#06b6d4', '#8b5cf6',
]

const CARD_TYPES = [
  { value: 'debito', label: 'Débito' },
  { value: 'credito', label: 'Crédito' },
  { value: 'prepago', label: 'Prepago' },
]

interface CardFormProps {
  open: boolean
  onClose: () => void
  onSuccess: () => void
  editCard?: Card
}

export function CardForm({ open, onClose, onSuccess, editCard }: CardFormProps) {
  const { createCard, updateCard } = useFinanceApi()
  const [loading, setLoading] = useState(false)
  const [name, setName] = useState(editCard?.name ?? '')
  const [type, setType] = useState(editCard?.type ?? 'debito')
  const [bank, setBank] = useState(editCard?.bank ?? '')
  const [last4, setLast4] = useState(editCard?.last4 ?? '')
  const [color, setColor] = useState(editCard?.color ?? CARD_COLORS[0])
  const [icon] = useState(editCard?.icon ?? 'credit-card')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      toast.error('El nombre es requerido')
      return
    }
    setLoading(true)
    try {
      if (editCard) {
        await updateCard(editCard.id, { name, type, bank, last4, color, icon })
        toast.success('Tarjeta actualizada')
      } else {
        await createCard({ name, type, bank, last4, color, icon })
        toast.success('Tarjeta creada')
      }
      onSuccess()
      onClose()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Error al guardar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editCard ? 'Editar tarjeta' : 'Nueva tarjeta'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-sm font-medium">Nombre</label>
            <Input value={name} onChange={e => setName(e.target.value)} placeholder="Ej: STM Lima, BCP Débito" />
          </div>
          <div>
            <label className="text-sm font-medium">Tipo</label>
            <Select value={type} onValueChange={setType}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {CARD_TYPES.map(t => (
                  <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div>
            <label className="text-sm font-medium">Banco</label>
            <Input value={bank} onChange={e => setBank(e.target.value)} placeholder="Ej: BCP, Interbank" />
          </div>
          <div>
            <label className="text-sm font-medium">Últimos 4 dígitos</label>
            <Input value={last4} onChange={e => setLast4(e.target.value)} placeholder="1234" maxLength={4} />
          </div>
          <div>
            <label className="text-sm font-medium">Color</label>
            <div className="flex gap-2 flex-wrap mt-1">
              {CARD_COLORS.map(c => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setColor(c)}
                  className="w-8 h-8 rounded-full border-2 transition-transform hover:scale-110"
                  style={{ backgroundColor: c, borderColor: color === c ? '#000' : 'transparent' }}
                />
              ))}
            </div>
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
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold">Mis tarjetas</h2>
        <Button size="sm" onClick={() => { setEditCard(undefined); setFormOpen(true) }}>
          <Plus className="w-4 h-4 mr-1" /> Nueva
        </Button>
      </div>

      {cards.length === 0 && (
        <div className="text-center py-8 text-muted-foreground">
          <CreditCard className="w-12 h-12 mx-auto mb-2 opacity-30" />
          <p>No tienes tarjetas registradas</p>
        </div>
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

      <CardForm open={formOpen} onClose={() => setFormOpen(false)} onSuccess={onRefresh} editCard={editCard} />
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

function ReloadForm({ open, onClose, onSuccess, card }: ReloadFormProps) {
  const { createReload } = useFinanceApi()
  const [loading, setLoading] = useState(false)
  const [amount, setAmount] = useState('')
  const [date, setDate] = useState(new Date().toISOString().split('T')[0])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const amountCents = Math.round(parseFloat(amount) * 100)
    if (!amount || isNaN(amountCents) || amountCents <= 0) {
      toast.error('Monto inválido')
      return
    }
    setLoading(true)
    try {
      await createReload(card.id, { amountCents, date, note: '' })
      toast.success('Recarga registrada')
      setAmount('')
      onSuccess()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Error al recargar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Recargar {card.name}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-sm font-medium">Monto (PEN)</label>
            <Input type="number" step="0.01" min="0.01" value={amount} onChange={e => setAmount(e.target.value)} placeholder="0.00" />
          </div>
          <div>
            <label className="text-sm font-medium">Fecha</label>
            <Input type="date" value={date} onChange={e => setDate(e.target.value)} />
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
