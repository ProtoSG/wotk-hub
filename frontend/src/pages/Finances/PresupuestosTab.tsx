import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, MoreVertical, Pencil, Plus, Target, Trash2 } from 'lucide-react'
import { getCategoryAccent, getCategoryIcon } from './categoryIcons'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { IconChip } from '@/components/ui/icon-chip'
import { Progress } from '@/components/ui/progress'
import { EmptyState } from '@/components/ui/empty-state'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import { type Budget } from '@/types/finance.types'
import { budgetsKey } from './financeKeys'
import { useUndoableDelete } from './useUndoableDelete'
import { useOpenFormOnQueryParam } from './useOpenFormOnQueryParam'
import { getBudgetStatus } from './budgetStatus'
import BudgetForm from './BudgetForm'

interface Props {
  month: string
}

export default function PresupuestosTab({ month }: Props) {
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Budget | null>(null)
  const { listBudgets, deleteBudget } = useFinanceApi()
  const queryClient = useQueryClient()

  const { data: budgets = [] } = useQuery({
    queryKey: budgetsKey(month),
    queryFn: () => listBudgets(month),
  })

  useOpenFormOnQueryParam(() => {
    setEditing(null)
    setFormOpen(true)
  })

  const { handleDelete } = useUndoableDelete<Budget, string>({
    getId: (b) => b.category,
    deleteFn: deleteBudget,
    removeFromCache: (b) => {
      let removedIndex = -1
      queryClient.setQueryData(budgetsKey(month), (prev: Budget[] = []) => {
        removedIndex = prev.findIndex((x) => x.category === b.category)
        return prev.filter((x) => x.category !== b.category)
      })
      return removedIndex
    },
    restoreToCache: (b, removedIndex) => {
      queryClient.setQueryData(budgetsKey(month), (prev: Budget[] = []) => {
        const next = [...prev]
        next.splice(Math.min(removedIndex, next.length), 0, b)
        return next
      })
    },
    successMessage: 'Presupuesto eliminado',
    errorMessage: 'No se pudo eliminar el presupuesto',
    onDeleteError: () => queryClient.invalidateQueries({ queryKey: budgetsKey(month) }),
  })

  return (
    <div className="space-y-4">
      <div className="hidden justify-end sm:flex">
        <Button
          onClick={() => {
            setEditing(null)
            setFormOpen(true)
          }}
        >
          <Plus className="h-4 w-4" />
          Nuevo presupuesto
        </Button>
      </div>

      {budgets.length === 0 ? (
        <CozyCard className="animate-card-in">
          <CardContent>
            <EmptyState
              icon={<Target className="h-8 w-8" />}
              title="Sin presupuestos definidos"
              description="Crea uno para controlar tus gastos por categoría."
              action={{
                label: 'Crear presupuesto',
                onClick: () => {
                  setEditing(null)
                  setFormOpen(true)
                },
              }}
            />
          </CardContent>
        </CozyCard>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {budgets.map((b, i) => {
            const { pct, over, isDanger, indicatorColor, accent, stripeStyle } = getBudgetStatus(b)
            // Category color normally (matches Citas' per-category chips —
            // not every budget should look the same); status color takes
            // over once it's actually a warning, so the danger signal still
            // reads at a glance.
            const chipAccent = isDanger ? accent : getCategoryAccent(b.category)
            return (
              <CozyCard
                key={b.id}
                className="animate-card-in"
                style={{ animationDelay: `${Math.min(i * 40, 320)}ms` }}
              >
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <div className="flex items-center gap-2">
                    <IconChip icon={getCategoryIcon(b.category)} accent={chipAccent} size="sm" />
                    <CardTitle className="text-sm font-medium">
                      {b.category}
                    </CardTitle>
                  </div>
                  <div className="flex items-center gap-1">
                    {over && (
                      <Badge variant="destructive" className="gap-1">
                        <AlertTriangle className="h-3 w-3" />
                        Excedido
                      </Badge>
                    )}
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" aria-label={`Más acciones para ${b.category}`}>
                          <MoreVertical className="h-3.5 w-3.5 rotate-90" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={() => {
                            setEditing(b)
                            setFormOpen(true)
                          }}
                        >
                          <Pencil className="h-4 w-4" />
                          Editar
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => handleDelete(b)}
                          className="text-destructive focus:text-destructive"
                        >
                          <Trash2 className="h-4 w-4" />
                          Eliminar
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </CardHeader>
                <CardContent className="space-y-2">
                  <div className="flex items-baseline justify-between">
                    <span className={`text-xl font-bold ${over ? 'text-destructive' : ''}`}>
                      {formatPEN(b.spentCents)}
                    </span>
                    <span className="text-sm text-muted-foreground">
                      de {formatPEN(b.monthlyLimitCents)}
                    </span>
                  </div>
                  <Progress value={pct} indicatorClassName={indicatorColor} indicatorStyle={stripeStyle} />
                  <p className="text-xs text-muted-foreground">
                    {over
                      ? `${formatPEN(b.spentCents - b.monthlyLimitCents)} por encima del límite`
                      : `Queda ${formatPEN(b.monthlyLimitCents - b.spentCents)}`}
                  </p>
                </CardContent>
              </CozyCard>
            )
          })}
        </div>
      )}

      <BudgetForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={() => queryClient.invalidateQueries({ queryKey: budgetsKey(month) })}
        editing={editing}
        usedCategories={budgets.map((b) => b.category)}
      />
    </div>
  )
}
