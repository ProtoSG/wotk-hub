import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Sparkles } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Skeleton } from '@/components/ui/skeleton'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { useDbStore } from '@/store/dbStore'
import { formatPEN } from '@/lib/currency'
import { currentMonth } from '@/lib/currency'
import type { FinanceSummary } from '@/types/finance.types'
import type { MetricData } from '@/types/dashboard.types'
import MetricCard from './MetricCard'
import TrendChart from '@/pages/Finances/TrendChart'

export default function DashboardPage() {
  const [summary, setSummary] = useState<FinanceSummary | null>(null)
  const { getSummary } = useFinanceApi()
  const connections = useDbStore((s) => s.connections)

  const load = useCallback(async () => {
    try {
      setSummary(await getSummary(currentMonth()))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo cargar el dashboard')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as other pages
    load()
  }, [load])

  // Card labels/shells render immediately regardless of fetch state; only
  // the value itself waits on `summary` (null -> MetricCard shows an inline
  // placeholder for just the number). "Conexiones guardadas" is hydrated
  // synchronously from the persisted zustand store, so it never needs to wait.
  const metrics: MetricData[] = [
    { label: 'Balance total', value: summary ? formatPEN(summary.balanceCents) : null, primary: true },
    { label: 'Ingresos del mes', value: summary ? formatPEN(summary.monthIncomeCents) : null },
    { label: 'Gastos del mes', value: summary ? formatPEN(summary.monthExpenseCents) : null },
    { label: 'Conexiones guardadas', value: connections.length },
  ]

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Inicio</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {metrics.map((m, i) => (
          <MetricCard key={m.label} {...m} style={{ animationDelay: `${Math.min(i * 40, 320)}ms` }} />
        ))}
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {summary ? (
          <TrendChart data={summary.monthlyTrend} />
        ) : (
          <CozyCard className="animate-card-in">
            <CardHeader>
              <CardTitle className="text-sm font-medium">Ingresos vs Gastos (6 meses)</CardTitle>
            </CardHeader>
            <CardContent>
              <Skeleton className="h-60 w-full" />
            </CardContent>
          </CozyCard>
        )}

        <CozyCard className="animate-card-in">
          <CardHeader>
            <CardTitle className="text-sm font-medium">Estadísticas de consultas</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex h-56 flex-col items-center justify-center gap-2 text-center text-muted-foreground">
              <Sparkles size={20} />
              <p className="text-sm">Próximamente</p>
              <p className="max-w-xs text-xs">
                El seguimiento de consultas y actividad del DB Manager todavía no está implementado en el backend.
              </p>
            </div>
          </CardContent>
        </CozyCard>
      </div>
    </div>
  )
}
