import { AreaChart, Area, ResponsiveContainer } from 'recharts'

export interface TrendSparklinePoint {
  month: string
  value: number
}

interface Props {
  data: TrendSparklinePoint[]
  color: string
}

/**
 * Purely decorative trend line for a KPI tile — no axes, no grid, no
 * tooltip, no labeled values (callers may plot approximations, e.g. a
 * running net rather than a real historical balance — see callers for
 * specifics). Renders nothing below 2 points, since a line needs at least
 * two to mean anything.
 */
export default function TrendSparkline({ data, color }: Props) {
  if (data.length < 2) return null

  const gradientId = `trend-sparkline-${color.replace(/[^a-z0-9]/gi, '')}`

  return (
    <div className="h-12 w-full">
      <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 200, height: 48 }}>
        <AreaChart data={data} margin={{ top: 4, right: 0, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.25} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <Area
            type="monotone"
            dataKey="value"
            stroke={color}
            strokeWidth={2}
            fill={`url(#${gradientId})`}
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}
