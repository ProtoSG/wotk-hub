import { useCallback, useEffect, useState } from 'react'
import { flushSync } from 'react-dom'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { useSearchParams } from 'react-router-dom'
import { PiggyBank, Plus, Trash2, Pencil, TrendingUp, Calendar, CreditCard } from 'lucide-react'
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
  DialogDescription,
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
import type { SavingsGoal, SavingsGoalInput, Card } from '@/types/finance.types'
import { GOAL_COLORS } from '@/types/finance.types'

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

function GoalForm({ open, onClose, onSaved, editGoal }: GoalFormProps) {
  const { createGoal, updateGoal, listCards } = useFinanceApi()
  const [cards, setCards] = useState<Card[]>([])
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
    listCards().then(setCards).catch(() => setCards([]))
  }, [])

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

interface ContributionFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  goal: SavingsGoal
}

const contributionSchema = z.object({
  amount: z.number().positive('Debe ser mayor a 0'),
  date: z.string().min(1, 'La fecha es requerida'),
})

type ContributionFormValues = z.infer<typeof contributionSchema>

function ContributionForm({ open, onClose, onSaved, goal }: ContributionFormProps) {
  const { createContribution } = useFinanceApi()
  const [saving, setSaving] = useState(false)

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ContributionFormValues>({
    resolver: zodResolver(contributionSchema),
    defaultValues: {
      amount: 0,
      date: new Date().toISOString().split('T')[0],
    },
  })

  useEffect(() => {
    if (open) reset({ amount: 0, date: new Date().toISOString().split('T')[0] })
  }, [open, reset])

  const onSubmit: SubmitHandler<ContributionFormValues> = async (values) => {
    setSaving(true)
    try {
      await createContribution(goal.id, {
        amountCents: Math.round(values.amount * 100),
        date: values.date,
        note: '',
      })
      toast.success('Ahorro registrado')
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Error al agregar')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Agregar ahorro a {goal.name}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Monto (PEN)</Label>
            <Input
              type="number"
              step="0.01"
              min="0.01"
              {...register('amount', { valueAsNumber: true })}
              placeholder="0.00"
            />
            {errors.amount && (
              <p className="text-xs text-destructive">{errors.amount.message}</p>
            )}
          </div>
          <div className="space-y-1">
            <Label>Fecha</Label>
            <Input type="date" {...register('date')} />
            {errors.date && (
              <p className="text-xs text-destructive">{errors.date.message}</p>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Registrando...' : 'Agregar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface DeleteGoalDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  goal: SavingsGoal | null
}

function DeleteGoalDialog({ open, onClose, onConfirm, goal }: DeleteGoalDialogProps) {
  const [loading, setLoading] = useState(false)

  const handleConfirm = async () => {
    setLoading(true)
    try {
      await onConfirm()
    } finally {
      setLoading(false)
      onClose()
    }
  }

  const showWarning = goal != null && goal.currentCents > 0 && goal.defaultCardId != null

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Eliminar meta</DialogTitle>
          <DialogDescription>Esta acción no se puede deshacer.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            ¿Estás seguro de que quieres eliminar la meta{' '}
            <span className="font-medium text-foreground">{goal?.name}</span>?
          </p>
          {showWarning && (
            <div className="flex items-start gap-2 rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
              <span className="text-base">⚠️</span>
              <span>
                Esta meta tiene <strong>{formatPEN(goal!.currentCents)}</strong> acumulados. Si la
                eliminas, <strong>NO</strong> se reintegrará a la tarjeta.
              </span>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            Cancelar
          </Button>
          <Button type="button" variant="destructive" onClick={handleConfirm} disabled={loading}>
            {loading ? 'Eliminando...' : 'Eliminar'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function MetasTab() {
  const [goals, setGoals] = useState<SavingsGoal[]>([])
  const [cards, setCards] = useState<Card[]>([])
  const [formOpen, setFormOpen] = useState(false)
  const [editGoal, setEditGoal] = useState<SavingsGoal | undefined>()
  const [contributionOpen, setContributionOpen] = useState(false)
  const [selectedGoal, setSelectedGoal] = useState<SavingsGoal | null>(null)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [goalToDelete, setGoalToDelete] = useState<SavingsGoal | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()
  const { listGoals, deleteGoal, listCards } = useFinanceApi()

  // Open form when navigated with ?new=1
  useEffect(() => {
    if (searchParams.get('new') === '1') {
      flushSync(() => {
        setEditGoal(undefined)
        setFormOpen(true)
        setSearchParams(
          (prev) => {
            const next = new URLSearchParams(prev)
            next.delete('new')
            return next
          },
          { replace: true }
        )
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- setSearchParams identity is stable, only react to searchParams changing
  }, [searchParams])

  const load = useCallback(async () => {
    try {
      const [goalsData, cardsData] = await Promise.all([listGoals(), listCards()])
      setGoals(goalsData)
      setCards(cardsData)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las metas')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

  const openDeleteDialog = (goal: SavingsGoal) => {
    setGoalToDelete(goal)
    setDeleteDialogOpen(true)
  }

  const handleConfirmDelete = async () => {
    if (!goalToDelete) return
    try {
      await deleteGoal(goalToDelete.id)
      toast.success('Meta eliminada')
      load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la meta')
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

      {goals.length === 0 ? (
        <CozyCard className="animate-card-in">
          <CardContent className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground">
            <PiggyBank className="h-10 w-10 opacity-30" />
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
          </CardContent>
        </CozyCard>
      ) : (
        <>
          <CozyCard className="animate-card-in bg-muted/50">
            <CardContent className="flex items-center justify-between gap-4 p-4">
              <div className="flex items-center gap-3">
                <TrendingUp className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="text-sm text-muted-foreground">Total ahorrado</p>
                  <p className="text-xl font-bold">{formatPEN(totalSaved)}</p>
                </div>
              </div>
              <div className="text-right">
                <p className="text-sm text-muted-foreground">Meta total</p>
                <p className="text-lg font-semibold">{formatPEN(totalTarget)}</p>
              </div>
            </CardContent>
          </CozyCard>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {goals.map((goal, i) => {
              const progress = Math.min((goal.currentCents / goal.targetCents) * 100, 100)
              const remaining = goal.targetCents - goal.currentCents
              // Every goal has a defaultCardId now — if it's missing from
              // cards, the card was archived after being assigned (Tarjetas
              // list only returns active cards), not that the goal has none.
              const card = cards.find((c) => c.id === goal.defaultCardId)

              return (
                <CozyCard
                  key={goal.id}
                  className="animate-card-in"
                  style={{ animationDelay: `${Math.min(i * 40, 320)}ms`, borderTop: `4px solid ${goal.color}` }}
                >
                  <CardHeader className="flex flex-row items-start justify-between pb-2">
                    <div className="flex items-center gap-2">
                      <PiggyBank className="h-4 w-4" style={{ color: goal.color }} />
                      <CardTitle className="text-sm font-medium">{goal.name}</CardTitle>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Editar meta ${goal.name}`}
                        onClick={() => {
                          setEditGoal(goal)
                          setFormOpen(true)
                        }}
                      >
                        <Pencil size={14} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Eliminar meta ${goal.name}`}
                        onClick={() => openDeleteDialog(goal)}
                      >
                        <Trash2 size={14} />
                      </Button>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="space-y-1">
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span>{formatPEN(goal.currentCents)}</span>
                        <span>{formatPEN(goal.targetCents)}</span>
                      </div>
                      <div className="h-2 overflow-hidden rounded-full bg-secondary">
                        <div
                          className="h-full rounded-full transition-all duration-500"
                          style={{
                            width: `${progress}%`,
                            backgroundColor: goal.color,
                            ...(progress >= 100
                              ? {
                                  backgroundImage:
                                    'repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.2)_4px,rgba(255,255,255,0.2)_8px)',
                                }
                              : {}),
                          }}
                        />
                      </div>
                      <p className="text-right text-xs text-muted-foreground">
                        {progress.toFixed(0)}% — falta {formatPEN(Math.max(0, remaining))}
                      </p>
                    </div>

                    {goal.deadline && (
                      <div className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Calendar className="h-3 w-3" />
                        <span>
                          Límite: {new Date(goal.deadline + 'T00:00:00').toLocaleDateString('es-PE')}
                        </span>
                      </div>
                    )}

                    {card ? (
                      <div className="flex items-center gap-1 text-xs">
                        <CreditCard className="h-3 w-3 text-muted-foreground" />
                        <span className="text-muted-foreground">Descontará de</span>
                        <span className="font-medium" style={{ color: card.color }}>
                          {card.name}
                        </span>
                      </div>
                    ) : (
                      <div className="flex items-center gap-1 text-xs text-amber-600 dark:text-amber-500">
                        <CreditCard className="h-3 w-3" />
                        <span>Tarjeta predeterminada eliminada — asigná una nueva</span>
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
                      <Plus className="mr-1 h-3 w-3" />
                      Agregar ahorro
                    </Button>
                  </CardContent>
                </CozyCard>
              )
            })}
          </div>
        </>
      )}

      <GoalForm open={formOpen} onClose={() => setFormOpen(false)} onSaved={load} editGoal={editGoal} />

      {selectedGoal && (
        <ContributionForm
          open={contributionOpen}
          onClose={() => setContributionOpen(false)}
          onSaved={load}
          goal={selectedGoal}
        />
      )}

      <DeleteGoalDialog
        open={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        onConfirm={handleConfirmDelete}
        goal={goalToDelete}
      />
    </div>
  )
}
