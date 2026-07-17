import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { LayoutDashboard, ArrowLeftRight, Repeat, Target, CreditCard, PiggyBank, Plus, Settings } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { cn } from '@/lib/utils'
import { currentMonth } from '@/lib/currency'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import type { Card } from '@/types/finance.types'
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

export default function FinancesPage() {
  const navigate = useNavigate()
  const [month, setMonth] = useState(currentMonth())
  const [searchParams, setSearchParams] = useSearchParams()
  const param = searchParams.get('tab') ?? ''
  const tab = TABS.some((t) => t.value === param) ? param : 'resumen'

  // Gate state: null while cards are still loading so we don't flash the gate
  // before the first listCards resolves.
  const [cards, setCards] = useState<Card[] | null>(null)
  const { listCards } = useFinanceApi()

  const loadCards = useCallback(async () => {
    try {
      setCards(await listCards())
    } catch (err) {
      // Surface but lift the gate so the user isn't hard-blocked on a transient
      // fetch failure (the backend deletes-last-card guard still protects).
      setCards([])
      console.error('listCards failed in FinancesPage gate:', err)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    loadCards()
  }, [loadCards])

  const gateActive = cards === null || cards.length === 0

  if (gateActive) {
    return (
      <div className="space-y-6 pb-24 sm:pb-0">
        <h1 className="text-2xl font-bold">Finanzas</h1>
        <OnboardingGate onSaved={loadCards} />
      </div>
    )
  }

  return (
    <div className="space-y-6 pb-24 sm:pb-0">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-1">
          <h1 className="text-2xl font-bold">Finanzas</h1>
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
          <ResumenTab month={month} />
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

      {FAB_TABS.has(tab) && (
        <button
          type="button"
          aria-label={FAB_LABELS[tab]}
          onClick={() => setSearchParams({ tab, new: '1' }, { replace: true })}
          className="fixed right-4 bottom-6 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-foreground text-background shadow-lg transition-transform hover:scale-105 active:scale-95 sm:hidden"
        >
          <Plus className="h-6 w-6" />
        </button>
      )}

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