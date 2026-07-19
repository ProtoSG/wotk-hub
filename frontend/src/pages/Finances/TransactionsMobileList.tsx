import { ArrowUpRight, ArrowDownRight, MoreVertical, Pencil, Trash2, RotateCcw, ArrowLeftRight } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { Button } from '@/components/ui/button'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import type { Transaction } from '@/types/finance.types'

interface Props {
  transactions: Transaction[]
  isLoading: boolean
  categoryLabelMap: Record<string, string>
  onEdit: (t: Transaction) => void
  onDelete: (t: Transaction) => void
  onRefund: (t: Transaction) => void
  onNewTransaction: () => void
}

export default function TransactionsMobileList({
  transactions,
  isLoading,
  categoryLabelMap,
  onEdit,
  onDelete,
  onRefund,
  onNewTransaction,
}: Props) {
  return (
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
        ) : transactions.length === 0 ? (
          <EmptyState
            icon={<ArrowLeftRight className="h-8 w-8" />}
            title="Sin movimientos este mes"
            description="Registrá tu primer movimiento."
            action={{ label: 'Nuevo movimiento', onClick: onNewTransaction }}
          />
        ) : (
          transactions.map((t) => (
            <div key={t.id} className="flex items-center gap-3 border-b p-4 last:border-0">
              <div
                className={cn(
                  'flex h-10 w-10 shrink-0 items-center justify-center rounded-full',
                  t.type === 'income' ? 'bg-income/10 text-income' : 'bg-expense/10 text-expense'
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
                  {categoryLabelMap[t.category] ?? t.category}
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
                  <DropdownMenuItem onClick={() => onEdit(t)}>
                    <Pencil className="h-4 w-4" />
                    Editar
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => onDelete(t)}
                    className="text-destructive focus:text-destructive"
                  >
                    <Trash2 className="h-4 w-4" />
                    Eliminar
                  </DropdownMenuItem>
                  {t.type === 'expense' && (
                    <DropdownMenuItem onClick={() => onRefund(t)}>
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
  )
}
