import { useState, useEffect, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import { LayoutDashboard, ArrowLeftRight, Repeat, Target, CreditCard, AlertCircle, PiggyBank, Plus } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'
import { currentMonth } from '@/lib/currency'
import MonthPicker from './MonthPicker'
import ResumenTab from './ResumenTab'
import { Button } from '@/components/ui/button'
import MovimientosTab from './MovimientosTab'
import SuscripcionesTab from './SuscripcionesTab'
import PresupuestosTab from './PresupuestosTab'
import { TarjetasTab, CardForm } from './TarjetasTab'
import { MetasTab } from './MetasTab'
import FloatingActionButton from './FloatingActionButton'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import type { Card, SavingsGoal } from '@/types/finance.types'

const TABS = [
  { value: 'resumen', label: 'Resumen', icon: LayoutDashboard },
  { value: 'movimientos', label: 'Movimientos', icon: ArrowLeftRight },
  { value: 'suscripciones', label: 'Suscripciones', icon: Repeat },
  { value: 'presupuestos', label: 'Presupuestos', icon: Target },
  { value: 'tarjetas', label: 'Tarjetas', icon: CreditCard },
  { value: 'metas', label: 'Metas', icon: PiggyBank },
]

export default function FinancesPage() {
  const [month, setMonth] = useState(currentMonth())
  const [searchParams, setSearchParams] = useSearchParams()
  const param = searchParams.get('tab') ?? ''
  const tab = TABS.some((t) => t.value === param) ? param : 'resumen'

  return (
    <div className="space-y-6 pb-24 sm:pb-0">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">Finanzas</h1>
        <MonthPicker month={month} onChange={setMonth} />
      </div>
      <Tabs value={tab} onValueChange={(v) => setSearchParams({ tab: v }, { replace: true })}>
        <TabsList className="hidden sm:inline-flex">
          {TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="resumen" className="mt-4">
          <ResumenTab month={month} />
        </TabsContent>
        <TabsContent value="movimientos" className="mt-4">
          <MovimientosTab month={month} />
        </TabsContent>
        <TabsContent value="suscripciones" className="mt-4">
          <SuscripcionesTab />
        </TabsContent>
        <TabsContent value="presupuestos" className="mt-4">
          <PresupuestosTab month={month} />
        </TabsContent>
        <TabsContent value="tarjetas" className="mt-4">
          <TarjetasTabWrapper />
        </TabsContent>
        <TabsContent value="metas" className="mt-4">
          <MetasTabWrapper />
        </TabsContent>
      </Tabs>

      <nav
        className="fixed left-4 z-40 flex h-14 items-center justify-around gap-0.5 rounded-full border bg-background px-2 shadow-lg sm:hidden"
        style={{
          right: tab === 'resumen' ? '1rem' : '5.5rem',
          bottom: 'calc(env(safe-area-inset-bottom) + 1rem)',
        }}
      >
        {TABS.map((t) => {
          const Icon = t.icon
          const active = tab === t.value
          return (
            <button
              key={t.value}
              type="button"
              onClick={() => setSearchParams({ tab: t.value }, { replace: true })}
              aria-label={t.label}
              className={cn(
                'flex h-11 flex-1 items-center justify-center rounded-full',
                active ? 'bg-muted text-foreground' : 'text-muted-foreground'
              )}
            >
              <Icon className="h-5 w-5" />
            </button>
          )
        })}
      </nav>
    </div>
  )
}

function TarjetasTabWrapper() {
  const { listCards } = useFinanceApi()
  const [cards, setCards] = useState<Card[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)
  const [editCard, setEditCard] = useState<Card | undefined>()
  const [formOpen, setFormOpen] = useState(false)

  useEffect(() => {
    let ignore = false
    listCards()
      .then(data => { if (!ignore) { setCards(data); setIsLoading(false) } })
      .catch(() => { if (!ignore) { setHasError(true); setIsLoading(false) } })
    return () => { ignore = true }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleRefresh = useCallback(() => {
    setIsLoading(true)
    setHasError(false)
    listCards()
      .then(data => { setCards(data); setIsLoading(false) })
      .catch(() => { setHasError(true); setIsLoading(false) })
  }, [listCards])

  if (isLoading) {
    return (
      <div className="flex justify-center py-8">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (hasError) {
    return (
      <div className="flex flex-col items-center gap-3 py-8 text-center">
        <div className="text-destructive">
          <AlertCircle className="h-8 w-8" />
        </div>
        <p className="text-sm text-muted-foreground">No se pudieron cargar las tarjetas</p>
        <Button size="sm" variant="outline" onClick={handleRefresh}>
          Reintentar
        </Button>
      </div>
    )
  }

  if (cards.length === 0) {
    return (
      <>
        <FloatingActionButton
          label="Nueva tarjeta"
          onClick={() => { setEditCard(undefined); setFormOpen(true) }}
        />
        <CardForm
          open={formOpen}
          onClose={() => setFormOpen(false)}
          onSuccess={(card) => {
            setCards([card])
            setFormOpen(false)
          }}
          editCard={editCard}
        />
        <div className="flex flex-col items-center gap-2 py-8 text-muted-foreground">
          <CreditCard className="h-8 w-8" />
          <p>No hay tarjetas registradas</p>
        </div>
      </>
    )
  }

  return (
    <>
      <div className="hidden justify-end sm:flex">
        <Button
          onClick={() => {
            setEditCard(undefined)
            setFormOpen(true)
          }}
        >
          <Plus size={14} />
          Nueva tarjeta
        </Button>
      </div>

      <FloatingActionButton
        label="Nueva tarjeta"
        onClick={() => { setEditCard(undefined); setFormOpen(true) }}
      />
      <CardForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSuccess={(card) => {
          if (editCard) {
            setCards(prev => prev.map(c => c.id === card.id ? card : c))
          } else {
            setCards(prev => [...prev, card])
          }
          setFormOpen(false)
        }}
        editCard={editCard}
      />
      <TarjetasTab cards={cards} onRefresh={handleRefresh} />
    </>
  )
}

function MetasTabWrapper() {
  const { listGoals } = useFinanceApi()
  const [goals, setGoals] = useState<SavingsGoal[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)

  useEffect(() => {
    let ignore = false
    listGoals()
      .then(data => { if (!ignore) { setGoals(data); setIsLoading(false) } })
      .catch(() => { if (!ignore) { setHasError(true); setIsLoading(false) } })
    return () => { ignore = true }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleRefresh = useCallback(() => {
    setIsLoading(true)
    setHasError(false)
    listGoals()
      .then(data => { setGoals(data); setIsLoading(false) })
      .catch(() => { setHasError(true); setIsLoading(false) })
  }, [listGoals])

  if (isLoading) {
    return (
      <div className="flex justify-center py-8">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (hasError) {
    return (
      <div className="flex flex-col items-center gap-3 py-8 text-center">
        <div className="text-destructive">
          <AlertCircle className="h-8 w-8" />
        </div>
        <p className="text-sm text-muted-foreground">No se pudieron cargar las metas</p>
        <Button size="sm" variant="outline" onClick={handleRefresh}>
          Reintentar
        </Button>
      </div>
    )
  }

  return <MetasTab goals={goals} onRefresh={handleRefresh} />
}
