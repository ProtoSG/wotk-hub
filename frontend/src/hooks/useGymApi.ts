import api from '@/lib/axios'
import type {
  ExerciseFilters,
  ExerciseFilterValues,
  ExerciseListResult,
} from '@/types/gym.types'

export function useGymApi() {
  async function listExercises(filters: ExerciseFilters = {}): Promise<ExerciseListResult> {
    const res = await api.get<ExerciseListResult>('/api/gym/exercises', { params: filters })
    return res.data
  }

  async function listExerciseFilters(): Promise<ExerciseFilterValues> {
    const res = await api.get<ExerciseFilterValues>('/api/gym/exercises/filters')
    return res.data
  }

  return { listExercises, listExerciseFilters }
}
