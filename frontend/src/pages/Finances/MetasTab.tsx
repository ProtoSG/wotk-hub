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
import { PiggyBank, Plus, Trash2, Pencil, TrendingUp, Calendar } from 'lucide-react'
import FloatingActionButton from './FloatingActionButton'
import { toast } from 'sonner'
import type { SavingsGoal, SavingsGoalInput } from '@/types/finance.types'
import { GOAL_COLORS } from '@/types/finance.types'

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
  onSuccess: (goal: SavingsGoal) => void
  editGoal?: SavingsGoal
}

export function GoalForm({ open, onClose, onSuccess, editGoal }: GoalFormProps) {
  const { createGoal, updateGoal } = useFinanceApi()
  const [loading, setLoading] = useState(false)
  const [name, setName] = useState(editGoal?.name ?? '')
  const [targetCents, setTargetCents] = useState(
    editGoal ? (editGoal.targetCents / 100).toFixed(2) : ''
  )
  const [deadline, setDeadline] = useState(editGoal?.deadline ?? '')
  const [icon, setIcon] = useState(editGoal?.icon ?? 'piggy-bank')
  const [color, setColor] = useState(editGoal?.color ?? GOAL_COLORS[0])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      toast.error('El nombre es requerido')
      return
    }
    const target = Math.round(parseFloat(targetCents) * 100)
    if (isNaN(target) || target <= 0) {
      toast.error('Monto objetivo inválido')
      return
    }
    setLoading(true)
    try {
      const input: SavingsGoalInput = {
        name,
        targetCents: target,
        deadline: deadline || undefined,
        icon,
        color,
      }
      let goal: SavingsGoal
      if (editGoal) {
        goal = await updateGoal(editGoal.id, input)
        toast.success('Meta actualizada')
      } else {
        goal = await createGoal(input)
        toast.success('Meta creada')
      }
      onSuccess(goal)
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
          <DialogTitle>{editGoal ? 'Editar meta' : 'Nueva meta de ahorro'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-sm font-medium">Nombre</label>
            <Input
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Ej: Viaje a Cusco, Fondo emergencial"
            />
          </div>
          <div>
            <label className="text-sm font-medium">Monto objetivo (PEN)</label>
            <Input
              type="number"
              step="0.01"
              min="0.01"
              value={targetCents}
              onChange={e => setTargetCents(e.target.value)}
              placeholder="500.00"
            />
          </div>
          <div>
            <label className="text-sm font-medium">Fecha límite (opcional)</label>
            <Input
              type="date"
              value={deadline}
              onChange={e => setDeadline(e.target.value)}
            />
          </div>
          <div>
            <label className="text-sm font-medium">Icono</label>
            <Select value={icon} onValueChange={val => setIcon(val)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {GOAL_ICON_OPTIONS.map(opt => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div>
            <label className="text-sm font-medium">Color</label>
            <div className="flex gap-2 flex-wrap mt-1">
              {GOAL_COLORS.map(c => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setColor(c)}
                  className="w-8 h-8 rounded-full border-2 transition-transform hover:scale-110"
                  style={{
                    backgroundColor: c,
                    borderColor: color === c ? '#000' : 'transparent',
                  }}
                />
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Guardando...' : editGoal ? 'Actualizar' : 'Crear'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface ContributionFormProps {
  open: boolean
  onClose: () => void
  onSuccess: () => void
  goal: SavingsGoal
}

export function ContributionForm({ open, onClose, onSuccess, goal }: ContributionFormProps) {
  const { createContribution } = useFinanceApi()
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
      await createContribution(goal.id, { amountCents, date, note: '' })
      toast.success('Ahorro registrado')
      setAmount('')
      onSuccess()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Error al agregar')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Agregar ahorro a {goal.name}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-sm font-medium">Monto (PEN)</label>
            <Input
              type="number"
              step="0.01"
              min="0.01"
              value={amount}
              onChange={e => setAmount(e.target.value)}
              placeholder="0.00"
            />
          </div>
          <div>
            <label className="text-sm font-medium">Fecha</label>
            <Input type="date" value={date} onChange={e => setDate(e.target.value)} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Registrando...' : 'Agregar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface MetasTabProps {
  goals: SavingsGoal[]
  onRefresh: () => void
}

export function MetasTab({ goals, onRefresh }: MetasTabProps) {
  const { deleteGoal } = useFinanceApi()
  const [formOpen, setFormOpen] = useState(false)
  const [contributionOpen, setContributionOpen] = useState(false)
  const [selectedGoal, setSelectedGoal] = useState<SavingsGoal | null>(null)
  const [editGoal, setEditGoal] = useState<SavingsGoal | undefined>()

  const formatPEN = (cents: number) =>
    new Intl.NumberFormat('es-PE', { style: 'currency', currency: 'PEN' }).format(cents / 100)

  const handleDelete = async (id: number) => {
    if (!confirm('¿Eliminar esta meta?')) return
    try {
      await deleteGoal(id)
      toast.success('Meta eliminada')
      onRefresh()
    } catch {
      toast.error('Error al eliminar')
    }
  }

  const totalSaved = goals.reduce((sum, g) => sum + g.currentCents, 0)
  const totalTarget = goals.reduce((sum, g) => sum + g.targetCents, 0)

  return (
    <div className="space-y-4">
      <div className="hidden justify-end sm:flex">
        <Button
          onClick={() => {
            setEditGoal(undefined)
            setFormOpen(true)
          }}
        >
          <Plus size={14} />
          Nueva meta
        </Button>
      </div>

      <FloatingActionButton
        label="Nueva meta"
        onClick={() => {
          setEditGoal(undefined)
          setFormOpen(true)
        }}
      />

      {goals.length === 0 && (
        <UICard className="p-6 flex flex-col items-center gap-2 text-center text-muted-foreground">
          <PiggyBank className="w-10 h-10 opacity-30" />
          <p>No tienes metas de ahorro</p>
          <button
            onClick={() => {
              setEditGoal(undefined)
              setFormOpen(true)
            }}
            className="mt-1 text-sm text-primary hover:underline"
          >
            Crear primera meta
          </button>
        </UICard>
      )}

      {goals.length > 0 && (
        <UICard className="p-4 flex items-center justify-between gap-4 bg-muted/50">
          <div className="flex items-center gap-3">
            <TrendingUp className="w-5 h-5 text-muted-foreground" />
            <div>
              <p className="text-sm text-muted-foreground">Total ahorrado</p>
              <p className="text-xl font-bold">{formatPEN(totalSaved)}</p>
            </div>
          </div>
          <div className="text-right">
            <p className="text-sm text-muted-foreground">Meta total</p>
            <p className="text-lg font-semibold">{formatPEN(totalTarget)}</p>
          </div>
        </UICard>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {goals.map(goal => {
          const progress = Math.min((goal.currentCents / goal.targetCents) * 100, 100)
          const remaining = goal.targetCents - goal.currentCents

          return (
            <UICard
              key={goal.id}
              className="p-4 flex flex-col gap-3"
              style={{ borderTop: `4px solid ${goal.color}` }}
            >
              <div className="flex justify-between items-start">
                <div className="flex items-center gap-2">
                  <PiggyBank
                    className="w-4 h-4"
                    style={{ color: goal.color }}
                  />
                  <p className="font-semibold">{goal.name}</p>
                </div>
                <div className="flex gap-1">
                  <button
                    onClick={() => {
                      setEditGoal(goal)
                      setFormOpen(true)
                    }}
                    className="p-1 hover:bg-accent rounded"
                  >
                    <Pencil className="w-3 h-3 text-muted-foreground" />
                  </button>
                  <button
                    onClick={() => handleDelete(goal.id)}
                    className="p-1 hover:bg-accent rounded"
                  >
                    <Trash2 className="w-3 h-3 text-destructive" />
                  </button>
                </div>
              </div>

              <div className="space-y-1">
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>{formatPEN(goal.currentCents)}</span>
                  <span>{formatPEN(goal.targetCents)}</span>
                </div>
                <div className="h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full rounded-full transition-all duration-500"
                    style={{
                      width: `${progress}%`,
                      backgroundColor: goal.color,
                    }}
                  />
                </div>
                <p className="text-xs text-muted-foreground text-right">
                  {progress.toFixed(0)}% — falta {formatPEN(Math.max(0, remaining))}
                </p>
              </div>

              {goal.deadline && (
                <div className="flex items-center gap-1 text-xs text-muted-foreground">
                  <Calendar className="w-3 h-3" />
                  <span>Limite: {new Date(goal.deadline + 'T00:00:00').toLocaleDateString('es-PE')}</span>
                </div>
              )}

              <Button
                size="sm"
                variant="outline"
                className="w-full text-xs"
                onClick={() => {
                  setSelectedGoal(goal)
                  setContributionOpen(true)
                }}
              >
                <Plus className="w-3 h-3 mr-1" />
                Agregar ahorro
              </Button>
            </UICard>
          )
        })}
      </div>

      <GoalForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSuccess={() => {
          setFormOpen(false)
          onRefresh()
        }}
        editGoal={editGoal}
      />

      {selectedGoal && (
        <ContributionForm
          open={contributionOpen}
          onClose={() => setContributionOpen(false)}
          onSuccess={() => {
            setContributionOpen(false)
            onRefresh()
          }}
          goal={selectedGoal}
        />
      )}
    </div>
  )
}
