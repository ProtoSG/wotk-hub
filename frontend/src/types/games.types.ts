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
  // Absolute instant (ISO) this riddle's 24h window closes, computed
  // server-side in Lima local time — don't re-derive a deadline from
  // publishedOn client-side, that's what caused the countdown to be off by
  // exactly the Lima/UTC offset (5h) before this field existed.
  expiresAt: string
  // Weekly "doble o nada" riddle (every Sunday, Lima time) — a wrong guess
  // wipes the solver's score instead of the normal miss-has-no-cost rule.
  isBonus: boolean
}

export interface RiddleGameSession {
  id: number
  teamId: number
  partnerId: number
  livesRemaining: number
  p1Score: number
  p2Score: number
  // Consecutive days solved without a miss — resets to 0 the moment a day
  // expires unsolved.
  streak: number
  currentRiddleId?: number
  status: RiddleStatus
  createdAt: string
  // Only present once status is 'solved' or 'gameover' — backend never
  // sends it while active, so an in-progress poll can't leak it.
  answer?: string
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

// ─── Live updates (WebSocket) ──────────────────────────────────────────────

// Discriminated union of every event the games WS hub can push — see
// backend/modules/games/types.go's wsEmojiMoviesEvent/wsRiddleEvent (the
// `type` string literals must match those exactly) and
// hooks/useGameSocket.ts, which is the only place these are consumed.
export interface EmojiMoviesSessionEvent {
  type: 'emoji_movies.session'
  session: EmojiGameSession
}

export interface RiddleSessionEvent {
  type: 'riddle.session'
  session: RiddleGameSession
}

export type GameSocketEvent = EmojiMoviesSessionEvent | RiddleSessionEvent
