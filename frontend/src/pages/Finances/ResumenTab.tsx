import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Wallet, TrendingUp, TrendingDown, Repeat } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { useCountUp } from '@/hooks/useCountUp'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import type { FinanceSummary } from '@/types/finance.types'
import TrendChart from './TrendChart'
import CategoryChart from './CategoryChart'

interface Props {
  month: string
}

interface Tile {
  label: string
  cents: number
  icon: typeof Wallet
  color?: string
  primary?: boolean
}

function AnimatedPEN({ cents }: { cents: number }) {
  const animated = useCountUp(cents)
  return <>{formatPEN(animated)}</>
}

export default function ResumenTab({ month }: Props) {
  const [summary, setSummary] = useState<FinanceSummary | null>(null)
  const [committed, setCommitted] = useState(0)
  const { getSummary, listSubscriptions } = useFinanceApi()

  const load = useCallback(async () => {
    try {
      const [s, subs] = await Promise.all([getSummary(month), listSubscriptions()])
      setSummary(s)
      setCommitted(subs.monthlyCommittedCents)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo cargar el resumen')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [month])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

  const tiles: Tile[] = [
    {
      label: 'Balance total',
      cents: summary?.balanceCents ?? 0,
      icon: Wallet,
      color: summary ? (summary.balanceCents >= 0 ? 'text-income' : 'text-expense') : undefined,
      primary: true,
    },
    {
      label: 'Ingresos del mes',
      cents: summary?.monthIncomeCents ?? 0,
      icon: TrendingUp,
      color: 'text-income',
    },
    {
      label: 'Gastos del mes',
      cents: summary?.monthExpenseCents ?? 0,
      icon: TrendingDown,
      color: 'text-expense',
    },
    { label: 'Comprometido mensual', cents: committed, icon: Repeat },
  ]

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {tiles.map(({ label, cents, icon: Icon, color, primary }, i) => (
          <CozyCard
            key={label}
            className="animate-card-in"
            style={{ animationDelay: `${Math.min(i * 40, 320)}ms` }}
          >
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
              <Icon size={16} className={color ?? 'text-muted-foreground'} />
            </CardHeader>
            <CardContent>
              <div className={`${primary ? 'text-3xl' : 'text-2xl'} font-bold ${color ?? ''}`}>
                <AnimatedPEN cents={cents} />
              </div>
            </CardContent>
          </CozyCard>
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <TrendChart data={summary?.monthlyTrend ?? []} />
        <CategoryChart data={summary?.categoryBreakdown ?? []} />
      </div>
    </div>
  )
}
