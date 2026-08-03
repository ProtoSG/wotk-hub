const penFormatter = new Intl.NumberFormat('es-PE', {
  style: 'currency',
  currency: 'PEN',
})

/** Formats integer cents as "S/ 1,234.56" */
export function formatPEN(cents: number): string {
  return penFormatter.format(cents / 100)
}

/** Converts a soles amount (e.g. 25.5) to integer cents */
export function solesToCents(soles: number): number {
  return Math.round(soles * 100)
}

/** Converts integer cents to soles for form inputs */
export function centsToSoles(cents: number): number {
  return cents / 100
}

/** Current month as "YYYY-MM" */
export function currentMonth(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

/** Shifts a "YYYY-MM" month by delta months */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  const d = new Date(y, m - 1 + delta, 1)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

/** Human label for "YYYY-MM", e.g. "julio 2026" */
export function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number)
  return new Date(y, m - 1, 1).toLocaleDateString('es-PE', { month: 'long', year: 'numeric' })
}

/** Compact label for "YYYY-MM", e.g. "jul 2026" — narrow layouts (mobile MonthPicker). */
export function monthLabelShort(month: string): string {
  const [y, m] = month.split('-').map(Number)
  return new Date(y, m - 1, 1).toLocaleDateString('es-PE', { month: 'short', year: 'numeric' })
}
