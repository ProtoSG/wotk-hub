import { useState } from 'react'
import { toast } from 'sonner'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import { type Subscription } from '@/types/finance.types'
import { subscriptionsKey } from './financeKeys'
import { useUndoableDelete } from './useUndoableDelete'
import { useOpenFormOnQueryParam } from './useOpenFormOnQueryParam'
import SubscriptionForm from './SubscriptionForm'
import SubscriptionsTable from './SubscriptionsTable'
import SubscriptionsMobileList from './SubscriptionsMobileList'

export default function SuscripcionesTab() {
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Subscription | null>(null)
  const { listSubscriptions, updateSubscription, deleteSubscription } = useFinanceApi()
  const queryClient = useQueryClient()

  const { data: { subscriptions, monthlyCommittedCents: committed } = { subscriptions: [], monthlyCommittedCents: 0 } } = useQuery({
    queryKey: subscriptionsKey(),
    queryFn: () => listSubscriptions(),
  })

  useOpenFormOnQueryParam(() => {
    setEditing(null)
    setFormOpen(true)
  })

  async function toggleActive(s: Subscription, active: boolean) {
    try {
      await updateSubscription(s.id, {
        name: s.name,
        amountCents: s.amountCents,
        frequency: s.frequency,
        category: s.category,
        nextBillingOn: s.nextBillingOn,
        cardId: s.cardId ?? 0,
        active,
      })
      queryClient.invalidateQueries({ queryKey: subscriptionsKey() })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo actualizar la suscripción')
    }
  }

  const { handleDelete } = useUndoableDelete<Subscription, number>({
    getId: (s) => s.id,
    deleteFn: deleteSubscription,
    removeFromCache: (s) => {
      let removedIndex = -1
      queryClient.setQueryData(
        subscriptionsKey(),
        (prev: { subscriptions: Subscription[]; monthlyCommittedCents: number } = { subscriptions: [], monthlyCommittedCents: 0 }) => {
          removedIndex = prev.subscriptions.findIndex((x) => x.id === s.id)
          return { ...prev, subscriptions: prev.subscriptions.filter((x) => x.id !== s.id) }
        }
      )
      return removedIndex
    },
    restoreToCache: (s, removedIndex) => {
      queryClient.setQueryData(
        subscriptionsKey(),
        (prev: { subscriptions: Subscription[]; monthlyCommittedCents: number } = { subscriptions: [], monthlyCommittedCents: 0 }) => {
          const next = [...prev.subscriptions]
          next.splice(Math.min(removedIndex, next.length), 0, s)
          return { ...prev, subscriptions: next }
        }
      )
    },
    successMessage: 'Suscripción eliminada',
    errorMessage: 'No se pudo eliminar la suscripción',
    onDeleteError: () => queryClient.invalidateQueries({ queryKey: subscriptionsKey() }),
  })

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <CozyCard className="animate-card-in min-w-64">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total mensual comprometido
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatPEN(committed)}</div>
            <p className="mt-1 text-xs text-muted-foreground">
              Los cobros se registran automáticamente como gastos al llegar la fecha
            </p>
          </CardContent>
        </CozyCard>
        <Button
          className="hidden sm:inline-flex"
          onClick={() => {
            setEditing(null)
            setFormOpen(true)
          }}
        >
          <Plus size={14} />
          Nueva suscripción
        </Button>
      </div>

      <SubscriptionsTable
        subscriptions={subscriptions}
        onEdit={(s) => {
          setEditing(s)
          setFormOpen(true)
        }}
        onDelete={handleDelete}
        onToggleActive={toggleActive}
        onNewSubscription={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

      <SubscriptionsMobileList
        subscriptions={subscriptions}
        onEdit={(s) => {
          setEditing(s)
          setFormOpen(true)
        }}
        onDelete={handleDelete}
        onToggleActive={toggleActive}
        onNewSubscription={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

      <SubscriptionForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={() => queryClient.invalidateQueries({ queryKey: subscriptionsKey() })}
        editing={editing}
      />
    </div>
  )
}
