import { Pencil, Trash2, Repeat } from 'lucide-react'
import { EmptyState } from '@/components/ui/empty-state'
import { Button } from '@/components/ui/button'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatPEN } from '@/lib/currency'
import { FREQUENCY_LABELS, type Subscription } from '@/types/finance.types'

interface Props {
  subscriptions: Subscription[]
  onEdit: (s: Subscription) => void
  onDelete: (s: Subscription) => void
  onToggleActive: (s: Subscription, active: boolean) => void
  onNewSubscription: () => void
}

export default function SubscriptionsTable({
  subscriptions,
  onEdit,
  onDelete,
  onToggleActive,
  onNewSubscription,
}: Props) {
  return (
    <CozyCard className="animate-card-in [animation-delay:60ms] hidden sm:block">
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Nombre</TableHead>
              <TableHead>Categoría</TableHead>
              <TableHead>Frecuencia</TableHead>
              <TableHead>Próximo cobro</TableHead>
              <TableHead className="text-right">Monto</TableHead>
              <TableHead>Activa</TableHead>
              <TableHead className="w-20" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {subscriptions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7}>
                  <EmptyState
                    icon={<Repeat className="h-8 w-8" />}
                    title="Sin suscripciones registradas"
                    description="Agrega tus suscripciones para hacer seguimiento de tus gastos recurrentes."
                    action={{ label: 'Agregar suscripción', onClick: onNewSubscription }}
                  />
                </TableCell>
              </TableRow>
            ) : (
              subscriptions.map((s) => (
                <TableRow key={s.id} className={s.active ? '' : 'opacity-50'}>
                  <TableCell className="font-medium">{s.name}</TableCell>
                  <TableCell>{s.category}</TableCell>
                  <TableCell>{FREQUENCY_LABELS[s.frequency]}</TableCell>
                  <TableCell className="whitespace-nowrap">{s.nextBillingOn}</TableCell>
                  <TableCell className="text-right font-medium">{formatPEN(s.amountCents)}</TableCell>
                  <TableCell>
                    <Switch checked={s.active} onCheckedChange={(v) => onToggleActive(s, v)} />
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Editar suscripción ${s.name}`}
                        onClick={() => onEdit(s)}
                      >
                        <Pencil size={14} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={`Eliminar suscripción ${s.name}`}
                        onClick={() => onDelete(s)}
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
  )
}
