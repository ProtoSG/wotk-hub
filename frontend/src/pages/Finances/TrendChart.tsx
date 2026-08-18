import { ComposedChart, Bar, Cell, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import type { TooltipContentProps } from 'recharts'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { ChartTooltip } from '@/components/ui/chart-tooltip'
import { formatPEN, currentMonth } from '@/lib/currency'
import type { TrendPoint } from '@/types/finance.types'

interface Props {
  data: TrendPoint[]
  /** Drill-down: clicking a month (bar or line point) jumps to Movimientos
   * on that month. */
  onMonthClick?: (month: string) => void
}

function TrendTooltip({ active, payload, label }: TooltipContentProps) {
  if (!active || !payload?.length) return null
  return (
    <ChartTooltip label={label}>
      {payload.map((entry, i) => (
        <p key={i} className="flex items-center gap-1.5 text-muted-foreground">
          <span className="inline-block h-2 w-2 rounded-full" style={{ backgroundColor: entry.color }} />
          {entry.name}:{' '}
          <span className="font-medium text-foreground">
            {formatPEN(Math.round((entry.value as number) * 100))}
          </span>
        </p>
      ))}
    </ChartTooltip>
  )
}

export default function TrendChart({ data, onMonthClick }: Props) {
  const thisMonth = currentMonth()
  const chartData = data.map((p) => ({
    month: p.month,
    name: p.month.slice(5) + '/' + p.month.slice(2, 4),
    Ingresos: p.incomeCents / 100,
    Gastos: p.expenseCents / 100,
    Neto: (p.incomeCents - p.expenseCents) / 100,
    isCurrent: p.month === thisMonth,
  }))
  const hasCurrentMonth = chartData.some((p) => p.isCurrent)

  return (
    <CozyCard className="animate-card-in">
      <CardHeader>
        <CardTitle className="text-sm font-medium">Ingresos vs Gastos (6 meses)</CardTitle>
      </CardHeader>
      <CardContent>
        <div style={{ cursor: onMonthClick ? 'pointer' : undefined }}>
          <ResponsiveContainer width="100%" height={240} initialDimension={{ width: 500, height: 240 }}>
            <ComposedChart
              data={chartData}
              onClick={(state) => {
                // Recharts v3's click state gives an index (as a string,
                // not the payload directly) — look the month back up from
                // our own chartData rather than trusting activePayload,
                // which isn't reliably populated on click events.
                const idx = state.activeIndex != null ? Number(state.activeIndex) : NaN
                const month = Number.isFinite(idx) ? chartData[idx]?.month : undefined
                if (month) onMonthClick?.(month)
              }}
            >
              <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.6} />
              <XAxis
                dataKey="name"
                tick={{ fontSize: 12, fontFamily: 'var(--font-sans)' }}
                className="fill-muted-foreground"
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fontSize: 12, fontFamily: 'var(--font-sans)' }}
                className="fill-muted-foreground"
                tickLine={false}
                axisLine={false}
                width={40}
              />
              <Tooltip content={TrendTooltip} cursor={{ fill: 'var(--muted)', opacity: 0.4 }} />
              <Legend
                iconType="circle"
                iconSize={8}
                wrapperStyle={{ fontSize: 12, fontFamily: 'var(--font-sans)', paddingTop: 8 }}
                labelStyle={{ color: 'var(--muted-foreground)' }}
              />
              {/* The in-progress month always looks "low" next to complete
                  months since it hasn't finished accumulating — fainter
                  bars (rather than a same-weight bar) keep it from reading
                  as a real drop. */}
              <Bar dataKey="Ingresos" fill="var(--income)" radius={[4, 4, 0, 0]} isAnimationActive={false}>
                {chartData.map((d, i) => (
                  <Cell key={i} fillOpacity={d.isCurrent ? 0.5 : 1} />
                ))}
              </Bar>
              <Bar dataKey="Gastos" fill="var(--expense)" radius={[4, 4, 0, 0]} isAnimationActive={false}>
                {chartData.map((d, i) => (
                  <Cell key={i} fillOpacity={d.isCurrent ? 0.5 : 1} />
                ))}
              </Bar>
              {/* --primary (hue 40) sits right next to --expense (hue 30) —
                  too close to tell apart in an 8px legend dot (confirmed
                  visually). --chart-2 is a blue, nowhere near the warm
                  income/expense pair. */}
              <Line
                type="monotone"
                dataKey="Neto"
                stroke="var(--chart-2)"
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
        {hasCurrentMonth && (
          <p className="mt-1 text-xs text-muted-foreground">* Este mes todavía está en curso.</p>
        )}
      </CardContent>
    </CozyCard>
  )
}
