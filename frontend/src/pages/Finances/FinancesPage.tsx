import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { LayoutDashboard, ArrowLeftRight, Repeat, Target, CreditCard, PiggyBank, Settings, Loader2 } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { FloatingActionButton } from '@/components/ui/floating-action-button'
import { cn } from '@/lib/utils'
import { currentMonth } from '@/lib/currency'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import MonthPicker from './MonthPicker'
import ResumenTab from './ResumenTab'
import MovimientosTab from './MovimientosTab'
import SuscripcionesTab from './SuscripcionesTab'
import PresupuestosTab from './PresupuestosTab'
import TarjetasTab, { CardFormFields } from './TarjetasTab'
import MetasTab from './MetasTab'

const FAB_TABS = new Set(['movimientos', 'suscripciones', 'presupuestos', 'tarjetas', 'metas'])

const FAB_LABELS: Record<string, string> = {
  movimientos: 'Nuevo movimiento',
  suscripciones: 'Nueva suscripción',
  presupuestos: 'Nuevo presupuesto',
  tarjetas: 'Nueva tarjeta',
  metas: 'Nueva meta',
}

const TABS = [
  { value: 'resumen', label: 'Resumen', icon: LayoutDashboard },
  { value: 'movimientos', label: 'Movimientos', icon: ArrowLeftRight },
  { value: 'suscripciones', label: 'Suscripciones', icon: Repeat },
  { value: 'presupuestos', label: 'Presupuestos', icon: Target },
  { value: 'tarjetas', label: 'Tarjetas', icon: CreditCard },
  { value: 'metas', label: 'Metas', icon: PiggyBank },
]

// Page-level onboarding gate (spec finance-onboarding / design #40). Blocks
// ALL Finances tabs until the owner has ≥1 card, regardless of card type.
// Reuses the existing listCards result + the CardFormFields body so the
// user creates their first card inline without ever seeing the tabbed content.
function OnboardingGate({ onSaved }: { onSaved: () => void }) {
  return (
    <CozyCard className="animate-card-in mx-auto mt-12 max-w-md">
      <CardContent className="flex flex-col items-center gap-4 py-8 text-center">
        <CreditCard className="h-12 w-12 opacity-30" />
        <div className="space-y-1">
          <h2 className="text-xl font-semibold">Para iniciar con tus finanzas</h2>
          <p className="text-sm text-muted-foreground">
            Agregá una tarjeta para empezar a registrar tus movimientos.
          </p>
        </div>
        <div className="w-full text-left">
          <CardFormFields onSaved={onSaved} />
        </div>
      </CardContent>
    </CozyCard>
  )
}

function cardsKey() {
  return ['finances', 'cards'] as const
}

function summaryKey(month: string) {
  return ['finances', 'summary', month] as const
}

function subscriptionsKey() {
  return ['finances', 'subscriptions'] as const
}

function goalsKey() {
  return ['finances', 'goals'] as const
}

export default function FinancesPage() {
  const navigate = useNavigate()
  const [month, setMonth] = useState(currentMonth())
  const [searchParams, setSearchParams] = useSearchParams()
  const param = searchParams.get('tab') ?? ''
  const tab = TABS.some((t) => t.value === param) ? param : 'resumen'

  const { listCards, getSummary, listSubscriptions, listGoals } = useFinanceApi()
  const queryClient = useQueryClient()

  // isPending is true while cards are still loading (no cached data yet) so
  // we don't flash the gate before the first listCards resolves.
  const { data: cards, isPending: cardsPending } = useQuery({
    queryKey: cardsKey(),
    queryFn: () => listCards(),
  })

  // Resumen's data (summary/subscriptions/goals) used to be fetched inside
  // ResumenTab itself, which only mounts once the gate below resolves — a
  // serialized extra round trip on the LCP path (gate fetch, then wait, then
  // this fetch). Fetching it here starts it in parallel with the cards query
  // instead. subscriptionsKey()/goalsKey() are shared with SuscripcionesTab
  // and MetasTab so switching tabs reuses this cache instead of re-fetching.
  const { data: summary, isPending: summaryPending } = useQuery({
    queryKey: summaryKey(month),
    queryFn: () => getSummary(month),
  })
  const { data: subscriptionsData, isPending: subscriptionsPending } = useQuery({
    queryKey: subscriptionsKey(),
    queryFn: () => listSubscriptions(),
  })
  const { data: goals = [], isPending: goalsPending } = useQuery({
    queryKey: goalsKey(),
    queryFn: () => listGoals(),
  })
  const committed = subscriptionsData?.monthlyCommittedCents ?? 0
  const resumenLoading = summaryPending || subscriptionsPending || goalsPending

  // cardsPending means "still loading, unknown" — show a spinner, not the
  // gate. Rendering the gate here used to flash "add your first card" at
  // every returning user for the split second before their real cards
  // arrived. Only a confirmed empty list means the gate.
  if (cardsPending) {
    return (
      <div className="space-y-6 pb-24 sm:pb-0">
        <h1 className="text-2xl font-bold">Finanzas</h1>
        <div className="flex min-h-[50vh] items-center justify-center">
          <Loader2 className="animate-spin text-muted-foreground" size={24} />
        </div>
      </div>
    )
  }

  const cardsList = cards ?? []

  if (cardsList.length === 0) {
    return (
      <div className="space-y-6 pb-24 sm:pb-0">
        <h1 className="text-2xl font-bold">Finanzas</h1>
        <OnboardingGate
          onSaved={() => queryClient.invalidateQueries({ queryKey: cardsKey() })}
        />
      </div>
    )
  }

  return (
    <>
      <div className="space-y-6 pb-24 sm:pb-0">
        <div className="flex flex-nowrap items-center justify-between gap-2 sm:gap-4">
          <div className="flex items-center gap-1">
            <h1 className="text-xl font-bold sm:text-2xl">Finanzas</h1>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Administrar categorías"
              onClick={() => navigate('/finances/categories')}
            >
              <Settings size={16} />
            </Button>
          </div>
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
          <TabsContent value="resumen" className="mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95">
            <ResumenTab
              summary={summary ?? null}
              committed={committed}
              cards={cardsList}
              goals={goals}
              isLoading={resumenLoading}
            />
          </TabsContent>
          <TabsContent value="movimientos" className="mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95">
            <MovimientosTab month={month} />
          </TabsContent>
          <TabsContent value="suscripciones" className="mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95">
            <SuscripcionesTab />
          </TabsContent>
          <TabsContent value="presupuestos" className="mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95">
            <PresupuestosTab month={month} />
          </TabsContent>
          <TabsContent value="tarjetas" className="mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95">
            <TarjetasTab />
          </TabsContent>
          <TabsContent value="metas" className="mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95">
            <MetasTab />
          </TabsContent>
        </Tabs>
      </div>

      {FAB_TABS.has(tab) && (
        <FloatingActionButton
          label={FAB_LABELS[tab]}
          onClick={() => setSearchParams({ tab, new: '1' }, { replace: true })}
          className="bottom-[max(env(safe-area-inset-bottom),1rem)]"
        />
      )}

      <nav
        className="fixed left-4 z-40 flex h-14 items-center justify-around gap-0.5 rounded-full border bg-background px-2 shadow-lg sm:hidden"
        style={{
          right: tab === 'resumen' ? '1rem' : '5.5rem',
          bottom: 'max(env(safe-area-inset-bottom), 1rem)',
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
    </>
  )
}
