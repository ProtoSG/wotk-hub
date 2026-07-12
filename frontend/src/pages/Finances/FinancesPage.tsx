import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { LayoutDashboard, ArrowLeftRight, Repeat, Target } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'
import { currentMonth } from '@/lib/currency'
import MonthPicker from './MonthPicker'
import ResumenTab from './ResumenTab'
import MovimientosTab from './MovimientosTab'
import SuscripcionesTab from './SuscripcionesTab'
import PresupuestosTab from './PresupuestosTab'

const TABS = [
  { value: 'resumen', label: 'Resumen', icon: LayoutDashboard },
  { value: 'movimientos', label: 'Movimientos', icon: ArrowLeftRight },
  { value: 'suscripciones', label: 'Suscripciones', icon: Repeat },
  { value: 'presupuestos', label: 'Presupuestos', icon: Target },
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
