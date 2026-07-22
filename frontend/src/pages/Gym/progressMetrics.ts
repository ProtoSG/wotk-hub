import { formatKg, formatVolume, gramsToKg } from '@/lib/weight'
import { formatDistance, formatDuration, metersToKm } from '@/lib/duration'
import type { ProgressPoint, TrackingType } from '@/types/gym.types'

export type ProgressMetric =
  | 'weight'
  | 'oneRepMax'
  | 'volume'
  | 'duration'
  | 'distance'
  | 'maxDuration'

export const METRIC_LABELS: Record<ProgressMetric, string> = {
  weight: 'Peso máximo',
  oneRepMax: '1RM estimado',
  volume: 'Volumen',
  duration: 'Tiempo total',
  distance: 'Distancia',
  maxDuration: 'Mejor tiempo',
}

/**
 * Which metrics mean anything for a given tracking type. Charting volume or an
 * estimated 1RM for a run would produce a line that moves without describing
 * anything — the numbers exist in the payload but the movement has no weight.
 */
export const METRICS_BY_TRACKING: Record<TrackingType, ProgressMetric[]> = {
  weight_reps: ['weight', 'oneRepMax', 'volume'],
  duration_distance: ['distance', 'duration'],
  duration: ['maxDuration', 'duration'],
}

/** The unit a metric is charted in, which decides axis and tooltip format. */
export type MetricUnit = 'kg' | 'km' | 'seconds'

export function metricUnit(metric: ProgressMetric): MetricUnit {
  if (metric === 'distance') return 'km'
  if (metric === 'duration' || metric === 'maxDuration') return 'seconds'
  return 'kg'
}

/** Raw value of a point for a metric, in that metric's storage unit. */
function rawValue(point: ProgressPoint, metric: ProgressMetric): number {
  switch (metric) {
    case 'weight':
      return point.maxWeightGrams
    case 'oneRepMax':
      return point.estimated1rmGrams
    case 'volume':
      return point.totalVolumeGrams
    case 'duration':
      return point.totalDurationSeconds
    case 'maxDuration':
      return point.maxDurationSeconds
    case 'distance':
      return point.totalDistanceMeters
  }
}

/** Chart-ready number: kg, km or seconds depending on the metric. */
export function chartValue(point: ProgressPoint, metric: ProgressMetric): number {
  const raw = rawValue(point, metric)
  const unit = metricUnit(metric)
  if (unit === 'kg') return gramsToKg(raw)
  if (unit === 'km') return metersToKm(raw)
  return raw
}

/** Human-readable value for tooltips and the best-of-period line. */
export function formatMetric(point: ProgressPoint, metric: ProgressMetric): string {
  const raw = rawValue(point, metric)
  if (metric === 'volume') return formatVolume(raw)
  if (metric === 'distance') return formatDistance(raw)
  if (metricUnit(metric) === 'seconds') return formatDuration(raw)
  return `${formatKg(raw)} kg`
}

/**
 * Describes the session's best effort, in the terms that tracking type is
 * measured in — the heaviest set, the longest hold, or the pace of the run.
 */
export function describeEffort(point: ProgressPoint, trackingType: TrackingType): string {
  if (trackingType === 'weight_reps') {
    return `Mejor serie: ${formatKg(point.topSet.weightGrams)} kg × ${point.topSet.reps}`
  }
  if (trackingType === 'duration') {
    return `Mejor tiempo: ${formatDuration(point.maxDurationSeconds)}`
  }
  return `${formatDistance(point.totalDistanceMeters)} en ${formatDuration(point.totalDurationSeconds)}`
}

/** Best point of a period for the metric on screen. */
export function bestPoint(points: ProgressPoint[], metric: ProgressMetric): ProgressPoint | null {
  if (points.length === 0) return null
  return points.reduce(
    (best, point) => (rawValue(point, metric) > rawValue(best, metric) ? point : best),
    points[0],
  )
}
