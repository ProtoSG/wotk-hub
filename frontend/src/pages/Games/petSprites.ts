import type { PetMood } from '@/types/pet.types'

// Split out of MascotaTab.tsx so both it and PetChat.tsx can import this
// mapping without re-declaring it — a plain .ts module also sidesteps the
// react-refresh/only-export-components lint rule that fires when a
// component file (.tsx) exports a non-component constant.
export const MOOD_SPRITE: Record<PetMood, string> = {
  happy: '/pet/happy.gif',
  neutral: '/pet/neutral.gif',
  sad: '/pet/sad.gif',
  hungry: '/pet/hungry.gif',
}
