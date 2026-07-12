import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import type { PieLabelRenderProps } from 'recharts'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { formatPEN } from '@/lib/currency'
import { CATEGORY_LABELS, type CategoryAmount } from '@/types/finance.types'

// Categorical chart palette — 8 slots, fixed order, never cycled. Two
// adjacent pairs sit in the validator's WARN band, which is only legal
// because this chart always pairs marks with a visible legend AND direct
// on-slice labels (see index.css for the full validation note).
const CHART_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-6)',
  'var(--chart-7)',
  'var(--chart-8)',
]

const MAX_SLICES = CHART_COLORS.length
const OTHER_CATEGORY = 'otros'

/**
 * There are more possible expense categories than chart slots. Rather than
 * generating a 9th/10th hue (the rainbow-palette anti-pattern this replaces
 * was already guilty of), fold every category past the top 7 by amount into
 * a single "Otros" slice. If "otros" is itself already one of the top 7 the
 * overflow is merged into that existing entry instead of appending a
 * duplicate slice.
 */
function foldIntoOtros(data: CategoryAmount[]): CategoryAmount[] {
  if (data.length <= MAX_SLICES) return data

  const sorted = [...data].sort((a, b) => b.amountCents - a.amountCents)
  const top = sorted.slice(0, MAX_SLICES - 1)
  const overflow = sorted.slice(MAX_SLICES - 1)
  const overflowTotal = overflow.reduce((sum, c) => sum + c.amountCents, 0)

  const hasOtros = top.some((c) => c.category === OTHER_CATEGORY)
  if (hasOtros) {
    return top.map((c) =>
      c.category === OTHER_CATEGORY ? { ...c, amountCents: c.amountCents + overflowTotal } : c
    )
  }

  return [...top, { category: OTHER_CATEGORY, amountCents: overflowTotal }]
}

const RADIAN = Math.PI / 180

/**
 * Direct value labels on the slices themselves — the mandatory secondary
 * encoding that makes the palette's WARN-band adjacent pairs legal (see
 * CHART_COLORS comment above). Positioned just outside the donut ring with
 * a short leader line rather than crammed inside the thin ring.
 */
function renderSliceLabel({ cx, cy, midAngle, outerRadius, percent }: PieLabelRenderProps) {
  if (midAngle === undefined || percent === undefined || percent === 0) return null
  const radius = outerRadius + 14
  const x = cx + radius * Math.cos(-midAngle * RADIAN)
  const y = cy + radius * Math.sin(-midAngle * RADIAN)
  return (
    <text
      x={x}
      y={y}
      fill="var(--muted-foreground)"
      fontSize={11}
      fontFamily="var(--font-sans)"
      textAnchor={x > cx ? 'start' : 'end'}
      dominantBaseline="central"
    >
      {`${Math.round(percent * 100)}%`}
    </text>
  )
}

interface Props {
  data: CategoryAmount[]
}

export default function CategoryChart({ data }: Props) {
  const chartData = foldIntoOtros(data).map((c) => ({
    name: CATEGORY_LABELS[c.category] ?? c.category,
    value: c.amountCents,
  }))

  return (
    <CozyCard className="animate-card-in [animation-delay:60ms]">
      <CardHeader>
        <CardTitle className="text-sm font-medium">Gastos por categoría</CardTitle>
      </CardHeader>
      <CardContent>
        {chartData.length === 0 ? (
          <div className="flex h-60 items-center justify-center text-sm text-muted-foreground">
            Sin gastos este mes
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={240} initialDimension={{ width: 500, height: 240 }}>
            <PieChart>
              <Pie
                data={chartData}
                dataKey="value"
                nameKey="name"
                innerRadius={55}
                outerRadius={85}
                paddingAngle={2}
                isAnimationActive={false}
                label={renderSliceLabel}
                labelLine={{ stroke: 'var(--border)' }}
              >
                {chartData.map((_, i) => (
                  <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip
                formatter={(value) => formatPEN(value as number)}
                contentStyle={{
                  backgroundColor: 'var(--card)',
                  border: '1px solid var(--border)',
                  borderRadius: '6px',
                  color: 'var(--foreground)',
                }}
              />
              <Legend
                iconType="circle"
                iconSize={8}
                wrapperStyle={{ fontSize: 12, fontFamily: 'var(--font-sans)', paddingTop: 8 }}
                labelStyle={{ color: 'var(--muted-foreground)' }}
              />
            </PieChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </CozyCard>
  )
}
