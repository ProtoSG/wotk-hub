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
import { formatDuration } from '@/lib/duration'
import type { ProgressPoint, TrackingType } from '@/types/gym.types'
import {
  chartValue,
  describeEffort,
  formatMetric,
  METRIC_LABELS,
  metricUnit,
  type ProgressMetric,
} from './progressMetrics'

interface ExerciseProgressChartProps {
  points: ProgressPoint[]
  metric: ProgressMetric
  trackingType: TrackingType
}

/**
 * A single series at a time. Three metrics on one axis would be meaningless —
 * volume runs in the thousands of kg while a top set is double digits — and
 * comparing two exercises isn't the question this screen answers.
 */
export default function ExerciseProgressChart({
  points,
  metric,
  trackingType,
}: ExerciseProgressChartProps) {
  const unit = metricUnit(metric)
  // Totals belong on a zero baseline; a level (a top set, a best hold) does
  // not — anchoring those at zero flattens months of progress into a line.
  const isTotal = metric === 'volume' || metric === 'duration' || metric === 'distance'

  const data = points.map((point) => ({
    date: formatAxisDate(point.occurredOn),
    // Charted in the unit the user thinks in (kg, km, seconds), since recharts
    // needs a plain number and the axis has to read naturally.
    value: chartValue(point, metric),
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
          domain={isTotal ? [0, 'auto'] : ['dataMin - 5', 'dataMax + 5']}
          tickFormatter={(value: number) => formatAxisValue(value, unit)}
        />
        <Tooltip
          content={(props) => (
            <ProgressTooltip {...props} metric={metric} trackingType={trackingType} />
          )}
        />
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
  trackingType,
}: TooltipContentProps & { metric: ProgressMetric; trackingType: TrackingType }) {
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
          {formatMetric(point, metric)}
        </span>
      </p>
      <p className="text-muted-foreground tabular-nums">{describeEffort(point, trackingType)}</p>
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

/**
 * Axis ticks stay short: tonnes past 1000 kg, clock notation for time. A gutter
 * full of digits pushes the plot area out of the card.
 */
function formatAxisValue(value: number, unit: 'kg' | 'km' | 'seconds'): string {
  if (unit === 'seconds') return formatDuration(Math.round(value))
  if (unit === 'kg' && value >= 1000) return `${Math.round(value / 1000)} t`
  if (unit === 'km') return value % 1 === 0 ? String(value) : value.toFixed(1)
  return String(Math.round(value))
}
