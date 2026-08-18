import {
  Briefcase,
  Car,
  Film,
  Gift,
  GraduationCap,
  HeartPulse,
  Home,
  Laptop,
  Repeat,
  Shirt,
  Tag,
  TrendingUp,
  Utensils,
  Zap,
  type LucideIcon,
} from 'lucide-react'

// Exact matches for the categories seeded by default (see
// backend/store/migrate.go's default_categories insert) — covers what most
// users will actually have without ever touching Categorías.
const CATEGORY_ICONS: Record<string, LucideIcon> = {
  comida: Utensils,
  transporte: Car,
  vivienda: Home,
  servicios: Zap,
  salud: HeartPulse,
  educacion: GraduationCap,
  entretenimiento: Film,
  ropa: Shirt,
  suscripciones: Repeat,
  sueldo: Briefcase,
  freelance: Laptop,
  inversiones: TrendingUp,
  regalo: Gift,
}

// Loose keyword fallback for categories the user renamed or added — names
// are free text (see Category.name in finance.types.ts), so this can't be
// exhaustive. Falls through to a generic Tag when nothing matches.
const KEYWORD_ICONS: [string, LucideIcon][] = [
  ['casa', Home],
  ['renta', Home],
  ['alquiler', Home],
  ['auto', Car],
  ['carro', Car],
  ['moto', Car],
  ['uber', Car],
  ['taxi', Car],
  ['comida', Utensils],
  ['restaurant', Utensils],
  ['super', Utensils],
  ['salud', HeartPulse],
  ['medic', HeartPulse],
  ['doctor', HeartPulse],
  ['gym', HeartPulse],
  ['estudio', GraduationCap],
  ['curso', GraduationCap],
  ['cine', Film],
  ['streaming', Film],
  ['ropa', Shirt],
  ['regalo', Gift],
  ['sueldo', Briefcase],
  ['trabajo', Briefcase],
  ['invers', TrendingUp],
  ['ahorro', TrendingUp],
]

export function getCategoryIcon(categoryName: string): LucideIcon {
  const key = categoryName.trim().toLowerCase()
  if (CATEGORY_ICONS[key]) return CATEGORY_ICONS[key]
  const match = KEYWORD_ICONS.find(([kw]) => key.includes(kw))
  return match ? match[1] : Tag
}

// Same validated categorical palette CategoryChart.tsx uses for the
// spend-breakdown pie (--chart-1..8, colorblind-checked — see that file's
// comment). Picked here by a stable hash of the category name instead of
// sort position, so a given category always lands on the same color
// wherever it shows up, not just inside that one chart.
const CHART_ACCENT_COUNT = 8

export function getCategoryAccent(categoryName: string): string {
  const key = categoryName.trim().toLowerCase()
  let hash = 0
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) >>> 0
  }
  return `--chart-${(hash % CHART_ACCENT_COUNT) + 1}`
}
