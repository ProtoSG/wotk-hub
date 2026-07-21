import type { ProgressPoint } from '@/types/gym.types'

export type ProgressMetric = 'weight' | 'oneRepMax' | 'volume'

export const METRIC_LABELS: Record<ProgressMetric, string> = {
  weight: 'Peso máximo',
  oneRepMax: '1RM estimado',
  volume: 'Volumen',
}

export const METRICS: ProgressMetric[] = ['weight', 'oneRepMax', 'volume']

/** The point's value for the metric on screen, in grams. */
export function metricGrams(point: ProgressPoint, metric: ProgressMetric): number {
  if (metric === 'weight') return point.maxWeightGrams
  if (metric === 'oneRepMax') return point.estimated1rmGrams
  return point.totalVolumeGrams
}

/** Best point of a period for the metric on screen. */
export function bestPoint(points: ProgressPoint[], metric: ProgressMetric): ProgressPoint | null {
  if (points.length === 0) return null
  return points.reduce(
    (best, point) => (metricGrams(point, metric) > metricGrams(best, metric) ? point : best),
    points[0],
  )
}
