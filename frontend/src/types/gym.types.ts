export interface Exercise {
  id: number
  name: string
  equipment: string
  /** Empty string when the source catalog had no value for it. */
  primaryMuscle: string
  /** Comma-separated list, e.g. "Triceps, Shoulders". Empty when none. */
  secondaryMuscle: string
  mediaUrl: string
  mediaType: 'image' | 'video' | ''
  isCustom: boolean
}

export interface ExerciseFilters {
  q?: string
  muscle?: string
  equipment?: string
  limit?: number
  offset?: number
}

export interface ExerciseListResult {
  exercises: Exercise[]
  /** Match count ignoring limit/offset, for "showing N of M". */
  total: number
}

export interface ExerciseFilterValues {
  muscles: string[]
  equipment: string[]
}
