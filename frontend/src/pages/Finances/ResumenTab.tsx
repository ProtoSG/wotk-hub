import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Wallet, TrendingUp, TrendingDown, Repeat, CreditCard } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Skeleton } from '@/components/ui/skeleton'
import { useCountUp } from '@/hooks/useCountUp'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import type { FinanceSummary, Card, SavingsGoal } from '@/types/finance.types'
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

function SkeletonTile() {
  return (
    <CozyCard>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-4 w-4 rounded" />
      </CardHeader>
      <CardContent>
        <Skeleton className="h-8 w-32" />
      </CardContent>
    </CozyCard>
  )
}

function SkeletonCharts() {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <CozyCard>
        <CardHeader className="pb-2">
          <Skeleton className="h-4 w-32" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-40 w-full" />
        </CardContent>
      </CozyCard>
      <CozyCard>
        <CardHeader className="pb-2">
          <Skeleton className="h-4 w-32" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-40 w-full" />
        </CardContent>
      </CozyCard>
    </div>
  )
}

export default function ResumenTab({ month }: Props) {
  const [summary, setSummary] = useState<FinanceSummary | null>(null)
  const [committed, setCommitted] = useState(0)
  const [cards, setCards] = useState<Card[]>([])
  const [goals, setGoals] = useState<SavingsGoal[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const { getSummary, listSubscriptions, listCards, listGoals } = useFinanceApi()

  const load = useCallback(async () => {
    setIsLoading(true)
    try {
      const [s, subs, cardsData, goalsData] = await Promise.all([
        getSummary(month),
        listSubscriptions(),
        listCards(),
        listGoals(),
      ])
      setSummary(s)
      setCommitted(subs.monthlyCommittedCents)
      setCards(cardsData)
      setGoals(goalsData)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo cargar el resumen')
    } finally {
      setIsLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [month])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    load()
  }, [load])

  // "Disponible" = netWorth (summary.balanceCents — transfer-agnostic, already
  // filters type != 'transfer') MINUS what's already committed to savings goals.
  // Computed frontend-side; no new backend endpoint (design #40 "Disponible").
  // Goal contributions visibly reduce this number because their `currentCents`
  // is subtracted from the spendable pool.
  const goalsCommittedToCents = goals.reduce((sum, g) => sum + g.currentCents, 0)
  const disponibleCents = summary ? summary.balanceCents - goalsCommittedToCents : 0

  const tiles: Tile[] = [
    {
      label: 'Disponible',
      cents: disponibleCents,
      icon: Wallet,
      color: disponibleCents >= 0 ? 'text-income' : 'text-expense',
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

  if (isLoading && summary === null) {
    return (
      <div className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <SkeletonTile />
          <SkeletonTile />
          <SkeletonTile />
          <SkeletonTile />
        </div>
        <SkeletonCharts />
      </div>
    )
  }

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
      {/* Saldos en tarjetas */}
      {cards.length > 0 && (
        <CozyCard className="animate-card-in" style={{ animationDelay: '200ms' }}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Saldos en tarjetas</CardTitle>
            <CreditCard size={16} className="text-muted-foreground" />
          </CardHeader>
          <CardContent className="space-y-2">
            {cards.map((card) => (
              <div key={card.id} className="flex justify-between text-sm">
                <span className="text-muted-foreground">
                  {card.name} •• ••{card.last4}
                </span>
                <span className="font-medium">{formatPEN(card.balanceCents)}</span>
              </div>
            ))}
            <div className="border-t pt-2 mt-2 flex justify-between font-semibold">
              <span>Total en tarjetas</span>
              <span>
                {formatPEN(
                  cards
                    .filter((c) => c.creditLimitCents === 0)
                    .reduce((sum, c) => sum + c.balanceCents, 0)
                )}
              </span>
            </div>
            {summary && (
              <div className="flex justify-between text-xs text-muted-foreground pt-1">
                <span>Neto:</span>
                <span>{formatPEN(summary.balanceCents)}</span>
              </div>
            )}
            {summary &&
              // "Sin asignar" reconciliation removed (structurally impossible
              // once every tx is tagged to a card; design #40 / explore R5).
              goalsCommittedToCents !== 0 && (
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>En metas de ahorro:</span>
                  <span>{formatPEN(goalsCommittedToCents)}</span>
                </div>
              )}
          </CardContent>
        </CozyCard>
      )}
      <div className="grid gap-4 lg:grid-cols-2">
        <TrendChart data={summary?.monthlyTrend ?? []} />
        <CategoryChart data={summary?.categoryBreakdown ?? []} />
      </div>
    </div>
  )
}
