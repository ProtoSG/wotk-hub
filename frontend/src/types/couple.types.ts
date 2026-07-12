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
