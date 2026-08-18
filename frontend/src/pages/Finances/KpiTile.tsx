import type { ComponentType, CSSProperties } from 'react'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { IconChip } from '@/components/ui/icon-chip'
import { useCountUp } from '@/hooks/useCountUp'
import { formatPENCompact } from '@/lib/currency'
import TrendSparkline, { type TrendSparklinePoint } from './TrendSparkline'

interface Props {
  label: string
  icon: ComponentType<{ className?: string }>
  accent: string // IconChip token, e.g. '--income'
  cents: number
  valueColor?: string // text color class for the value, e.g. 'text-income'
  sparkline?: { data: TrendSparklinePoint[]; color: string }
  style?: CSSProperties
}

/**
 * Compact KPI card: icon + label on one row, big value below, optional
 * trend sparkline underneath. Used for the 3 secondary mobile/tablet tiles
 * (no sparkline) and reused as-is for all 4 desktop tiles (with sparkline)
 * — same visual language everywhere, only the row layout around it changes
 * per breakpoint (see ResumenTab).
 */
export default function KpiTile({ label, icon: Icon, accent, cents, valueColor, sparkline, style }: Props) {
  const animated = useCountUp(cents)
  return (
    <CozyCard className="animate-card-in" style={style}>
      <CardContent className="flex flex-col gap-1.5 p-3 sm:p-6 sm:pt-6">
        <div className="flex items-center gap-1.5">
          <IconChip icon={Icon} accent={accent} size="xs" />
          <span className="truncate text-xs font-medium text-muted-foreground sm:text-sm">{label}</span>
        </div>
        <div className={`text-lg font-bold sm:text-2xl ${valueColor ?? ''}`}>{formatPENCompact(animated)}</div>
        {sparkline && <TrendSparkline data={sparkline.data} color={sparkline.color} />}
      </CardContent>
    </CozyCard>
  )
}
