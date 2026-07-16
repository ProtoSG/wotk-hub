export const EXPENSE_CATEGORIES = [
  'comida',
  'transporte',
  'vivienda',
  'servicios',
  'salud',
  'educacion',
  'entretenimiento',
  'ropa',
  'suscripciones',
  'otros',
] as const

export const INCOME_CATEGORIES = ['sueldo', 'freelance', 'inversiones', 'regalo', 'otros'] as const

export type ExpenseCategory = (typeof EXPENSE_CATEGORIES)[number]
export type IncomeCategory = (typeof INCOME_CATEGORIES)[number]

export const CATEGORY_LABELS: Record<string, string> = {
  comida: 'Comida',
  transporte: 'Transporte',
  vivienda: 'Vivienda',
  servicios: 'Servicios',
  salud: 'Salud',
  educacion: 'Educación',
  entretenimiento: 'Entretenimiento',
  ropa: 'Ropa',
  suscripciones: 'Suscripciones',
  otros: 'Otros',
  sueldo: 'Sueldo',
  freelance: 'Freelance',
  inversiones: 'Inversiones',
  regalo: 'Regalo',
}

export type TransactionType = 'income' | 'expense'
export type Frequency = 'weekly' | 'monthly' | 'yearly'

export const FREQUENCY_LABELS: Record<Frequency, string> = {
  weekly: 'Semanal',
  monthly: 'Mensual',
  yearly: 'Anual',
}

export interface Transaction {
  id: number
  type: TransactionType
  amountCents: number
  category: string
  description: string
  date: string // YYYY-MM-DD
  createdAt: string
}

export interface TransactionInput {
  type: TransactionType
  amountCents: number
  category: string
  description: string
  date: string
}

export interface Subscription {
  id: number
  name: string
  amountCents: number
  frequency: Frequency
  category: string
  nextBillingOn: string // YYYY-MM-DD
  active: boolean
  createdAt: string
}

export interface SubscriptionInput {
  name: string
  amountCents: number
  frequency: Frequency
  category: string
  nextBillingOn: string
  active: boolean
}

export interface Budget {
  id: number
  category: string
  monthlyLimitCents: number
  spentCents: number
}

export interface TrendPoint {
  month: string // YYYY-MM
  incomeCents: number
  expenseCents: number
}

export interface CategoryAmount {
  category: string
  amountCents: number
}

export interface FinanceSummary {
  balanceCents: number
  monthIncomeCents: number
  monthExpenseCents: number
  monthlyTrend: TrendPoint[]
  categoryBreakdown: CategoryAmount[]
}

export type CardType = 'debito' | 'credito' | 'prepago'

export const CARD_TYPE_LABELS: Record<CardType, string> = {
  debito: 'Débito',
  credito: 'Crédito',
  prepago: 'Prepago',
}

export interface Card {
  id: number
  name: string
  type: CardType
  bank: string
  last4: string
  color: string
  icon: string
  balanceCents: number
  createdAt: string
}

export interface CardInput {
  name: string
  type: CardType
  bank: string
  last4: string
  color: string
  icon: string
}

export interface CardReload {
  id: number
  cardId: number
  amountCents: number
  date: string
  note: string
  createdAt: string
}

export interface CardReloadInput {
  amountCents: number
  date: string
  note: string
}

export interface TransactionFilters {
  month?: string
  type?: TransactionType
  category?: string
}
