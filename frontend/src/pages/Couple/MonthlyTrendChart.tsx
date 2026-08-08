import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import type { TooltipContentProps } from 'recharts'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard, paperSurfaceStyle } from '@/components/ui/cozy-card'
import type { CoupleDate } from '@/types/couple.types'

const MONTHS_WINDOW = 6

const MONTH_LABELS = [
  'ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic',
]

/** Same warm-paper tooltip surface as Finanzas/TrendChart. */
function TrendTooltip({ active, payload, label }: TooltipContentProps) {
  if (!active || !payload?.length) return null
  const value = payload[0]?.value as number
  return (
    <div
      className="rounded-[var(--radius)] px-3 py-2 text-sm shadow-[0_1px_2px_oklch(0.35_0.03_40/0.07),0_12px_28px_-10px_oklch(0.35_0.06_40/0.18)]"
      style={paperSurfaceStyle}
    >
      <p className="mb-1 font-medium text-foreground">{label}</p>
      <p className="text-muted-foreground">
        <span className="font-medium text-foreground">{value}</span> cita{value === 1 ? '' : 's'}
      </p>
    </div>
  )
}

interface Props {
  dates: CoupleDate[]
}

export default function MonthlyTrendChart({ dates }: Props) {
  const now = new Date()
  const months: { key: string; name: string }[] = []
  for (let i = MONTHS_WINDOW - 1; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    months.push({ key: `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`, name: MONTH_LABELS[d.getMonth()] })
  }

  const counts = new Map<string, number>()
  for (const d of dates) {
    const key = d.occurredOn.slice(0, 7)
    counts.set(key, (counts.get(key) ?? 0) + 1)
  }

  const chartData = months.map((m) => ({ name: m.name, Citas: counts.get(m.key) ?? 0 }))
  const hasAny = chartData.some((d) => d.Citas > 0)

  return (
    <CozyCard className="animate-card-in [animation-delay:240ms]">
      <CardHeader>
        <CardTitle className="text-sm font-medium text-muted-foreground">Citas por mes</CardTitle>
      </CardHeader>
      <CardContent>
        {!hasAny ? (
          <div className="flex h-60 items-center justify-center text-sm text-muted-foreground">
            Sin citas en los últimos {MONTHS_WINDOW} meses
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={240} initialDimension={{ width: 500, height: 240 }}>
            <BarChart data={chartData}>
              <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.6} />
              <XAxis
                dataKey="name"
                tick={{ fontSize: 12, fontFamily: 'var(--font-sans)' }}
                className="fill-muted-foreground"
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                allowDecimals={false}
                tick={{ fontSize: 12, fontFamily: 'var(--font-sans)' }}
                className="fill-muted-foreground"
                tickLine={false}
                axisLine={false}
                width={30}
              />
              {/* Single series — title already names it, no legend needed. */}
              <Tooltip content={TrendTooltip} cursor={{ fill: 'var(--muted)', opacity: 0.4 }} />
              <Bar dataKey="Citas" fill="var(--primary)" radius={[4, 4, 0, 0]} isAnimationActive={false} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </CozyCard>
  )
}
