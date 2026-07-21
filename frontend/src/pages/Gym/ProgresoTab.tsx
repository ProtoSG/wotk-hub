import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Flame, LineChart as LineChartIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { EmptyState } from '@/components/ui/empty-state'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useGymApi } from '@/hooks/useGymApi'
import { cn } from '@/lib/utils'
import { formatKg, formatVolume } from '@/lib/weight'
import {
  exerciseProgressKey,
  loggedExercisesKey,
  progressSummaryKey,
} from './gymKeys'
import ExerciseProgressChart from './ExerciseProgressChart'
import { bestPoint, METRIC_LABELS, METRICS, type ProgressMetric } from './progressMetrics'

const RANGES = [
  { value: '1m', label: '1M', months: 1 },
  { value: '3m', label: '3M', months: 3 },
  { value: '6m', label: '6M', months: 6 },
  { value: '1a', label: '1A', months: 12 },
] as const

export default function ProgresoTab() {
  const [exerciseId, setExerciseId] = useState<number | null>(null)
  const [range, setRange] = useState<(typeof RANGES)[number]['value']>('3m')
  const [metric, setMetric] = useState<ProgressMetric>('weight')
  const { loggedExercises, exerciseProgress, progressSummary } = useGymApi()

  const { data: exercises = [], isPending: loadingExercises } = useQuery({
    queryKey: loggedExercisesKey(),
    queryFn: () => loggedExercises(),
  })

  const { data: summary } = useQuery({
    queryKey: progressSummaryKey(),
    queryFn: () => progressSummary(),
  })

  // Falls back to the first logged exercise so the chart is populated on
  // arrival instead of asking for a choice before showing anything.
  const selectedId = exerciseId ?? exercises[0]?.id ?? null
  const from = rangeStart(RANGES.find((r) => r.value === range)!.months)

  const { data: points = [], isPending: loadingPoints } = useQuery({
    queryKey: exerciseProgressKey(selectedId ?? 0, from),
    queryFn: () => exerciseProgress(selectedId!, { from }),
    enabled: selectedId !== null,
  })

  if (loadingExercises) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-20 w-full rounded-lg" />
        <Skeleton className="h-72 w-full rounded-lg" />
      </div>
    )
  }

  if (exercises.length === 0) {
    return (
      <EmptyState
        icon={<LineChartIcon />}
        title="Todavía no hay nada que graficar"
        description="Registrá series en un par de entrenamientos y acá vas a ver cómo evoluciona cada ejercicio."
      />
    )
  }

  const selected = exercises.find((e) => e.id === selectedId)
  const best = bestPoint(points, metric)

  return (
    <div className="space-y-4">
      {summary && <SummaryTiles summary={summary} />}

      <CozyCard>
        <CardHeader className="gap-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle className="text-sm font-medium">Progreso por ejercicio</CardTitle>
            <div className="flex gap-1 rounded-md bg-muted p-1">
              {RANGES.map((r) => (
                <button
                  key={r.value}
                  type="button"
                  onClick={() => setRange(r.value)}
                  className={cn(
                    'min-h-9 flex-1 rounded px-3 text-sm font-medium transition-colors sm:flex-none',
                    range === r.value
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground',
                  )}
                >
                  {r.label}
                </button>
              ))}
            </div>
          </div>

          <Select
            value={selectedId === null ? '' : String(selectedId)}
            onValueChange={(value) => setExerciseId(Number(value))}
          >
            <SelectTrigger aria-label="Elegir ejercicio">
              <SelectValue placeholder="Elegí un ejercicio" />
            </SelectTrigger>
            <SelectContent>
              {exercises.map((exercise) => (
                <SelectItem key={exercise.id} value={String(exercise.id)}>
                  {exercise.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <div className="flex flex-wrap gap-2">
            {METRICS.map((value) => (
              <Button
                key={value}
                variant={metric === value ? 'default' : 'outline'}
                size="sm"
                onClick={() => setMetric(value)}
              >
                {METRIC_LABELS[value]}
              </Button>
            ))}
          </div>
        </CardHeader>

        <CardContent className="space-y-3">
          {loadingPoints ? (
            <Skeleton className="h-[260px] w-full rounded-md" />
          ) : points.length === 0 ? (
            <p className="py-16 text-center text-sm text-muted-foreground">
              Sin registros de {selected?.name ?? 'este ejercicio'} en este período.
            </p>
          ) : (
            <>
              <ExerciseProgressChart points={points} metric={metric} />
              {best && (
                <p className="text-sm text-muted-foreground">
                  Mejor marca del período:{' '}
                  <span className="font-medium text-foreground tabular-nums">
                    {metric === 'volume'
                      ? formatVolume(best.totalVolumeGrams)
                      : `${formatKg(metric === 'weight' ? best.maxWeightGrams : best.estimated1rmGrams)} kg`}
                  </span>{' '}
                  el {formatDay(best.occurredOn)}
                </p>
              )}
            </>
          )}
        </CardContent>
      </CozyCard>
    </div>
  )
}

function SummaryTiles({
  summary,
}: {
  summary: { sessionsThisMonth: number; volumeThisMonthGrams: number; weekStreak: number; topMuscle: string }
}) {
  const tiles = [
    { label: 'Este mes', value: `${summary.sessionsThisMonth}`, hint: 'entrenamientos' },
    { label: 'Volumen', value: formatVolume(summary.volumeThisMonthGrams), hint: 'este mes' },
    {
      label: 'Racha',
      value: `${summary.weekStreak}`,
      hint: summary.weekStreak === 1 ? 'semana' : 'semanas',
      icon: summary.weekStreak > 0 ? Flame : undefined,
    },
    { label: 'Músculo', value: summary.topMuscle || '—', hint: 'más entrenado' },
  ]

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {tiles.map((tile) => {
        const Icon = tile.icon
        return (
          <div key={tile.label} className="rounded-lg border bg-card px-4 py-3">
            <p className="text-sm text-muted-foreground">{tile.label}</p>
            <p className="mt-0.5 flex items-center gap-1.5 text-lg font-semibold tabular-nums">
              {Icon && <Icon className="h-4 w-4 text-primary" />}
              <span className="truncate">{tile.value}</span>
            </p>
            <p className="text-xs text-muted-foreground">{tile.hint}</p>
          </div>
        )
      })}
    </div>
  )
}

/** ISO date `months` back from today, for the range filter. */
function rangeStart(months: number): string {
  const date = new Date()
  date.setMonth(date.getMonth() - months)
  return date.toISOString().slice(0, 10)
}

function formatDay(occurredOn: string): string {
  return new Date(`${occurredOn}T00:00:00`).toLocaleDateString('es', {
    day: 'numeric',
    month: 'short',
  })
}
