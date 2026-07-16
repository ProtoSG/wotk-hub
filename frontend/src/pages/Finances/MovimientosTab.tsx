import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2, ArrowUpRight, ArrowDownRight, MoreVertical } from 'lucide-react'
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
import FloatingActionButton from './FloatingActionButton'

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
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Transaction | null>(null)
  const { listTransactions, deleteTransaction, listCards } = useFinanceApi()
  const pendingDeletes = useRef(new Map<number, number>())

  const load = useCallback(async () => {
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

  const allCategories = [...new Set([...EXPENSE_CATEGORIES, ...INCOME_CATEGORIES])]

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

      <FloatingActionButton
        label="Nuevo movimiento"
        onClick={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

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
              {transactions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                    Sin movimientos este mes
                  </TableCell>
                </TableRow>
              ) : (
                transactions.map((t) => (
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
          {transactions.length === 0 ? (
            <div className="py-8 text-center text-sm text-muted-foreground">
              Sin movimientos este mes
            </div>
          ) : (
            transactions.map((t) => (
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
    </div>
  )
}
