import { useCallback } from 'react'
import api from '@/lib/axios'
import type { PetState } from '@/types/pet.types'

export interface PetPhoto {
  id: number
  url: string
  createdAt: string
}

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

  // Not admin-gated, same reasoning as buyStreakFreeze (see pet.Rename).
  const renamePet = useCallback(async (name: string): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/shop/rename', { name })
    return res.data
  }, [])

  // Admin-only backend-side (see pet.Reset) — not just a frontend-hidden
  // button, since this discards real shared progress.
  const resetPet = useCallback(async (): Promise<{ pet: PetState }> => {
    const res = await api.post<{ pet: PetState }>('/api/pet/reset')
    return res.data
  }, [])

  const uploadPetPhoto = useCallback(async (file: File): Promise<{ url: string }> => {
    const formData = new FormData()
    formData.append('photo', file)
    // Long timeout: camera captures are large and may be on slow connections.
    const res = await api.post<{ url: string }>('/api/pet/photos', formData, { timeout: 120000 })
    return res.data
  }, [])

  const listPetPhotos = useCallback(async (): Promise<PetPhoto[]> => {
    const res = await api.get<{ photos: PetPhoto[] }>('/api/pet/photos')
    return res.data.photos
  }, [])

  // Admin-only backend-side (see pet.DeletePetPhoto/ClearPetPhotos) — same
  // reasoning as resetPet, not just a frontend-hidden button.
  const deletePetPhoto = useCallback(async (photoId: number): Promise<void> => {
    await api.delete(`/api/pet/photos/${photoId}`)
  }, [])

  const clearPetPhotos = useCallback(async (): Promise<void> => {
    await api.delete('/api/pet/photos')
  }, [])

  return {
    getPetState,
    bathePet,
    breakfastPet,
    lunchPet,
    playWithPet,
    dinnerPet,
    buyStreakFreeze,
    renamePet,
    resetPet,
    uploadPetPhoto,
    listPetPhotos,
    deletePetPhoto,
    clearPetPhotos,
  }
}
