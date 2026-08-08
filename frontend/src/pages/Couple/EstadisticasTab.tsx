import { Heart } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { formatPEN } from '@/lib/currency'
import { cn } from '@/lib/utils'
import type { CoupleDate } from '@/types/couple.types'

interface EstadisticasTabProps {
  dates: CoupleDate[]
  canSeePrice: boolean
}

export default function EstadisticasTab({ dates, canSeePrice }: EstadisticasTabProps) {
  const doneDates = dates.filter((d) => d.status === 'done')

  const rated = doneDates.filter((d) => d.rating != null)
  const avgRating = rated.length
    ? rated.reduce((sum, d) => sum + (d.rating ?? 0), 0) / rated.length
    : null
  const totalSpentCents = doneDates.reduce((sum, d) => sum + (d.costCents ?? 0), 0)

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6">
      <div className={cn('grid grid-cols-2 gap-4', canSeePrice && 'sm:grid-cols-3')}>
        <CozyCard className="animate-card-in">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Citas registradas</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{dates.length}</div>
          </CardContent>
        </CozyCard>
        <CozyCard className="animate-card-in [animation-delay:60ms]">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Calificación promedio</CardTitle>
          </CardHeader>
          <CardContent>
            {avgRating != null ? (
              <div className="text-2xl font-bold">{avgRating.toFixed(1)}</div>
            ) : (
              <Heart className="h-6 w-6 text-muted-foreground/35" strokeWidth={1.75} />
            )}
          </CardContent>
        </CozyCard>
        {canSeePrice && (
          <CozyCard className="col-span-2 animate-card-in [animation-delay:120ms] sm:col-span-1">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Total invertido</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{formatPEN(totalSpentCents)}</div>
            </CardContent>
          </CozyCard>
        )}
      </div>
    </div>
  )
}
