import api from '@/lib/axios'
import type {
  EmojiGameSession,
  EmojiPuzzle,
  GuessResult,
  MovieDifficulty,
  RevealResult,
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

  return { randomMovie, createSession, getSession, joinSession, guess, reveal, activeSessions }
}
