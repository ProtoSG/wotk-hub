import { useCallback } from 'react'
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

// Every function below is wrapped in useCallback with an empty dep array —
// `api` is a stable module-level import, so nothing here legitimately
// changes across renders. Without this, callers get a fresh function
// reference every render; any effect that lists one of these in its deps
// (e.g. UltimaPreguntaTab's initial-load effect) never stabilizes and
// re-fires on every render, which was silently hammering the API in a tight
// render loop — no polling interval involved.
export function useGamesApi() {
  const randomMovie = useCallback(async (difficulty?: MovieDifficulty): Promise<EmojiPuzzle> => {
    const res = await api.get<EmojiPuzzle>('/api/games/emoji-movies/random', {
      params: difficulty ? { difficulty } : undefined,
    })
    return res.data
  }, [])

  const createSession = useCallback(async (movieDifficulty?: MovieDifficulty): Promise<EmojiGameSession> => {
    const res = await api.post<EmojiGameSession>('/api/games/sessions', {
      movieDifficulty: movieDifficulty ?? '',
    })
    return res.data
  }, [])

  const getSession = useCallback(async (id: number): Promise<EmojiGameSession> => {
    const res = await api.get<EmojiGameSession>(`/api/games/sessions/${id}`)
    return res.data
  }, [])

  const joinSession = useCallback(async (id: number): Promise<EmojiGameSession> => {
    const res = await api.post<EmojiGameSession>(`/api/games/sessions/${id}/join`)
    return res.data
  }, [])

  const guess = useCallback(async (id: number, guessText: string): Promise<GuessResult> => {
    const res = await api.post<GuessResult>(`/api/games/sessions/${id}/guess`, { guess: guessText })
    return res.data
  }, [])

  const reveal = useCallback(async (id: number): Promise<RevealResult> => {
    const res = await api.post<RevealResult>(`/api/games/sessions/${id}/reveal`)
    return res.data
  }, [])

  const activeSessions = useCallback(async (): Promise<EmojiGameSession[]> => {
    const res = await api.get<{ sessions: EmojiGameSession[] }>('/api/games/sessions/active')
    return res.data.sessions
  }, [])

  // ─── Riddle Game ─────────────────────────────────────────────────────────────

  const getRiddleToday = useCallback(async (): Promise<{ riddle: DailyRiddle | null }> => {
    const res = await api.get<{ riddle: DailyRiddle | null }>('/api/games/riddle/today')
    return res.data
  }, [])

  const getRiddleSession = useCallback(async (): Promise<{ session: RiddleGameSession | null }> => {
    const res = await api.get<{ session: RiddleGameSession | null }>('/api/games/riddle/session')
    return res.data
  }, [])

  const submitRiddleGuess = useCallback(async (guessText: string): Promise<RiddleGuessResult> => {
    const res = await api.post<RiddleGuessResult>('/api/games/riddle/guess', { guess: guessText })
    return res.data
  }, [])

  const getRiddleHistory = useCallback(async (): Promise<{ history: RiddleHistoryItem[] }> => {
    const res = await api.get<{ history: RiddleHistoryItem[] }>('/api/games/riddle/history')
    return res.data
  }, [])

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
