/**
 * Weights travel as integer grams (see GYM_SPEC.md) for the same reason money
 * travels as cents: 2.5 kg increments and totals stay exact. These helpers are
 * the only place the conversion to display kg happens.
 */

export function gramsToKg(grams: number): number {
  return grams / 1000
}

export function kgToGrams(kg: number): number {
  return Math.round(kg * 1000)
}

/** "80", "82.5" — trailing zeros dropped, since most weights are whole. */
export function formatKg(grams: number): string {
  const kg = gramsToKg(grams)
  return Number.isInteger(kg) ? String(kg) : kg.toFixed(1)
}

export function formatKgLabel(grams: number): string {
  return `${formatKg(grams)} kg`
}

/**
 * Session volume runs into the hundreds of thousands of grams, so it reads as
 * tonnes past 1000 kg rather than as a wall of digits.
 */
export function formatVolume(grams: number): string {
  const kg = gramsToKg(grams)
  if (kg >= 1000) {
    const tonnes = kg / 1000
    return `${tonnes.toFixed(tonnes >= 10 ? 0 : 1)} t`
  }
  return `${Math.round(kg)} kg`
}

/**
 * Parses a weight the user typed. Accepts a comma as the decimal separator,
 * which is what a Spanish keyboard offers first.
 */
export function parseKgInput(value: string): number | null {
  const normalized = value.replace(',', '.').trim()
  if (normalized === '') return 0
  const kg = Number(normalized)
  if (!Number.isFinite(kg) || kg < 0) return null
  return kgToGrams(kg)
}
