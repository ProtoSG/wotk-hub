import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useSearchParams } from 'react-router-dom'
import { Plus, Pencil, Trash2, AlertTriangle, Target } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Progress } from '@/components/ui/progress'
import { EmptyState } from '@/components/ui/empty-state'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import { CATEGORY_LABELS, type Budget } from '@/types/finance.types'
import BudgetForm from './BudgetForm'

interface Props {
  month: string
}

const UNDO_WINDOW_MS = 4500

export default function PresupuestosTab({ month }: Props) {
  const [budgets, setBudgets] = useState<Budget[]>([])
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Budget | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()
  const { listBudgets, deleteBudget } = useFinanceApi()
  const pendingDeletes = useRef(new Map<string, number>())

  // Open form when navigated with ?new=1
  useEffect(() => {
    if (searchParams.get('new') === '1') {
      setEditing(null)
      setFormOpen(true)
      setSearchParams({}, { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const load = useCallback(async () => {
    try {
      setBudgets(await listBudgets(month))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar los presupuestos')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [month])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

  async function commitDelete(category: string) {
    pendingDeletes.current.delete(category)
    try {
      await deleteBudget(category)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el presupuesto')
      load()
    }
  }

  function handleDelete(b: Budget) {
    let removedIndex = -1
    setBudgets((prev) => {
      removedIndex = prev.findIndex((x) => x.category === b.category)
      return prev.filter((x) => x.category !== b.category)
    })

    const timer = window.setTimeout(() => commitDelete(b.category), UNDO_WINDOW_MS)
    pendingDeletes.current.set(b.category, timer)

    toast.success('Presupuesto eliminado', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(b.category)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(b.category)
          }
          setBudgets((prev) => {
            const next = [...prev]
            next.splice(Math.min(removedIndex, next.length), 0, b)
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
            setEditing(null)
            setFormOpen(true)
          }}
        >
          <Plus size={14} />
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
            const pct = b.monthlyLimitCents > 0 ? (b.spentCents / b.monthlyLimitCents) * 100 : 0
            const over = b.spentCents > b.monthlyLimitCents
            const isDanger = over || pct >= 80
            const indicatorColor = over ? 'bg-destructive' : pct >= 80 ? 'bg-amber-500' : 'bg-primary'
            const stripeStyle =
              isDanger
                ? {
                    backgroundImage:
                      'repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.2)_4px,rgba(255,255,255,0.2)_8px)',
                  }
                : {}
            return (
              <CozyCard
                key={b.id}
                className="animate-card-in"
                style={{ animationDelay: `${Math.min(i * 40, 320)}ms` }}
              >
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium">
                    {CATEGORY_LABELS[b.category] ?? b.category}
                  </CardTitle>
                  <div className="flex items-center gap-1">
                    {over && (
                      <Badge variant="destructive" className="gap-1">
                        <AlertTriangle size={12} />
                        Excedido
                      </Badge>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Editar presupuesto de ${CATEGORY_LABELS[b.category] ?? b.category}`}
                      onClick={() => {
                        setEditing(b)
                        setFormOpen(true)
                      }}
                    >
                      <Pencil size={14} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Eliminar presupuesto de ${CATEGORY_LABELS[b.category] ?? b.category}`}
                      onClick={() => handleDelete(b)}
                    >
                      <Trash2 size={14} />
                    </Button>
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
        onSaved={load}
        editing={editing}
        usedCategories={budgets.map((b) => b.category)}
      />
    </div>
  )
}
