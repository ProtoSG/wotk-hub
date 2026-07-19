import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LayoutDashboard, ArrowLeftRight, Repeat, Target, CreditCard, PiggyBank, Settings, Loader2 } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { FloatingActionButton } from '@/components/ui/floating-action-button'
import { currentMonth } from '@/lib/currency'
import { useFinancesPageData } from './useFinancesPageData'
import { useActiveTab } from './useActiveTab'
import MobileTabNav from './MobileTabNav'
import MonthPicker from './MonthPicker'
import OnboardingGate from './OnboardingGate'
import ResumenTab from './ResumenTab'
import MovimientosTab from './MovimientosTab'
import SuscripcionesTab from './SuscripcionesTab'
import PresupuestosTab from './PresupuestosTab'
import TarjetasTab from './TarjetasTab'
import MetasTab from './MetasTab'

const FAB_TABS = new Set(['movimientos', 'suscripciones', 'presupuestos', 'tarjetas', 'metas'])

const FAB_LABELS: Record<string, string> = {
  movimientos   : 'Nuevo movimiento',
  suscripciones : 'Nueva suscripción',
  presupuestos  : 'Nuevo presupuesto',
  tarjetas      : 'Nueva tarjeta',
  metas         : 'Nueva meta',
}

const TABS = [
  { value: 'resumen',       label: 'Resumen',       icon: LayoutDashboard },
  { value: 'movimientos',   label: 'Movimientos',   icon: ArrowLeftRight },
  { value: 'suscripciones', label: 'Suscripciones', icon: Repeat },
  { value: 'presupuestos',  label: 'Presupuestos',  icon: Target },
  { value: 'tarjetas',      label: 'Tarjetas',      icon: CreditCard },
  { value: 'metas',         label: 'Metas',         icon: PiggyBank },
]

export default function FinancesPage() {
  const navigate                  = useNavigate()
  const [month, setMonth]         = useState(currentMonth())
  const { tab, setSearchParams }  = useActiveTab(TABS, 'resumen')

  const {
    cardsList,
    cardsPending,
    summary,
    committed,
    goals,
    resumenLoading,
    invalidateCards
  } = useFinancesPageData(month)

  const viewState = cardsPending ? 'loading' : cardsList.length === 0 ? 'onboarding' : 'ready'

  return (
    <>
      <div className="space-y-6 pb-24 sm:pb-0">
        {viewState === 'ready' ? (
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
        ) : (
          <h1 className="text-2xl font-bold">Finanzas</h1>
        )}

        {viewState === 'loading' && (
          <div className="flex min-h-[50vh] items-center justify-center">
            <Loader2 className="animate-spin text-muted-foreground" size={24} />
          </div>
        )}

        {viewState === 'onboarding' && <OnboardingGate onSaved={invalidateCards} />}

        {viewState === 'ready' && (
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
                summary={summary}
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
        )}
      </div>

      {viewState === 'ready' && FAB_TABS.has(tab) && (
        <FloatingActionButton
          label={FAB_LABELS[tab]}
          onClick={() => setSearchParams({ tab, new: '1' }, { replace: true })}
          className="bottom-[max(env(safe-area-inset-bottom),1rem)]"
        />
      )}

      {viewState === 'ready' && (
        <MobileTabNav
          tabs={TABS}
          activeTab={tab}
          onChange={(value) => setSearchParams({ tab: value }, { replace: true })}
          fabVisible={FAB_TABS.has(tab)}
        />
      )}
    </>
  )
}
