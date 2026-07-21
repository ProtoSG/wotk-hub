import { useEffect, useState } from 'react'

/** Seconds elapsed since `startedAt`, ticking once a second. */
export function useElapsedSeconds(startedAt: string | undefined): number {
  // The elapsed value is derived, not stored — the tick only exists to
  // re-render, so a changing startedAt needs no resync.
  const [, setTick] = useState(0)

  useEffect(() => {
    const timer = window.setInterval(() => setTick((n) => n + 1), 1_000)
    return () => window.clearInterval(timer)
  }, [])

  return secondsSince(startedAt)
}

function secondsSince(startedAt: string | undefined): number {
  if (!startedAt) return 0
  const started = new Date(startedAt).getTime()
  if (Number.isNaN(started)) return 0
  return Math.max(0, Math.floor((Date.now() - started) / 1_000))
}

/**
 * "12:04" under an hour, "1:05:03" past it — clock format, with the minutes
 * zero-padded once hours appear so the digits don't jump around as it counts.
 */
export function formatDuration(totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const paddedSeconds = String(seconds).padStart(2, '0')

  if (hours === 0) return `${minutes}:${paddedSeconds}`
  return `${hours}:${String(minutes).padStart(2, '0')}:${paddedSeconds}`
}
