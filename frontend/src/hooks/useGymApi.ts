import api from '@/lib/axios'
import type {
  Exercise,
  ExerciseFilters,
  ExerciseInput,
  ExerciseFilterValues,
  ExerciseListResult,
  LastSetsResult,
  ProgressFilters,
  ProgressPoint,
  ProgressSummary,
  Routine,
  RoutineInput,
  RoutineSummary,
  Session,
  SessionInput,
  SessionSummary,
  SetInput,
} from '@/types/gym.types'

export interface SessionFilters {
  from?: string
  to?: string
}

export function useGymApi() {
  async function listExercises(filters: ExerciseFilters = {}): Promise<ExerciseListResult> {
    const res = await api.get<ExerciseListResult>('/api/gym/exercises', { params: filters })
    return res.data
  }

  async function listExerciseFilters(): Promise<ExerciseFilterValues> {
    const res = await api.get<ExerciseFilterValues>('/api/gym/exercises/filters')
    return res.data
  }

  async function createExercise(input: ExerciseInput): Promise<Exercise> {
    const res = await api.post<Exercise>('/api/gym/exercises', input)
    return res.data
  }

  /** Full edit — custom exercises only; the API rejects seeded ones. */
  async function updateExercise(id: number, input: ExerciseInput): Promise<Exercise> {
    const res = await api.put<Exercise>(`/api/gym/exercises/${id}`, input)
    return res.data
  }

  /** Text-only edit, allowed on seeded exercises too. */
  async function updateExerciseDescription(id: number, description: string): Promise<Exercise> {
    const res = await api.put<Exercise>(`/api/gym/exercises/${id}/description`, { description })
    return res.data
  }

  async function deleteExercise(id: number): Promise<void> {
    await api.delete(`/api/gym/exercises/${id}`)
  }

  /** Sets logged for this exercise in the most recent session that had any. */
  async function lastSets(exerciseId: number): Promise<LastSetsResult> {
    const res = await api.get<LastSetsResult>(`/api/gym/exercises/${exerciseId}/last-sets`)
    return res.data
  }

  /** Only the exercises that have logged sets — what's worth charting. */
  async function loggedExercises(): Promise<Exercise[]> {
    const res = await api.get<ExerciseListResult>('/api/gym/progress/exercises')
    return res.data.exercises
  }

  async function exerciseProgress(
    exerciseId: number,
    filters: ProgressFilters = {},
  ): Promise<ProgressPoint[]> {
    const res = await api.get<{ points: ProgressPoint[] }>(
      `/api/gym/progress/exercises/${exerciseId}`,
      { params: filters },
    )
    return res.data.points
  }

  async function progressSummary(): Promise<ProgressSummary> {
    const res = await api.get<ProgressSummary>('/api/gym/progress/summary')
    return res.data
  }

  async function listRoutines(): Promise<RoutineSummary[]> {
    const res = await api.get<{ routines: RoutineSummary[] }>('/api/gym/routines')
    return res.data.routines
  }

  async function getRoutine(id: number): Promise<Routine> {
    const res = await api.get<Routine>(`/api/gym/routines/${id}`)
    return res.data
  }

  async function createRoutine(input: RoutineInput): Promise<Routine> {
    const res = await api.post<Routine>('/api/gym/routines', input)
    return res.data
  }

  async function updateRoutine(id: number, input: RoutineInput): Promise<Routine> {
    const res = await api.put<Routine>(`/api/gym/routines/${id}`, input)
    return res.data
  }

  async function deleteRoutine(id: number): Promise<void> {
    await api.delete(`/api/gym/routines/${id}`)
  }

  async function listSessions(filters: SessionFilters = {}): Promise<SessionSummary[]> {
    const res = await api.get<{ sessions: SessionSummary[] }>('/api/gym/sessions', { params: filters })
    return res.data.sessions
  }

  /** The in-progress session, or null when nothing is being logged. */
  async function activeSession(): Promise<Session | null> {
    const res = await api.get<{ session: Session | null }>('/api/gym/sessions/active')
    return res.data.session
  }

  async function getSession(id: number): Promise<Session> {
    const res = await api.get<Session>(`/api/gym/sessions/${id}`)
    return res.data
  }

  async function createSession(input: SessionInput): Promise<Session> {
    const res = await api.post<Session>('/api/gym/sessions', input)
    return res.data
  }

  async function updateSession(id: number, input: SessionInput): Promise<Session> {
    const res = await api.put<Session>(`/api/gym/sessions/${id}`, input)
    return res.data
  }

  async function finishSession(id: number): Promise<Session> {
    const res = await api.post<Session>(`/api/gym/sessions/${id}/finish`)
    return res.data
  }

  async function deleteSession(id: number): Promise<void> {
    await api.delete(`/api/gym/sessions/${id}`)
  }

  async function addSessionExercise(sessionId: number, exerciseId: number): Promise<Session> {
    const res = await api.post<Session>(`/api/gym/sessions/${sessionId}/exercises`, { exerciseId })
    return res.data
  }

  async function removeSessionExercise(sessionId: number, sessionExerciseId: number): Promise<Session> {
    const res = await api.delete<Session>(`/api/gym/sessions/${sessionId}/exercises/${sessionExerciseId}`)
    return res.data
  }

  /** Replaces the whole set list for one exercise; order assigns set numbers. */
  async function replaceSets(
    sessionId: number,
    sessionExerciseId: number,
    sets: SetInput[],
  ): Promise<Session> {
    const res = await api.put<Session>(
      `/api/gym/sessions/${sessionId}/exercises/${sessionExerciseId}/sets`,
      { sets },
    )
    return res.data
  }

  return {
    listExercises,
    listExerciseFilters,
    createExercise,
    updateExercise,
    updateExerciseDescription,
    deleteExercise,
    lastSets,
    loggedExercises,
    exerciseProgress,
    progressSummary,
    listRoutines,
    getRoutine,
    createRoutine,
    updateRoutine,
    deleteRoutine,
    listSessions,
    activeSession,
    getSession,
    createSession,
    updateSession,
    finishSession,
    deleteSession,
    addSessionExercise,
    removeSessionExercise,
    replaceSets,
  }
}
