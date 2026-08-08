import { useCallback } from 'react'
import api from '@/lib/axios'
import type { PetState } from '@/types/pet.types'

// Every function wrapped in useCallback with an empty dep array — api is a
// stable module import, so nothing here legitimately changes across
// renders. Without this, any effect listing one of these in its deps never
// stabilizes and re-fires every render — see useGamesApi.ts for the actual
// incident this pattern fixed (a render loop hammering the API, found while
// debugging the riddle game earlier).
export function usePetApi() {
  const getPetState = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.get<{ pet: PetState }>('/api/pet')
    return res.data
  }, [])

  const bathePet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/bathe')
    return res.data
  }, [])

  const breakfastPet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/breakfast')
    return res.data
  }, [])

  const lunchPet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/lunch')
    return res.data
  }, [])

  const playWithPet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/play')
    return res.data
  }, [])

  const dinnerPet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/dinner')
    return res.data
  }, [])

  // Not admin-gated — a normal spend either partner can make, same as the 5
  // care actions (see pet.BuyFreeze).
  const buyStreakFreeze = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/shop/freeze')
    return res.data
  }, [])

  // Admin-only backend-side (see pet.Reset) — not just a frontend-hidden
  // button, since this discards real shared progress.
  const resetPet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/reset')
    return res.data
  }, [])

  return { getPetState, bathePet, breakfastPet, lunchPet, playWithPet, dinnerPet, buyStreakFreeze, resetPet }
}
