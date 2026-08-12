export type PetMood = 'happy' | 'neutral' | 'sad' | 'hungry'
export type CareAction = 'bathe' | 'breakfast' | 'lunch' | 'play' | 'dinner'

export interface PetActionStatus {
  done: boolean
  by: string | null
  locked: boolean
  // True once this action's window has closed (its deadline hour passed,
  // Lima time) and it's still undone today. Doesn't lock it out — still
  // tappable for the normal boost — but care_score already took the
  // missed-window penalty for it server-side; the frontend uses this just
  // to flag it instead of showing it as a plain still-open turn.
  missed: boolean
  unlocksAtHour: number
}

export interface PetState {
  careScore: number
  mood: PetMood
  // Empty until the shop's rename item is bought at least once.
  name: string
  streak: number
  sparks: number
  streakFreezes: number
  // True only in the one response where a freeze was just spent to protect
  // the streak — MascotaTab toasts this once instead of the spend being
  // silent (see backend petRow.freezeJustConsumed).
  freezeJustConsumed: boolean
  perfectDay: boolean
  bathe: PetActionStatus
  breakfast: PetActionStatus
  lunch: PetActionStatus
  play: PetActionStatus
  dinner: PetActionStatus
}
