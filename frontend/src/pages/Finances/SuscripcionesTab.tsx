import { useCallback, useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { toast } from 'sonner'
import { useSearchParams } from 'react-router-dom'
import { Plus, Pencil, Trash2, Repeat, MoreVertical } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Switch } from '@/components/ui/switch'
import { EmptyState } from '@/components/ui/empty-state'
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
import { CATEGORY_LABELS, FREQUENCY_LABELS, type Subscription } from '@/types/finance.types'
import SubscriptionForm from './SubscriptionForm'

const UNDO_WINDOW_MS = 4500

export default function SuscripcionesTab() {
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [committed, setCommitted] = useState(0)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Subscription | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()
  const { listSubscriptions, updateSubscription, deleteSubscription } = useFinanceApi()
  const pendingDeletes = useRef(new Map<number, number>())

  // Open form when navigated with ?new=1
  useEffect(() => {
    if (searchParams.get('new') === '1') {
      flushSync(() => {
        setEditing(null)
        setFormOpen(true)
        setSearchParams({}, { replace: true })
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- setSearchParams identity is stable, only react to searchParams changing
  }, [searchParams])

  const load = useCallback(async () => {
    try {
      const data = await listSubscriptions()
      setSubscriptions(data.subscriptions)
      setCommitted(data.monthlyCommittedCents)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las suscripciones')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

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
      load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo actualizar la suscripción')
    }
  }

  async function commitDelete(id: number) {
    pendingDeletes.current.delete(id)
    try {
      await deleteSubscription(id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la suscripción')
      load()
    }
  }

  function handleDelete(s: Subscription) {
    let removedIndex = -1
    setSubscriptions((prev) => {
      removedIndex = prev.findIndex((x) => x.id === s.id)
      return prev.filter((x) => x.id !== s.id)
    })

    const timer = window.setTimeout(() => commitDelete(s.id), UNDO_WINDOW_MS)
    pendingDeletes.current.set(s.id, timer)

    toast.success('Suscripción eliminada', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(s.id)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(s.id)
          }
          setSubscriptions((prev) => {
            const next = [...prev]
            next.splice(Math.min(removedIndex, next.length), 0, s)
            return next
          })
        },
      },
    })
  }

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
                      action={{
                        label: 'Agregar suscripción',
                        onClick: () => {
                          setEditing(null)
                          setFormOpen(true)
                        },
                      }}
                    />
                  </TableCell>
                </TableRow>
              ) : (
                subscriptions.map((s) => (
                  <TableRow key={s.id} className={s.active ? '' : 'opacity-50'}>
                    <TableCell className="font-medium">{s.name}</TableCell>
                    <TableCell>{CATEGORY_LABELS[s.category] ?? s.category}</TableCell>
                    <TableCell>{FREQUENCY_LABELS[s.frequency]}</TableCell>
                    <TableCell className="whitespace-nowrap">{s.nextBillingOn}</TableCell>
                    <TableCell className="text-right font-medium">{formatPEN(s.amountCents)}</TableCell>
                    <TableCell>
                      <Switch checked={s.active} onCheckedChange={(v) => toggleActive(s, v)} />
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={`Editar suscripción ${s.name}`}
                          onClick={() => {
                            setEditing(s)
                            setFormOpen(true)
                          }}
                        >
                          <Pencil size={14} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={`Eliminar suscripción ${s.name}`}
                          onClick={() => handleDelete(s)}
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

      <CozyCard className="animate-card-in [animation-delay:60ms] sm:hidden">
        <CardContent className="p-0">
          {subscriptions.length === 0 ? (
            <EmptyState
              icon={<Repeat className="h-8 w-8" />}
              title="Sin suscripciones registradas"
              description="Agrega tus suscripciones para hacer seguimiento."
              action={{
                label: 'Agregar suscripción',
                onClick: () => {
                  setEditing(null)
                  setFormOpen(true)
                },
              }}
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
                    {CATEGORY_LABELS[s.category] ?? s.category} · {FREQUENCY_LABELS[s.frequency]}
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    Próximo cobro: {s.nextBillingOn}
                  </div>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1.5">
                  <span className="text-sm font-semibold">{formatPEN(s.amountCents)}</span>
                  <Switch checked={s.active} onCheckedChange={(v) => toggleActive(s, v)} />
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
                        setEditing(s)
                        setFormOpen(true)
                      }}
                    >
                      <Pencil className="h-4 w-4" />
                      Editar
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => handleDelete(s)}
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

      <SubscriptionForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={load}
        editing={editing}
      />
    </div>
  )
}
