import { Car, GraduationCap, Home, PiggyBank, Plane, Target, type LucideIcon } from 'lucide-react'

export const GOAL_ICON_OPTIONS = [
  { value: 'piggy-bank', label: 'Ahorro' },
  { value: 'target', label: 'Meta' },
  { value: 'plane', label: 'Viaje' },
  { value: 'home', label: 'Casa' },
  { value: 'car', label: 'Auto' },
  { value: 'graduation-cap', label: 'Educación' },
]

// goal.icon only ever holds one of GOAL_ICON_OPTIONS' values — MetasTab uses
// this to render the icon the user actually picked instead of a fixed one
// for every goal.
export const GOAL_ICON_MAP: Record<string, LucideIcon> = {
  'piggy-bank': PiggyBank,
  target: Target,
  plane: Plane,
  home: Home,
  car: Car,
  'graduation-cap': GraduationCap,
}
