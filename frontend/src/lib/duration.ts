/**
 * Durations travel as whole seconds and distances as whole metres, for the
 * same reason weights travel as grams: no float drift in totals.
 */

/** "45s", "2:30", "1:05:00" — the shape a stopwatch would show. */
export function formatDuration(totalSeconds: number): string {
  if (totalSeconds < 60) return `${totalSeconds}s`

  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const paddedSeconds = String(seconds).padStart(2, '0')

  if (hours === 0) return `${minutes}:${paddedSeconds}`
  return `${hours}:${String(minutes).padStart(2, '0')}:${paddedSeconds}`
}

/**
 * Parses what the user typed into a duration field. Accepts both a bare count
 * of seconds ("45") and clock notation ("2:30", "1:05:00"), because either is
 * a reasonable thing to type into a box labelled "tiempo".
 */
export function parseDurationInput(value: string): number | null {
  const trimmed = value.trim()
  if (trimmed === '') return 0

  const parts = trimmed.split(':')
  if (parts.length > 3) return null

  let seconds = 0
  for (const part of parts) {
    const n = Number(part)
    if (!Number.isFinite(n) || n < 0) return null
    seconds = seconds * 60 + n
  }
  return Math.round(seconds)
}

/** Metres under a kilometre, km above it. */
export function formatDistance(meters: number): string {
  if (meters < 1000) return `${meters} m`
  const km = meters / 1000
  return `${km % 1 === 0 ? km : km.toFixed(2)} km`
}

/** Input is in kilometres — nobody logs a run in metres. */
export function parseKmInput(value: string): number | null {
  const normalized = value.replace(',', '.').trim()
  if (normalized === '') return 0
  const km = Number(normalized)
  if (!Number.isFinite(km) || km < 0) return null
  return Math.round(km * 1000)
}

export function metersToKm(meters: number): number {
  return meters / 1000
}

/** "5:30 /km" — pace is what tells you whether a run actually improved. */
export function formatPace(seconds: number, meters: number): string {
  if (meters <= 0 || seconds <= 0) return '—'
  const secondsPerKm = Math.round(seconds / (meters / 1000))
  const minutes = Math.floor(secondsPerKm / 60)
  return `${minutes}:${String(secondsPerKm % 60).padStart(2, '0')} /km`
}
