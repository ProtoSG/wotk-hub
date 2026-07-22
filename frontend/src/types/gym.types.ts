/**
 * How an exercise is logged. Cardio is distance and time, an isometric hold is
 * time alone, and forcing either into reps records a meaningless number.
 */
export type TrackingType = 'weight_reps' | 'duration_distance' | 'duration'

export interface Exercise {
  id: number
  name: string
  equipment: string
  /** Empty string when the source catalog had no value for it. */
  primaryMuscle: string
  /** Comma-separated list, e.g. "Triceps, Shoulders". Empty when none. */
  secondaryMuscle: string
  /** Spanish how-to text; empty until seeded or written in the app. */
  description: string
  trackingType: TrackingType
  mediaUrl: string
  mediaType: 'image' | 'video' | ''
  isCustom: boolean
}

export interface ExerciseInput {
  name: string
  equipment: string
  primaryMuscle: string
  secondaryMuscle: string
  description: string
  trackingType: TrackingType
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

export interface ExerciseSet {
  id: number
  setNumber: number
  reps: number
  /** Integer grams — see lib/weight.ts for display conversion. */
  weightGrams: number
  /** Both stay 0 on a weight_reps set. */
  durationSeconds: number
  distanceMeters: number
  isWarmup: boolean
  completed: boolean
}

/** The set shape a bulk replace sends: no id, order carries the set number. */
export interface SetInput {
  reps: number
  weightGrams: number
  durationSeconds: number
  distanceMeters: number
  isWarmup: boolean
  completed: boolean
}

export interface SessionExercise {
  id: number
  exerciseId: number
  position: number
  notes: string
  exercise: Exercise
  sets: ExerciseSet[]
}

export interface Session {
  id: number
  routineId: number | null
  name: string
  occurredOn: string
  startedAt: string
  /** null while the session is in progress. */
  finishedAt: string | null
  notes: string
  exercises: SessionExercise[]
}

export interface SessionSummary {
  id: number
  routineId: number | null
  name: string
  occurredOn: string
  startedAt: string
  finishedAt: string | null
  notes: string
  exerciseCount: number
  totalReps: number
  totalVolumeGrams: number
}

export interface SessionInput {
  /** When set, the session is materialized from that template. */
  routineId?: number
  name: string
  occurredOn: string
  notes: string
}

export interface RoutineExercise {
  id: number
  exerciseId: number
  position: number
  targetSets: number
  targetReps: number
  notes: string
  exercise: Exercise
}

export interface Routine {
  id: number
  name: string
  notes: string
  color: string
  icon: string
  archived: boolean
  exercises: RoutineExercise[]
}

export interface RoutineSummary {
  id: number
  name: string
  notes: string
  color: string
  icon: string
  archived: boolean
  exerciseCount: number
}

/** One template entry as the builder sends it; order carries the position. */
export interface RoutineExerciseInput {
  exerciseId: number
  targetSets: number
  targetReps: number
  notes: string
}

export interface RoutineInput {
  name: string
  notes: string
  exercises: RoutineExerciseInput[]
}

export interface LastSetsResult {
  sets: ExerciseSet[]
  /** Date of the session the sets came from; absent when there are none. */
  occurredOn?: string
}

export interface TopSet {
  reps: number
  weightGrams: number
}

/** One session's worth of an exercise's progress. */
export interface ProgressPoint {
  sessionId: number
  occurredOn: string
  maxWeightGrams: number
  totalReps: number
  totalVolumeGrams: number
  topSet: TopSet
  /** Epley estimate from the top set: weight x (1 + reps/30). */
  estimated1rmGrams: number
  totalDurationSeconds: number
  totalDistanceMeters: number
  maxDurationSeconds: number
}

export interface ProgressSummary {
  sessionsThisMonth: number
  volumeThisMonthGrams: number
  /** Consecutive weeks with at least one session. */
  weekStreak: number
  /** Empty when nothing was logged in the trailing 90 days. */
  topMuscle: string
}

export interface ProgressFilters {
  from?: string
  to?: string
}
