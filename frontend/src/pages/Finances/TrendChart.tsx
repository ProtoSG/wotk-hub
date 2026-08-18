import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import type { TooltipContentProps } from 'recharts'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { ChartTooltip } from '@/components/ui/chart-tooltip'
import { formatPEN } from '@/lib/currency'
import type { TrendPoint } from '@/types/finance.types'

interface Props {
  data: TrendPoint[]
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

export default function TrendChart({ data }: Props) {
  const chartData = data.map((p) => ({
    name: p.month.slice(5) + '/' + p.month.slice(2, 4),
    Ingresos: p.incomeCents / 100,
    Gastos: p.expenseCents / 100,
  }))

  return (
    <CozyCard className="animate-card-in">
      <CardHeader>
        <CardTitle className="text-sm font-medium">Ingresos vs Gastos (6 meses)</CardTitle>
      </CardHeader>
      <CardContent>
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
            <Bar dataKey="Ingresos" fill="var(--income)" radius={[4, 4, 0, 0]} isAnimationActive={false} />
            <Bar dataKey="Gastos" fill="var(--expense)" radius={[4, 4, 0, 0]} isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </CozyCard>
  )
}
