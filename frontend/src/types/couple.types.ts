export const DATE_CATEGORIES = [
  'cena',
  'almuerzo',
  'cine',
  'viaje',
  'aire_libre',
  'casa',
  'evento',
  'otro',
] as const

export type DateCategory = (typeof DATE_CATEGORIES)[number]

export const DATE_CATEGORY_LABELS: Record<string, string> = {
  cena: 'Cena',
  almuerzo: 'Almuerzo',
  cine: 'Cine',
  viaje: 'Viaje',
  aire_libre: 'Aire libre',
  casa: 'En casa',
  evento: 'Evento',
  otro: 'Otro',
}

export type DateStatus = 'planned' | 'done'

export interface CoupleDate {
  id: number
  occurredOn: string // YYYY-MM-DD
  place: string
  category: string
  notes: string
  costCents?: number
  rating?: number
  tiktokUrl: string
  status: DateStatus
  createdAt: string
}

export interface CoupleDateInput {
  occurredOn: string
  place: string
  category: string
  notes: string
  costCents?: number | null
  rating?: number | null
  tiktokUrl: string
  status: DateStatus
}

export interface CoupleDatePhoto {
  id: number
  url: string // freshly presigned MinIO GET URL (full-size), expires — don't cache long-term
  thumbnailUrl: string // freshly presigned MinIO GET URL (resized thumbnail), expires — don't cache long-term
  externalUrl?: string // optional external URL override; when set, takes precedence over MinIO URLs
  createdAt: string
}

// A photo plus enough of its parent date to group/label it in the
// cross-date Galería tab — see useCoupleApi.listGallery.
export interface GalleryPhoto extends CoupleDatePhoto {
  dateId: number
  dateOccurredOn: string // YYYY-MM-DD
  datePlace: string
}
