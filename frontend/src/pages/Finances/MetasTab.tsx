import { useState } from 'react'
import { toast } from 'sonner'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { PiggyBank, Plus, Trash2, Pencil, TrendingUp, Calendar, CreditCard, MoreVertical } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import type { SavingsGoal } from '@/types/finance.types'
import { goalsKey, cardsKey } from './financeKeys'
import { useOpenFormOnQueryParam } from './useOpenFormOnQueryParam'
import GoalForm from './GoalForm'
import ContributionForm from './ContributionForm'
import DeleteGoalDialog from './DeleteGoalDialog'

export default function MetasTab() {
  const [formOpen, setFormOpen] = useState(false)
  const [editGoal, setEditGoal] = useState<SavingsGoal | undefined>()
  const [contributionOpen, setContributionOpen] = useState(false)
  const [selectedGoal, setSelectedGoal] = useState<SavingsGoal | null>(null)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [goalToDelete, setGoalToDelete] = useState<SavingsGoal | null>(null)
  const { listGoals, deleteGoal, listCards } = useFinanceApi()
  const queryClient = useQueryClient()

  const { data: goals = [] } = useQuery({
    queryKey: goalsKey(),
    queryFn: () => listGoals(),
  })

  const { data: cards = [] } = useQuery({
    queryKey: cardsKey(),
    queryFn: () => listCards(),
  })

  useOpenFormOnQueryParam(() => {
    setEditGoal(undefined)
    setFormOpen(true)
  })

  const openDeleteDialog = (goal: SavingsGoal) => {
    setGoalToDelete(goal)
    setDeleteDialogOpen(true)
  }

  const handleConfirmDelete = async () => {
    if (!goalToDelete) return
    try {
      await deleteGoal(goalToDelete.id)
      toast.success('Meta eliminada')
      queryClient.invalidateQueries({ queryKey: goalsKey() })
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
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" aria-label={`Más acciones para ${goal.name}`}>
                          <MoreVertical size={14} />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={() => {
                            setEditGoal(goal)
                            setFormOpen(true)
                          }}
                        >
                          <Pencil className="h-4 w-4" />
                          Editar
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => openDeleteDialog(goal)}
                          className="text-destructive focus:text-destructive"
                        >
                          <Trash2 className="h-4 w-4" />
                          Eliminar
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
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

      <GoalForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={() => queryClient.invalidateQueries({ queryKey: goalsKey() })}
        editGoal={editGoal}
      />

      {selectedGoal && (
        <ContributionForm
          open={contributionOpen}
          onClose={() => setContributionOpen(false)}
          onSaved={() => {
            // A contribution both bumps goal.currentCents and debits the
            // goal's default card (it creates a transfer-kind Transaction
            // row, same as a card transfer) — invalidate all three.
            queryClient.invalidateQueries({ queryKey: goalsKey() })
            queryClient.invalidateQueries({ queryKey: cardsKey() })
            queryClient.invalidateQueries({ queryKey: ['finances', 'transactions'] })
          }}
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
