import api from '@/lib/axios'
import type { CoupleDate, CoupleDateInput, CoupleDatePhoto, GalleryPhoto } from '@/types/couple.types'

export function useCoupleApi() {
  async function listDates(): Promise<CoupleDate[]> {
    const res = await api.get<{ dates: CoupleDate[] }>('/api/couple/dates')
    return res.data.dates
  }

  async function createDate(input: CoupleDateInput): Promise<CoupleDate> {
    const res = await api.post<CoupleDate>('/api/couple/dates', input)
    return res.data
  }

  async function updateDate(id: number, input: CoupleDateInput): Promise<CoupleDate> {
    const res = await api.put<CoupleDate>(`/api/couple/dates/${id}`, input)
    return res.data
  }

  async function deleteDate(id: number): Promise<void> {
    await api.delete(`/api/couple/dates/${id}`)
  }

  async function listPhotos(dateId: number): Promise<CoupleDatePhoto[]> {
    const res = await api.get<{ photos: CoupleDatePhoto[] }>(`/api/couple/dates/${dateId}/photos`)
    return res.data.photos
  }

  async function uploadPhoto(dateId: number, file: File): Promise<CoupleDatePhoto> {
    const formData = new FormData()
    formData.append('photo', file)
    const res = await api.post<CoupleDatePhoto>(`/api/couple/dates/${dateId}/photos`, formData)
    return res.data
  }

  async function deletePhoto(dateId: number, photoId: number): Promise<void> {
    await api.delete(`/api/couple/dates/${dateId}/photos/${photoId}`)
  }

  async function listGallery(): Promise<GalleryPhoto[]> {
    const res = await api.get<{ photos: GalleryPhoto[] }>('/api/couple/photos')
    return res.data.photos
  }

  return { listDates, createDate, updateDate, deleteDate, listPhotos, uploadPhoto, deletePhoto, listGallery }
}
