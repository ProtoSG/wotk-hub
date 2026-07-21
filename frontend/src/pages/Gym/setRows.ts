import type { ExerciseSet, SetInput } from '@/types/gym.types'

/**
 * A set row as the logging UI holds it. `key` is client-only and stable for
 * the row's lifetime: sets have no id until they're saved, and the API
 * renumbers them on every bulk replace, so neither the id nor the array index
 * can identify a row across edits. Without it, deleting a middle row makes
 * React reuse the wrong input state.
 */
export interface SetRow extends SetInput {
  key: string
}

/**
 * Keys are derived from data, never from a module-level counter: `toRows` runs
 * during render (as a useState initializer), and mutating shared state there
 * is impure — the same render could produce different keys.
 */
export function toRows(sets: readonly ExerciseSet[]): SetRow[] {
  return sets.map((set) => ({
    key: `saved-${set.id}`,
    reps: set.reps,
    weightGrams: set.weightGrams,
    isWarmup: set.isWarmup,
    completed: set.completed,
  }))
}

export function toInputs(rows: readonly SetRow[]): SetInput[] {
  return rows.map(({ reps, weightGrams, isWarmup, completed }) => ({
    reps,
    weightGrams,
    isWarmup,
    completed,
  }))
}

/**
 * Builds the next row to append. It copies the previous working set's weight
 * and reps, because the overwhelmingly common case is repeating the same
 * numbers — and starts uncompleted, since the set hasn't happened yet.
 */
export function nextRow(rows: readonly SetRow[]): SetRow {
  const previous = [...rows].reverse().find((row) => !row.isWarmup) ?? rows[rows.length - 1]
  return {
    // Only ever called from an event handler, so a random key is safe here —
    // and it can't collide with the `saved-<id>` keys above.
    key: `draft-${crypto.randomUUID()}`,
    reps: previous?.reps ?? 0,
    weightGrams: previous?.weightGrams ?? 0,
    isWarmup: false,
    completed: false,
  }
}

export interface NumberedRow {
  row: SetRow
  /**
   * Position among the working sets. Warmups carry the number of the last
   * working set before them and are never displayed by number, so "3 series"
   * means three real sets regardless of how many warmups are interleaved.
   */
  workingNumber: number
}

/** Pairs each row with its working-set number in a single pass. */
export function withWorkingNumbers(rows: readonly SetRow[]): NumberedRow[] {
  const numbered: NumberedRow[] = []
  for (const row of rows) {
    const previous = numbered.length === 0 ? 0 : numbered[numbered.length - 1].workingNumber
    numbered.push({ row, workingNumber: row.isWarmup ? previous : previous + 1 })
  }
  return numbered
}

/** True when both lists carry the same values in the same order. */
export function rowsEqual(a: readonly SetRow[], b: readonly SetRow[]): boolean {
  if (a.length !== b.length) return false
  return a.every((row, i) => {
    const other = b[i]
    return (
      row.reps === other.reps &&
      row.weightGrams === other.weightGrams &&
      row.isWarmup === other.isWarmup &&
      row.completed === other.completed
    )
  })
}
