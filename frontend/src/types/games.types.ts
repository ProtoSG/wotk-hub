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
