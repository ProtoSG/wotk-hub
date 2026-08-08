import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import type { PieLabelRenderProps } from 'recharts'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { DATE_CATEGORIES, DATE_CATEGORY_LABELS, type CoupleDate } from '@/types/couple.types'

// Same validated 8-slot categorical palette as Finanzas/CategoryChart — same
// surface (--card), so the validate_palette.js pass documented in index.css
// already covers this exact palette+surface pair.
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

const RADIAN = Math.PI / 180

// Direct value labels on the slices — mandatory secondary encoding for the
// palette's WARN-band adjacent pairs (see CHART_COLORS comment), same as
// Finanzas/CategoryChart.
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
  dates: CoupleDate[]
}

export default function CategoryDistributionChart({ dates }: Props) {
  const counts = new Map<string, number>()
  for (const d of dates) {
    counts.set(d.category, (counts.get(d.category) ?? 0) + 1)
  }

  // DATE_CATEGORIES has exactly 8 entries — one per chart slot, so there's
  // no "9th category folds into Otros" case to handle here (unlike
  // Finanzas' free-form expense categories). Iterating DATE_CATEGORIES in
  // its fixed declared order — rather than sorting by count — pins each
  // category to the same color every time regardless of which categories
  // are present or how many dates each has: color follows the category,
  // never its rank.
  const chartData = DATE_CATEGORIES.map((cat, i) => ({
    category: cat,
    name: DATE_CATEGORY_LABELS[cat] ?? cat,
    value: counts.get(cat) ?? 0,
    color: CHART_COLORS[i],
  })).filter((d) => d.value > 0)

  return (
    <CozyCard className="animate-card-in [animation-delay:180ms]">
      <CardHeader>
        <CardTitle className="text-sm font-medium text-muted-foreground">Categoría favorita</CardTitle>
      </CardHeader>
      <CardContent>
        {chartData.length === 0 ? (
          <div className="flex h-60 items-center justify-center text-sm text-muted-foreground">
            Todavía no hay citas realizadas
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
                {chartData.map((d) => (
                  <Cell key={d.category} fill={d.color} />
                ))}
              </Pie>
              <Tooltip
                formatter={(value) => [`${value} cita${value === 1 ? '' : 's'}`, undefined]}
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
