import type { CSSProperties } from 'react'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Skeleton } from '@/components/ui/skeleton'
import type { MetricData } from '@/types/dashboard.types'

interface Props extends MetricData {
  style?: CSSProperties
}

export default function MetricCard({ label, value, change, trend, primary, style }: Props) {
  const TrendIcon = trend === 'up' ? TrendingUp : trend === 'down' ? TrendingDown : Minus
  const trendColor = trend === 'up' ? 'text-success' : trend === 'down' ? 'text-destructive' : 'text-muted-foreground'

  return (
    <CozyCard className="animate-card-in" style={style}>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className={primary ? 'text-3xl font-bold' : 'text-2xl font-bold'}>
          {value === null ? (
            <Skeleton className="h-8 w-24" />
          ) : (
            <span key={value} className="inline-block animate-in fade-in-0 zoom-in-95 duration-300">
              {value}
            </span>
          )}
        </div>
        {change && (
          <div className={`flex items-center gap-1 mt-1 text-xs ${trendColor}`}>
            <TrendIcon size={12} />
            <span>{change} desde la semana pasada</span>
          </div>
        )}
      </CardContent>
    </CozyCard>
  )
}
