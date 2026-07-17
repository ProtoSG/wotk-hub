import { useCallback, useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { toast } from 'sonner'
import { useSearchParams } from 'react-router-dom'
import { Plus, Pencil, Trash2, ArrowUpRight, ArrowDownRight, MoreVertical, RotateCcw, SlidersHorizontal, ArrowLeftRight } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import {
  EXPENSE_CATEGORIES,
  INCOME_CATEGORIES,
  CATEGORY_LABELS,
  type Transaction,
  type TransactionType,
  type Card,
} from '@/types/finance.types'
import TransactionForm from './TransactionForm'

const ALL = 'all'
const UNDO_WINDOW_MS = 4500

interface Props {
  month: string
}

export default function MovimientosTab({ month }: Props) {
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [cards, setCards] = useState<Card[]>([])
  const [typeFilter, setTypeFilter] = useState<string>(ALL)
  const [categoryFilter, setCategoryFilter] = useState<string>(ALL)
  const [cardFilter, setCardFilter] = useState<number | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Transaction | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [searchParams, setSearchParams] = useSearchParams()
  const { listTransactions, deleteTransaction, listCards, refundTransaction } = useFinanceApi()
  const pendingDeletes = useRef(new Map<number, number>())

  // Open form when navigated with ?new=1
  useEffect(() => {
    if (searchParams.get('new') === '1') {
      flushSync(() => {
        setEditing(null)
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
    setIsLoading(true)
    try {
      const [data, cardData] = await Promise.all([
        listTransactions({
          month,
          ...(typeFilter !== ALL && { type: typeFilter as TransactionType }),
          ...(categoryFilter !== ALL && { category: categoryFilter }),
        }),
        listCards(),
      ])
      setTransactions(data)
      setCards(cardData)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar los movimientos')
    } finally {
      setIsLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [month, typeFilter, categoryFilter])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

  async function commitDelete(id: number) {
    pendingDeletes.current.delete(id)
    try {
      await deleteTransaction(id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el movimiento')
      load()
    }
  }

  function handleDelete(t: Transaction) {
    let removedIndex = -1
    setTransactions((prev) => {
      removedIndex = prev.findIndex((x) => x.id === t.id)
      return prev.filter((x) => x.id !== t.id)
    })

    const timer = window.setTimeout(() => commitDelete(t.id), UNDO_WINDOW_MS)
    pendingDeletes.current.set(t.id, timer)

    toast.success('Movimiento eliminado', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(t.id)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(t.id)
          }
          setTransactions((prev) => {
            const next = [...prev]
            next.splice(Math.min(removedIndex, next.length), 0, t)
            return next
          })
        },
      },
    })
  }

  const [refundTarget, setRefundTarget] = useState<Transaction | null>(null)
  const [refundDialogOpen, setRefundDialogOpen] = useState(false)
  const [refunding, setRefunding] = useState(false)

  function handleRefund(t: Transaction) {
    setRefundTarget(t)
    setRefundDialogOpen(true)
  }

  async function confirmRefund() {
    if (!refundTarget) return
    setRefunding(true)
    try {
      await refundTransaction(refundTarget.id)
      toast.success('Gasto marcado como reembolsado')
      setRefundDialogOpen(false)
      setRefundTarget(null)
      load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo marcar como reembolsado')
    } finally {
      setRefunding(false)
    }
  }

  const allCategories = [...new Set([...EXPENSE_CATEGORIES, ...INCOME_CATEGORIES])]

  const filteredTransactions = cardFilter == null
    ? transactions
    : transactions.filter((t) => t.cardId === cardFilter)

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={typeFilter} onValueChange={setTypeFilter}>
          <SelectTrigger className="w-36">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Todos</SelectItem>
            <SelectItem value="income">Ingresos</SelectItem>
            <SelectItem value="expense">Gastos</SelectItem>
          </SelectContent>
        </Select>
        <Select value={categoryFilter} onValueChange={setCategoryFilter}>
          <SelectTrigger className="w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>Todas las categorías</SelectItem>
            {allCategories.map((c) => (
              <SelectItem key={c} value={c}>
                {CATEGORY_LABELS[c] ?? c}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="ml-auto hidden sm:block">
          <Button
            onClick={() => {
              setEditing(null)
              setFormOpen(true)
            }}
          >
            <Plus size={14} />
            Nuevo movimiento
          </Button>
        </div>
      </div>

      {/* Horizontal scrollable card filter */}
      <div className="flex gap-3 overflow-x-auto snap-x snap-mandatory px-1" style={{ scrollbarWidth: 'none' }}>
        {/* "Todos" card */}
        <button
          onClick={() => setCardFilter(null)}
          className={cn(
            'min-w-[180px] h-[100px] rounded-xl shadow-sm p-4 flex flex-col justify-between border-2 transition-all snap-start shrink-0',
            cardFilter === null
              ? 'border-primary bg-muted/50'
              : 'border-dashed border-muted-foreground/30 bg-muted/30 hover:bg-muted/50'
          )}
        >
          <div className="flex justify-between items-start">
            <span className="text-foreground/90 text-sm font-medium truncate">Todos</span>
            <SlidersHorizontal className="text-foreground/50" size={14} />
          </div>
          <span className="text-foreground/40 text-xs">{transactions.length} movimientos</span>
        </button>

        {/* Card filters */}
        {cards.map((card) => (
          <button
            key={card.id}
            onClick={() => setCardFilter(card.id)}
            className={cn(
              'min-w-[180px] h-[100px] rounded-xl shadow-md p-4 flex flex-col justify-between transition-all snap-start shrink-0 border-2',
              cardFilter === card.id
                ? 'border-white/80 ring-2 ring-primary/50'
                : 'border-transparent'
            )}
            style={{ backgroundColor: card.color }}
          >
            <div className="flex justify-between items-start">
              <span className="text-white/90 text-sm font-medium truncate">{card.name}</span>
              <span className="text-white/70 text-xs">•••• {card.last4}</span>
            </div>
            <span className="text-white/60 text-xs">S/ {(card.balanceCents / 100).toFixed(2)}</span>
          </button>
        ))}
      </div>

      <CozyCard className="animate-card-in hidden sm:block">
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Fecha</TableHead>
                <TableHead>Tipo</TableHead>
                <TableHead>Categoría</TableHead>
                <TableHead>Descripción</TableHead>
                <TableHead>Tarjeta</TableHead>
                <TableHead className="text-right">Monto</TableHead>
                <TableHead className="w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <>
                  {Array.from({ length: 6 }).map((_, i) => (
                    <TableRow key={i}>
                      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-5 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                      <TableCell><Skeleton className="h-5 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-8 w-12" /></TableCell>
                    </TableRow>
                  ))}
                </>
              ) : filteredTransactions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7}>
                    <EmptyState
                      icon={<ArrowLeftRight className="h-8 w-8" />}
                      title="Sin movimientos este mes"
                      description="Registrá tu primer movimiento para empezar a controlar tus finanzas."
                      action={{
                        label: 'Nuevo movimiento',
                        onClick: () => {
                          setEditing(null)
                          setFormOpen(true)
                        },
                      }}
                    />
                  </TableCell>
                </TableRow>
              ) : (
                filteredTransactions.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="whitespace-nowrap">{t.date}</TableCell>
                    <TableCell>
                      <Badge variant={t.type === 'income' ? 'default' : 'secondary'}>
                        {t.type === 'income' ? 'Ingreso' : 'Gasto'}
                      </Badge>
                    </TableCell>
                    <TableCell>{CATEGORY_LABELS[t.category] ?? t.category}</TableCell>
                    <TableCell className="max-w-64 truncate text-muted-foreground">
                      {t.description || '—'}
                    </TableCell>
                    <TableCell>
                      {t.cardId != null ? (
                        (() => {
                          const card = cards.find((c) => c.id === t.cardId)
                          return card ? (
                            <Badge
                              variant="outline"
                              style={{ borderColor: card.color, color: card.color }}
                              className="text-xs"
                            >
                              {card.name} ({card.last4})
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="text-xs">
                              #{t.cardId}
                            </Badge>
                          )
                        })()
                      ) : (
                        <span className="text-xs text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell
                      className={`text-right font-medium ${
                        t.type === 'income' ? 'text-income' : 'text-expense'
                      }`}
                    >
                      {t.type === 'income' ? '+' : '-'}
                      {formatPEN(t.amountCents)}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Editar movimiento"
                          onClick={() => {
                            setEditing(t)
                            setFormOpen(true)
                          }}
                        >
                          <Pencil size={14} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Eliminar movimiento"
                          onClick={() => handleDelete(t)}
                        >
                          <Trash2 size={14} />
                        </Button>
                        {t.type === 'expense' && (
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Marcar como reembolsado"
                            onClick={() => handleRefund(t)}
                          >
                            <RotateCcw size={14} />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </CozyCard>

      <CozyCard className="animate-card-in sm:hidden">
        <CardContent className="p-0">
          {isLoading ? (
            <div className="p-4 space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3">
                  <Skeleton className="h-10 w-10 rounded-full shrink-0" />
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-3 w-48" />
                  </div>
                  <Skeleton className="h-4 w-16" />
                </div>
              ))}
            </div>
          ) : filteredTransactions.length === 0 ? (
            <EmptyState
              icon={<ArrowLeftRight className="h-8 w-8" />}
              title="Sin movimientos este mes"
              description="Registrá tu primer movimiento."
              action={{
                label: 'Nuevo movimiento',
                onClick: () => {
                  setEditing(null)
                  setFormOpen(true)
                },
              }}
            />
          ) : (
            filteredTransactions.map((t) => (
              <div key={t.id} className="flex items-center gap-3 border-b p-4 last:border-0">
                <div
                  className={cn(
                    'flex h-10 w-10 shrink-0 items-center justify-center rounded-full',
                    t.type === 'income'
                      ? 'bg-income/10 text-income'
                      : 'bg-expense/10 text-expense'
                  )}
                >
                  {t.type === 'income' ? (
                    <ArrowUpRight className="h-5 w-5" />
                  ) : (
                    <ArrowDownRight className="h-5 w-5" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium">
                    {CATEGORY_LABELS[t.category] ?? t.category}
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    {t.description ? `${t.description} · ${t.date}` : t.date}
                  </div>
                </div>
                <div
                  className={cn(
                    'shrink-0 text-sm font-semibold',
                    t.type === 'income' ? 'text-income' : 'text-expense'
                  )}
                >
                  {t.type === 'income' ? '+' : '-'}
                  {formatPEN(t.amountCents)}
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="shrink-0" aria-label="Más acciones">
                      <MoreVertical className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      onClick={() => {
                        setEditing(t)
                        setFormOpen(true)
                      }}
                    >
                      <Pencil className="h-4 w-4" />
                      Editar
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => handleDelete(t)}
                      className="text-destructive focus:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                      Eliminar
                    </DropdownMenuItem>
                    {t.type === 'expense' && (
                      <DropdownMenuItem onClick={() => handleRefund(t)}>
                        <RotateCcw className="h-4 w-4" />
                        Marcar como reembolsado
                      </DropdownMenuItem>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            ))
          )}
        </CardContent>
      </CozyCard>

      <TransactionForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={load}
        editing={editing}
      />

      <Dialog open={refundDialogOpen} onOpenChange={(v) => !v && setRefundDialogOpen(false)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Marcar como reembolsado</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            ¿Marcar este gasto como reembolsado? Se creará una transacción de ingreso por{' '}
            <strong>{refundTarget ? formatPEN(refundTarget.amountCents) : ''}</strong> en la misma
            tarjeta.
          </p>
          <p className="text-xs text-muted-foreground">
            El reembolso sí repondrá el saldo de la tarjeta.
          </p>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setRefundDialogOpen(false)}>
              Cancelar
            </Button>
            <Button onClick={confirmRefund} disabled={refunding}>
              {refunding ? 'Reembolsando…' : 'Confirmar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
