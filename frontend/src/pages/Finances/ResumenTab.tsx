import { useState } from 'react'
import { CreditCard, Info, Repeat, TrendingDown, TrendingUp, Wallet } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { IconChip } from '@/components/ui/icon-chip'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useCountUp } from '@/hooks/useCountUp'
import { formatPEN } from '@/lib/currency'
import type { FinanceSummary, Card, SavingsGoal, TrendPoint } from '@/types/finance.types'
import TrendChart from './TrendChart'
import CategoryChart from './CategoryChart'
import KpiTile from './KpiTile'
import TrendSparkline, { type TrendSparklinePoint } from './TrendSparkline'
import { CARD_ICON_MAP } from './cardIcons'

interface Props {
  summary: FinanceSummary | null
  committed: number
  cards: Card[]
  goals: SavingsGoal[]
  isLoading: boolean
}

function AnimatedPEN({ cents }: { cents: number }) {
  const animated = useCountUp(cents)
  return <>{formatPEN(animated)}</>
}

// There's no historical balance series in the backend (`monthlyTrend` only
// carries income/expense per month), so "Disponible" over time is
// approximated as a running net starting from 0 at the first point — the
// shape of the trend, not a real historical account balance.
function cumulativeNet(trend: TrendPoint[]): TrendSparklinePoint[] {
  return trend.map((_, i) => {
    const runningCents = trend.slice(0, i + 1).reduce((sum, p) => sum + p.incomeCents - p.expenseCents, 0)
    return { month: trend[i].month, value: runningCents / 100 }
  })
}

function SkeletonHero() {
  return (
    <CozyCard>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-9 w-9 rounded-full" />
      </CardHeader>
      <CardContent>
        <Skeleton className="h-10 w-40" />
        <Skeleton className="mt-3 h-12 w-full" />
      </CardContent>
    </CozyCard>
  )
}

function SkeletonTile() {
  return (
    <CozyCard>
      <CardContent className="flex flex-col gap-1.5 p-3 sm:p-6 sm:pt-6">
        <div className="flex items-center gap-1.5">
          <Skeleton className="h-6 w-6 rounded-full" />
          <Skeleton className="h-3 w-12" />
        </div>
        <Skeleton className="h-6 w-14" />
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

export default function ResumenTab({ summary, committed, cards, goals, isLoading }: Props) {
  // Controlled (not just hover) so the "Neto" explainer tooltip also opens
  // on tap — Radix's default hover/focus-only trigger doesn't respond to
  // touch, and this app is mobile-first.
  const [netoTooltipOpen, setNetoTooltipOpen] = useState(false)

  // "Disponible" = netWorth (summary.balanceCents — transfer-agnostic, already
  // filters type != 'transfer') MINUS what's already committed to savings goals.
  // Computed frontend-side; no new backend endpoint (design #40 "Disponible").
  // Goal contributions visibly reduce this number because their `currentCents`
  // is subtracted from the spendable pool.
  const goalsCommittedToCents = goals.reduce((sum, g) => sum + g.currentCents, 0)
  const disponibleCents = summary ? summary.balanceCents - goalsCommittedToCents : 0
  const disponiblePositive = disponibleCents >= 0
  const heroColor = disponiblePositive ? 'text-income' : 'text-expense'
  const heroAccent = disponiblePositive ? '--income' : '--expense'

  const monthlyTrend = summary?.monthlyTrend ?? []
  const disponibleTrend = cumulativeNet(monthlyTrend)
  const incomeTrend: TrendSparklinePoint[] = monthlyTrend.map((p) => ({ month: p.month, value: p.incomeCents / 100 }))
  const expenseTrend: TrendSparklinePoint[] = monthlyTrend.map((p) => ({
    month: p.month,
    value: p.expenseCents / 100,
  }))

  if (isLoading && summary === null) {
    return (
      <div className="space-y-4">
        <div className="space-y-4 xl:hidden">
          <SkeletonHero />
          <div className="grid grid-cols-3 gap-2 sm:gap-4">
            <SkeletonTile />
            <SkeletonTile />
            <SkeletonTile />
          </div>
        </div>
        <div className="hidden gap-4 xl:grid xl:grid-cols-4">
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
      {/* Mobile/tablet: "Disponible" gets its own full-width hero card, the
          other 3 KPIs stay compact in a row below. Desktop (xl+) has room
          for all 4 in a single row instead — see the block right after. */}
      <div className="space-y-4 xl:hidden">
        <CozyCard className="animate-card-in">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Disponible</CardTitle>
            <IconChip icon={Wallet} accent={heroAccent} size="sm" />
          </CardHeader>
          <CardContent>
            <div className={`text-4xl font-bold ${heroColor}`}>
              <AnimatedPEN cents={disponibleCents} />
            </div>
            <div className="mt-2">
              <TrendSparkline data={disponibleTrend} color={`var(${heroAccent})`} />
            </div>
          </CardContent>
        </CozyCard>

        <div className="grid grid-cols-3 gap-2 sm:gap-4">
          <KpiTile
            label="Ingresos"
            icon={TrendingUp}
            accent="--income"
            cents={summary?.monthIncomeCents ?? 0}
            style={{ animationDelay: '40ms' }}
          />
          <KpiTile
            label="Gastos"
            icon={TrendingDown}
            accent="--expense"
            cents={summary?.monthExpenseCents ?? 0}
            style={{ animationDelay: '80ms' }}
          />
          <KpiTile
            label="Suscripciones"
            icon={Repeat}
            accent="--primary"
            cents={committed}
            style={{ animationDelay: '120ms' }}
          />
        </div>
      </div>

      {/* Desktop: all 4 KPIs in one row, same size, each with its own trend
          sparkline — except Suscripciones, which has no historical series
          in the backend (`monthlyCommittedCents` is a current snapshot, not
          a monthly series) so it's left without one rather than faked. */}
      <div className="hidden gap-4 xl:grid xl:grid-cols-4">
        <KpiTile
          label="Disponible"
          icon={Wallet}
          accent={heroAccent}
          cents={disponibleCents}
          valueColor={heroColor}
          sparkline={{ data: disponibleTrend, color: `var(${heroAccent})` }}
        />
        <KpiTile
          label="Ingresos"
          icon={TrendingUp}
          accent="--income"
          cents={summary?.monthIncomeCents ?? 0}
          sparkline={{ data: incomeTrend, color: 'var(--income)' }}
          style={{ animationDelay: '40ms' }}
        />
        <KpiTile
          label="Gastos"
          icon={TrendingDown}
          accent="--expense"
          cents={summary?.monthExpenseCents ?? 0}
          sparkline={{ data: expenseTrend, color: 'var(--expense)' }}
          style={{ animationDelay: '80ms' }}
        />
        <KpiTile
          label="Suscripciones"
          icon={Repeat}
          accent="--primary"
          cents={committed}
          style={{ animationDelay: '120ms' }}
        />
      </div>

      {/* Saldos en tarjetas */}
      {cards.length > 0 && (
        <CozyCard className="animate-card-in" style={{ animationDelay: '200ms' }}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Saldos en tarjetas</CardTitle>
            <CreditCard className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="space-y-2.5">
            {cards.map((card) => (
              <div key={card.id} className="flex items-center justify-between gap-2 text-sm">
                <span className="flex min-w-0 items-center gap-2 text-muted-foreground">
                  <IconChip icon={CARD_ICON_MAP[card.icon] ?? CreditCard} accent={card.color} size="xs" />
                  <span className="truncate">
                    {card.name} •• ••{card.last4}
                  </span>
                </span>
                <span
                  className={`shrink-0 font-medium ${card.balanceCents < 0 ? 'text-expense' : ''}`}
                >
                  {formatPEN(card.balanceCents)}
                </span>
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
                <TooltipProvider>
                  <Tooltip open={netoTooltipOpen} onOpenChange={setNetoTooltipOpen}>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        onClick={() => setNetoTooltipOpen((v) => !v)}
                        className="inline-flex items-center gap-1"
                      >
                        Neto
                        <Info className="h-3 w-3" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="top" className="max-w-56 text-xs">
                      El Total en tarjetas ya descontó los aportes a metas (mueven plata real de la
                      tarjeta). El Neto no los resta — por eso puede ser mayor mientras haya metas con
                      saldo aportado.
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
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
