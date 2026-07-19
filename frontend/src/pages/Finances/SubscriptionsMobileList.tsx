import { MoreVertical, Pencil, Trash2, Repeat, Power } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { Button } from '@/components/ui/button'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import { FREQUENCY_LABELS, type Subscription } from '@/types/finance.types'

interface Props {
  subscriptions: Subscription[]
  onEdit: (s: Subscription) => void
  onDelete: (s: Subscription) => void
  onToggleActive: (s: Subscription, active: boolean) => void
  onNewSubscription: () => void
}

export default function SubscriptionsMobileList({
  subscriptions,
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
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Repeat className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium">{s.name}</div>
                <div className="truncate text-xs text-muted-foreground">
                  {s.category} · {FREQUENCY_LABELS[s.frequency]}
                </div>
                <div className="truncate text-xs text-muted-foreground">
                  Próximo cobro: {s.nextBillingOn}
                </div>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1.5">
                <span className="text-sm font-semibold">{formatPEN(s.amountCents)}</span>
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" className="shrink-0" aria-label="Más acciones">
                    <MoreVertical className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => onEdit(s)}>
                    <Pencil className="h-4 w-4" />
                    Editar
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => onToggleActive(s, !s.active)}>
                    <Power className="h-4 w-4" />
                    {s.active ? 'Desactivar' : 'Activar'}
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
