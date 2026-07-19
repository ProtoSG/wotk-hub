import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, SlidersHorizontal } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { useCategories } from '@/hooks/useCategories'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import {
  type Transaction,
  type TransactionType,
} from '@/types/finance.types'
import { transactionsKey, cardsKey } from './financeKeys'
import { useUndoableDelete } from './useUndoableDelete'
import { useOpenFormOnQueryParam } from './useOpenFormOnQueryParam'
import TransactionForm from './TransactionForm'
import TransactionsTable from './TransactionsTable'
import TransactionsMobileList from './TransactionsMobileList'
import CardCarousel from './CardCarousel'
import { CardTextureOverlay } from './cardVisuals'

const ALL = 'all'

interface Props {
  month: string
}

export default function MovimientosTab({ month }: Props) {
  const [typeFilter, setTypeFilter] = useState<string>(ALL)
  const [categoryFilter, setCategoryFilter] = useState<string>(ALL)
  const [cardFilter, setCardFilter] = useState<number | null>(null)
  const [filterSheetOpen, setFilterSheetOpen] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Transaction | null>(null)
  const { listTransactions, deleteTransaction, listCards, refundTransaction } = useFinanceApi()
  const { data: categoriesByKind } = useCategories()
  const queryClient = useQueryClient()

  const { data: transactions = [], isPending: isLoading } = useQuery({
    queryKey: transactionsKey(month, typeFilter, categoryFilter),
    queryFn: () =>
      listTransactions({
        month,
        ...(typeFilter !== ALL && { type: typeFilter as TransactionType }),
        ...(categoryFilter !== ALL && { category: categoryFilter }),
      }),
  })

  const { data: cards = [] } = useQuery({
    queryKey: cardsKey(),
    queryFn: () => listCards(),
  })

  // Build a name→label map from categories for display
  const categoryLabelMap = useMemo(() => {
    const map: Record<string, string> = {}
    for (const c of [...(categoriesByKind?.expense ?? []), ...(categoriesByKind?.income ?? [])]) {
      map[c.name] = c.label
    }
    return map
  }, [categoriesByKind])

  useOpenFormOnQueryParam(() => {
    setEditing(null)
    setFormOpen(true)
  })

  function invalidateAll() {
    queryClient.invalidateQueries({ queryKey: ['finances', 'transactions'] })
    queryClient.invalidateQueries({ queryKey: cardsKey() })
  }

  const { handleDelete } = useUndoableDelete<Transaction, number>({
    getId: (t) => t.id,
    deleteFn: deleteTransaction,
    removeFromCache: (t) => {
      let removedIndex = -1
      queryClient.setQueryData(transactionsKey(month, typeFilter, categoryFilter), (prev: Transaction[] = []) => {
        removedIndex = prev.findIndex((x) => x.id === t.id)
        return prev.filter((x) => x.id !== t.id)
      })
      return removedIndex
    },
    restoreToCache: (t, removedIndex) => {
      queryClient.setQueryData(transactionsKey(month, typeFilter, categoryFilter), (prev: Transaction[] = []) => {
        const next = [...prev]
        next.splice(Math.min(removedIndex, next.length), 0, t)
        return next
      })
    },
    successMessage: 'Movimiento eliminado',
    errorMessage: 'No se pudo eliminar el movimiento',
    onDeleteError: invalidateAll,
  })

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
      invalidateAll()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo marcar como reembolsado')
    } finally {
      setRefunding(false)
    }
  }

  const allCategoryOptions = [
    ...(categoriesByKind?.expense ?? []),
    ...(categoriesByKind?.income ?? []),
  ]

  const filteredTransactions = cardFilter == null
    ? transactions
    : transactions.filter((t) => t.cardId === cardFilter)

  return (
    <div className="space-y-4">
      <Sheet open={filterSheetOpen} onOpenChange={setFilterSheetOpen}>
        {/* Desktop-only: filters inline. Mobile puts the same filters behind
            the icon in CardCarousel's trailingAction below instead of a
            dedicated row, since they're rarely touched. */}
        <div className="hidden flex-wrap items-center gap-2 sm:flex">
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
              {allCategoryOptions.map((c) => (
                <SelectItem key={c.id} value={c.name}>
                  {c.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="ml-auto">
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

        <SheetContent side="bottom">
          <SheetHeader>
            <SheetTitle>Filtros</SheetTitle>
          </SheetHeader>
          <div className="space-y-3 pt-2">
            <Select value={typeFilter} onValueChange={setTypeFilter}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Todos</SelectItem>
                <SelectItem value="income">Ingresos</SelectItem>
                <SelectItem value="expense">Gastos</SelectItem>
              </SelectContent>
            </Select>
            <Select value={categoryFilter} onValueChange={setCategoryFilter}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Todas las categorías</SelectItem>
                {allCategoryOptions.map((c) => (
                  <SelectItem key={c.id} value={c.name}>
                    {c.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </SheetContent>

        {/* Horizontal scrollable card filter — desktop only, mobile uses the swipeable carousel below */}
        <div className="hidden gap-3 overflow-x-auto snap-x snap-mandatory px-1 sm:flex" style={{ scrollbarWidth: 'none' }}>
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
                'relative min-w-[180px] h-[100px] rounded-xl shadow-md p-4 flex flex-col justify-between overflow-hidden transition-all snap-start shrink-0 border-2',
                cardFilter === card.id
                  ? 'border-white/80 ring-2 ring-primary/50'
                  : 'border-transparent'
              )}
              style={{ backgroundColor: card.color }}
            >
              <CardTextureOverlay />
              <div className="flex justify-between items-start">
                <span className="text-white/90 text-sm font-medium truncate">{card.name}</span>
                <span className="text-white/70 text-xs">•••• {card.last4}</span>
              </div>
              <span className="text-white/60 text-xs">{formatPEN(card.balanceCents)}</span>
            </button>
          ))}
        </div>

        {/* Swipeable card carousel — mobile only. Its "Todos" slide opens
            the filter sheet below (tap target instead of a separate icon). */}
        <CardCarousel
          cards={cards}
          transactionsCount={transactions.length}
          onCardChange={setCardFilter}
          onOpenFilters={() => setFilterSheetOpen(true)}
        />
      </Sheet>

      <TransactionsTable
        transactions={filteredTransactions}
        isLoading={isLoading}
        cards={cards}
        categoryLabelMap={categoryLabelMap}
        onEdit={(t) => {
          setEditing(t)
          setFormOpen(true)
        }}
        onDelete={handleDelete}
        onRefund={handleRefund}
        onNewTransaction={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

      <TransactionsMobileList
        transactions={filteredTransactions}
        isLoading={isLoading}
        categoryLabelMap={categoryLabelMap}
        onEdit={(t) => {
          setEditing(t)
          setFormOpen(true)
        }}
        onDelete={handleDelete}
        onRefund={handleRefund}
        onNewTransaction={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

      <TransactionForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={invalidateAll}
        editing={editing}
        defaultCardId={cardFilter}
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
