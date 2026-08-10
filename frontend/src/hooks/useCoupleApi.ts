import api from '@/lib/axios'
import type { CoupleDate, CoupleDateInput, CoupleDatePhoto, GalleryPhoto, Poem, PoemInput } from '@/types/couple.types'

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
    // Longer than the client's 30s default: the backend decodes, corrects
    // EXIF orientation, resizes two variants, and uploads both to MinIO
    // before responding — real work, not a typical fast JSON round trip,
    // and can run close to 30s on a modest VPS for a single large photo.
    const res = await api.post<CoupleDatePhoto>(`/api/couple/dates/${dateId}/photos`, formData, { timeout: 120000 })
    return res.data
  }

  async function deletePhoto(dateId: number, photoId: number): Promise<void> {
    await api.delete(`/api/couple/dates/${dateId}/photos/${photoId}`)
  }

  async function listGallery(): Promise<GalleryPhoto[]> {
    const res = await api.get<{ photos: GalleryPhoto[] }>('/api/couple/photos')
    return res.data.photos
  }

  async function listPoems(): Promise<Poem[]> {
    const res = await api.get<{ poems: Poem[] }>('/api/couple/poems')
    return res.data.poems
  }

  async function createPoem(input: PoemInput): Promise<Poem> {
    const res = await api.post<Poem>('/api/couple/poems', input)
    return res.data
  }

  async function deletePoem(id: number): Promise<void> {
    await api.delete(`/api/couple/poems/${id}`)
  }

  async function markPoemSeen(id: number): Promise<void> {
    await api.post(`/api/couple/poems/${id}/seen`)
  }

  return { listDates, createDate, updateDate, deleteDate, listPhotos, uploadPhoto, deletePhoto, listGallery, listPoems, createPoem, deletePoem, markPoemSeen }
}
