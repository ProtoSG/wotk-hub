import { ArrowDownRight, ArrowUpRight, Calendar, MoreVertical, Pencil, Repeat, Trash2 } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { Button } from '@/components/ui/button'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { IconChip } from '@/components/ui/icon-chip'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import { FREQUENCY_LABELS, type Subscription } from '@/types/finance.types'

interface Props {
  subscriptions: Subscription[]
  categoryLabelMap: Record<string, string>
  onEdit: (s: Subscription) => void
  onDelete: (s: Subscription) => void
  onToggleActive: (s: Subscription, active: boolean) => void
  onNewSubscription: () => void
}

// `nextBillingOn` is a date-only string ("2026-08-15"). Parsed alone, the
// Date constructor treats it as UTC midnight, which shifts to the previous
// day in negative-UTC-offset timezones (e.g. Peru, UTC-5) — appending a
// local midnight time avoids that off-by-one.
function formatShortLocalDate(dateOnly: string): string {
  const date = new Date(`${dateOnly}T00:00:00`)
  if (Number.isNaN(date.getTime())) return dateOnly
  return date.toLocaleDateString('es-PE', { day: 'numeric', month: 'short' })
}

export default function SubscriptionsMobileList({
  subscriptions,
  categoryLabelMap,
  onEdit,
  onDelete,
  onToggleActive,
  onNewSubscription,
}: Props) {
  return (
    <CozyCard className="animate-card-in [animation-delay:60ms] sm:hidden">
      <CardContent className="p-0">
        {subscriptions.length === 0 ? (
          <EmptyState
            icon={<Repeat className="h-8 w-8" />}
            title="Sin suscripciones registradas"
            description="Agrega tus suscripciones para hacer seguimiento."
            action={{ label: 'Agregar suscripción', onClick: onNewSubscription }}
          />
        ) : (
          subscriptions.map((s) => (
            <div
              key={s.id}
              className={cn(
                'flex items-center gap-3 border-b p-4 last:border-0',
                !s.active && 'opacity-50'
              )}
            >
              <IconChip
                icon={s.type === 'income' ? ArrowUpRight : ArrowDownRight}
                accent={s.type === 'income' ? '--income' : '--expense'}
                size="sm"
                className="shrink-0"
              />
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium">{s.name}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {categoryLabelMap[s.category] ?? s.category} · {FREQUENCY_LABELS[s.frequency]}
                </div>
                <div className="flex items-center gap-1 truncate text-xs text-muted-foreground">
                  <Calendar className="h-3 w-3 shrink-0" />
                  {formatShortLocalDate(s.nextBillingOn)}
                </div>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1.5">
                <span
                  className={`text-sm font-semibold ${s.type === 'income' ? 'text-income' : 'text-expense'}`}
                >
                  {s.type === 'income' ? '+' : '-'}
                  {formatPEN(s.amountCents)}
                </span>
                {/* Visible switch, same as desktop's table — toggling active
                    shouldn't cost an extra tap into the overflow menu just
                    because the screen is narrower. */}
                <Switch
                  checked={s.active}
                  onCheckedChange={(v) => onToggleActive(s, v)}
                  aria-label={`${s.active ? 'Desactivar' : 'Activar'} ${s.name}`}
                />
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" className="shrink-0" aria-label="Más acciones">
                    <MoreVertical className="h-4 w-4 rotate-90" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => onEdit(s)}>
                    <Pencil className="h-4 w-4" />
                    Editar
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => onDelete(s)}
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
  )
}
