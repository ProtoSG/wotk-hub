import api from '@/lib/axios'
import type { CoupleDate, CoupleDateInput } from '@/types/couple.types'

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

  return { listDates, createDate, updateDate, deleteDate }
}
