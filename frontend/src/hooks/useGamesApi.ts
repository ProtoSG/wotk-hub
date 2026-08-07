import api from '@/lib/axios'
import type {
  EmojiGameSession,
  EmojiPuzzle,
  GuessResult,
  MovieDifficulty,
  RevealResult,
  DailyRiddle,
  RiddleGameSession,
  RiddleGuessResult,
  RiddleHistoryItem,
} from '@/types/games.types'

export function useGamesApi() {
  async function randomMovie(difficulty?: MovieDifficulty): Promise<EmojiPuzzle> {
    const res = await api.get<EmojiPuzzle>('/api/games/emoji-movies/random', {
      params: difficulty ? { difficulty } : undefined,
    })
    return res.data
  }

  async function createSession(movieDifficulty?: MovieDifficulty): Promise<EmojiGameSession> {
    const res = await api.post<EmojiGameSession>('/api/games/sessions', {
      movieDifficulty: movieDifficulty ?? '',
    })
    return res.data
  }

  async function getSession(id: number): Promise<EmojiGameSession> {
    const res = await api.get<EmojiGameSession>(`/api/games/sessions/${id}`)
    return res.data
  }

  async function joinSession(id: number): Promise<EmojiGameSession> {
    const res = await api.post<EmojiGameSession>(`/api/games/sessions/${id}/join`)
    return res.data
  }

  async function guess(id: number, guessText: string): Promise<GuessResult> {
    const res = await api.post<GuessResult>(`/api/games/sessions/${id}/guess`, { guess: guessText })
    return res.data
  }

  async function reveal(id: number): Promise<RevealResult> {
    const res = await api.post<RevealResult>(`/api/games/sessions/${id}/reveal`)
    return res.data
  }

  async function activeSessions(): Promise<EmojiGameSession[]> {
    const res = await api.get<{ sessions: EmojiGameSession[] }>('/api/games/sessions/active')
    return res.data.sessions
  }

  // ─── Riddle Game ─────────────────────────────────────────────────────────────

  async function getRiddleToday(): Promise<{ riddle: DailyRiddle | null }> {
    const res = await api.get<{ riddle: DailyRiddle | null }>('/api/games/riddle/today')
    return res.data
  }

  async function getRiddleSession(): Promise<{ session: RiddleGameSession | null }> {
    const res = await api.get<{ session: RiddleGameSession | null }>('/api/games/riddle/session')
    return res.data
  }

  async function submitRiddleGuess(guess: string): Promise<RiddleGuessResult> {
    const res = await api.post<RiddleGuessResult>('/api/games/riddle/guess', { guess })
    return res.data
  }

  async function getRiddleHistory(): Promise<{ history: RiddleHistoryItem[] }> {
    const res = await api.get<{ history: RiddleHistoryItem[] }>('/api/games/riddle/history')
    return res.data
  }

  return {
    randomMovie,
    createSession,
    getSession,
    joinSession,
    guess,
    reveal,
    activeSessions,
    getRiddleToday,
    getRiddleSession,
    submitRiddleGuess,
    getRiddleHistory,
  }
}
