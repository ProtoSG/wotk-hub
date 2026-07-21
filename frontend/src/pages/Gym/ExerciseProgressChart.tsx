import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { TooltipContentProps } from 'recharts'
import { paperSurfaceStyle } from '@/components/ui/cozy-card'
import { formatKg, formatVolume, gramsToKg } from '@/lib/weight'
import type { ProgressPoint } from '@/types/gym.types'
import { METRIC_LABELS, metricGrams, type ProgressMetric } from './progressMetrics'

interface ExerciseProgressChartProps {
  points: ProgressPoint[]
  metric: ProgressMetric
}

/**
 * A single series at a time. Three metrics on one axis would be meaningless —
 * volume runs in the thousands of kg while a top set is double digits — and
 * comparing two exercises isn't the question this screen answers.
 */
export default function ExerciseProgressChart({ points, metric }: ExerciseProgressChartProps) {
  const isVolume = metric === 'volume'

  const data = points.map((point) => ({
    date: formatAxisDate(point.occurredOn),
    // Charted in kg, not grams: the axis should read in the unit the user
    // thinks in, and recharts needs a plain number.
    value: gramsToKg(metricGrams(point, metric)),
    point,
  }))

  return (
    <ResponsiveContainer width="100%" height={260} initialDimension={{ width: 500, height: 260 }}>
      <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
        <CartesianGrid vertical={false} stroke="var(--border)" strokeOpacity={0.6} />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 12, fontFamily: 'var(--font-sans)' }}
          className="fill-muted-foreground"
          tickLine={false}
          axisLine={false}
          minTickGap={24}
        />
        <YAxis
          tick={{ fontSize: 12, fontFamily: 'var(--font-sans)' }}
          className="fill-muted-foreground"
          tickLine={false}
          axisLine={false}
          width={48}
          // A lifter's working weight lives far from zero, so a zero-based
          // axis would flatten months of progress into a straight line.
          domain={isVolume ? [0, 'auto'] : ['dataMin - 5', 'dataMax + 5']}
          tickFormatter={(value: number) => (isVolume ? formatAxisVolume(value) : String(Math.round(value)))}
        />
        <Tooltip content={(props) => <ProgressTooltip {...props} metric={metric} />} />
        <Line
          type="monotone"
          dataKey="value"
          stroke="var(--primary)"
          strokeWidth={2}
          dot={{ r: 3, fill: 'var(--primary)', strokeWidth: 0 }}
          activeDot={{ r: 5 }}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}

/**
 * Same warm-paper surface as the Finances tooltip, so the chart chrome stays
 * part of the app's system instead of Recharts' stock box.
 */
function ProgressTooltip({
  active,
  payload,
  metric,
}: TooltipContentProps & { metric: ProgressMetric }) {
  if (!active || !payload?.length) return null
  const entry = payload[0].payload as { point: ProgressPoint }
  const point = entry.point

  return (
    <div
      className="rounded-[var(--radius)] px-3 py-2 text-sm shadow-[0_1px_2px_oklch(0.35_0.03_40/0.07),0_12px_28px_-10px_oklch(0.35_0.06_40/0.18)]"
      style={paperSurfaceStyle}
    >
      <p className="mb-1 font-medium text-foreground">{formatTooltipDate(point.occurredOn)}</p>
      <p className="text-muted-foreground">
        {METRIC_LABELS[metric]}:{' '}
        <span className="font-medium text-foreground tabular-nums">
          {metric === 'volume'
            ? formatVolume(point.totalVolumeGrams)
            : `${formatKg(metricGrams(point, metric))} kg`}
        </span>
      </p>
      <p className="text-muted-foreground tabular-nums">
        Mejor serie: {formatKg(point.topSet.weightGrams)} kg × {point.topSet.reps}
      </p>
    </div>
  )
}

function formatAxisDate(occurredOn: string): string {
  return new Date(`${occurredOn}T00:00:00`).toLocaleDateString('es', {
    day: 'numeric',
    month: 'short',
  })
}

function formatTooltipDate(occurredOn: string): string {
  return new Date(`${occurredOn}T00:00:00`).toLocaleDateString('es', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

/** Axis ticks are in kg; past a tonne they'd crowd the gutter with digits. */
function formatAxisVolume(kg: number): string {
  return kg >= 1000 ? `${Math.round(kg / 1000)} t` : String(Math.round(kg))
}
