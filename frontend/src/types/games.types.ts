export type MovieDifficulty = 'easy' | 'medium' | 'hard'

export type SessionStatus = 'waiting' | 'active' | 'finished'

export interface EmojiPuzzle {
  id: number
  emojiStr: string
  difficulty: MovieDifficulty
}

export interface EmojiGameSession {
  id: number
  player1Id: number
  player2Id?: number
  p1Score: number
  p2Score: number
  currentEmoji: string
  status: SessionStatus
  createdAt: string
}

export interface GuessResult {
  correct: boolean
  session: EmojiGameSession
}

export interface RevealResult {
  answer: string
  session: EmojiGameSession
}

// ─── Riddle Game ─────────────────────────────────────────────────────────────

export type RiddleDifficulty = 'easy' | 'medium' | 'hard'
export type RiddleStatus = 'active' | 'solved' | 'expired' | 'gameover'

export interface DailyRiddle {
  id: number
  question: string
  hint: string
  difficulty: RiddleDifficulty
  publishedOn: string
  createdAt: string
}

export interface RiddleGameSession {
  id: number
  teamId: number
  partnerId: number
  livesRemaining: number
  p1Score: number
  p2Score: number
  currentRiddleId?: number
  status: RiddleStatus
  createdAt: string
}

export interface RiddleAttempt {
  id: number
  sessionId: number
  riddleId: number
  solverId: number
  solvedAt: string
  pointsEarned: number
}

export interface RiddleGuessResult {
  correct: boolean
  pointsEarned: number
  session: RiddleGameSession
}

export interface RiddleHistoryItem {
  riddleId: number
  question: string
  answer: string
  solvedBy: string
  solvedAt: string
  pointsEarned: number
  expired: boolean
}
