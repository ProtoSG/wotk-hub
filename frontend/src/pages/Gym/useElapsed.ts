import { useEffect, useState } from 'react'

/**
 * Minutes elapsed since `startedAt`, ticking once a minute. Minute resolution
 * on purpose: a seconds counter reads as a stopwatch you're supposed to race,
 * which is the wrong pressure for a workout log.
 */
export function useElapsedMinutes(startedAt: string | undefined): number {
  // The elapsed value is derived, not stored — the tick only exists to
  // re-render once a minute, so a changing startedAt needs no resync.
  const [, setTick] = useState(0)

  useEffect(() => {
    const timer = window.setInterval(() => setTick((n) => n + 1), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  return minutesSince(startedAt)
}

function minutesSince(startedAt: string | undefined): number {
  if (!startedAt) return 0
  const started = new Date(startedAt).getTime()
  if (Number.isNaN(started)) return 0
  return Math.max(0, Math.floor((Date.now() - started) / 60_000))
}

/** "45 min", "1 h 05 min" */
export function formatDuration(minutes: number): string {
  if (minutes < 60) return `${minutes} min`
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return `${hours} h ${String(rest).padStart(2, '0')} min`
}
