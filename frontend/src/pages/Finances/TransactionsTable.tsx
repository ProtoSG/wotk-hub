import { ArrowLeftRight, MoreVertical, Pencil, RotateCcw, Trash2 } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { formatPEN } from '@/lib/currency'
import type { Card, Transaction } from '@/types/finance.types'

interface Props {
  transactions: Transaction[]
  isLoading: boolean
  cards: Card[]
  categoryLabelMap: Record<string, string>
  onEdit: (t: Transaction) => void
  onDelete: (t: Transaction) => void
  onRefund: (t: Transaction) => void
  onNewTransaction: () => void
}

export default function TransactionsTable({
  transactions,
  isLoading,
  cards,
  categoryLabelMap,
  onEdit,
  onDelete,
  onRefund,
  onNewTransaction,
}: Props) {
  return (
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
            ) : transactions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7}>
                  <EmptyState
                    icon={<ArrowLeftRight className="h-8 w-8" />}
                    title="Sin movimientos este mes"
                    description="Registrá tu primer movimiento para empezar a controlar tus finanzas."
                    action={{ label: 'Nuevo movimiento', onClick: onNewTransaction }}
                  />
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
                  <TableCell>{categoryLabelMap[t.category] ?? t.category}</TableCell>
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
                    <div className="flex justify-end">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" aria-label="Más acciones">
                            <MoreVertical className="h-3.5 w-3.5 rotate-90" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={() => onEdit(t)}>
                            <Pencil className="h-4 w-4" />
                            Editar
                          </DropdownMenuItem>
                          {t.type === 'expense' && (
                            <DropdownMenuItem onClick={() => onRefund(t)}>
                              <RotateCcw className="h-4 w-4" />
                              Marcar como reembolsado
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem onClick={() => onDelete(t)} className="text-destructive focus:text-destructive">
                            <Trash2 className="h-4 w-4" />
                            Eliminar
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>
    </CozyCard>
  )
}
