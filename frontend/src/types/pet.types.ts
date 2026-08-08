export type PetMood = 'happy' | 'neutral' | 'sad' | 'hungry'
export type CareAction = 'bathe' | 'breakfast' | 'lunch' | 'play' | 'dinner'

export interface PetActionStatus {
  done: boolean
  by: string | null
  locked: boolean
  unlocksAtHour: number
}

export interface PetState {
  careScore: number
  mood: PetMood
  streak: number
  sparks: number
  streakFreezes: number
  perfectDay: boolean
  bathe: PetActionStatus
  breakfast: PetActionStatus
  lunch: PetActionStatus
  play: PetActionStatus
  dinner: PetActionStatus
}
