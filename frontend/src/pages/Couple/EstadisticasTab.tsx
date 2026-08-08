import { Heart, CalendarDays, Clock, Wallet } from 'lucide-react'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { formatPEN } from '@/lib/currency'
import { cn } from '@/lib/utils'
import type { CoupleDate } from '@/types/couple.types'
import CategoryDistributionChart from './CategoryDistributionChart'
import MonthlyTrendChart from './MonthlyTrendChart'
import SpendByCategoryChart from './SpendByCategoryChart'

interface EstadisticasTabProps {
  dates: CoupleDate[]
  canSeePrice: boolean
}

// Days since the most recent done date — a lightweight nudge ("van 12 días
// sin salir") rather than a full streak-with-goal mechanic. Null when
// there's no done date yet (empty state below handles that).
function daysSinceLastDate(doneDates: CoupleDate[]): number | null {
  if (doneDates.length === 0) return null
  const mostRecent = doneDates.reduce((latest, d) => (d.occurredOn > latest ? d.occurredOn : latest), doneDates[0].occurredOn)
  const diffMs = Date.now() - new Date(mostRecent + 'T00:00:00').getTime()
  return Math.max(0, Math.floor(diffMs / 86_400_000))
}

export default function EstadisticasTab({ dates, canSeePrice }: EstadisticasTabProps) {
  const doneDates = dates.filter((d) => d.status === 'done')

  const rated = doneDates.filter((d) => d.rating != null)
  const avgRating = rated.length
    ? rated.reduce((sum, d) => sum + (d.rating ?? 0), 0) / rated.length
    : null
  const totalSpentCents = doneDates.reduce((sum, d) => sum + (d.costCents ?? 0), 0)
  const daysSince = daysSinceLastDate(doneDates)

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6">
      <div className={cn('grid grid-cols-2 gap-4', canSeePrice ? 'sm:grid-cols-4' : 'sm:grid-cols-3')}>
        <CozyCard className="animate-card-in">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <CalendarDays className="h-4 w-4" />
              Citas registradas
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{dates.length}</div>
          </CardContent>
        </CozyCard>
        <CozyCard className="animate-card-in [animation-delay:60ms]">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Clock className="h-4 w-4" />
              Última cita
            </CardTitle>
          </CardHeader>
          <CardContent>
            {daysSince != null ? (
              <div className="text-2xl font-bold">
                {daysSince === 0 ? 'Hoy' : `Hace ${daysSince}d`}
              </div>
            ) : (
              <Heart className="h-6 w-6 text-muted-foreground/35" strokeWidth={1.75} />
            )}
          </CardContent>
        </CozyCard>
        <CozyCard className={cn('animate-card-in [animation-delay:120ms]', !canSeePrice && 'col-span-2 sm:col-span-1')}>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Heart className="h-4 w-4" />
              Calificación promedio
            </CardTitle>
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
          <CozyCard className="animate-card-in [animation-delay:180ms]">
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                <Wallet className="h-4 w-4" />
                Total invertido
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{formatPEN(totalSpentCents)}</div>
            </CardContent>
          </CozyCard>
        )}
      </div>

      <div className={cn('grid gap-4', canSeePrice ? 'lg:grid-cols-3' : 'lg:grid-cols-2')}>
        <CategoryDistributionChart dates={doneDates} />
        <MonthlyTrendChart dates={doneDates} />
        {canSeePrice && <SpendByCategoryChart dates={doneDates} />}
      </div>
    </div>
  )
}
