import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Trophy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useGamesApi } from '@/hooks/useGamesApi'
import { useAuthStore } from '@/store/authStore'
import type { EmojiGameSession, MovieDifficulty } from '@/types/games.types'

const POLL_INTERVAL_MS = 2000

const DIFFICULTIES: { value: MovieDifficulty; label: string }[] = [
  { value: 'easy', label: 'Fácil' },
  { value: 'medium', label: 'Media' },
  { value: 'hard', label: 'Difícil' },
]

export default function EmojiMoviesTab() {
  const { createSession, joinSession, getSession, guess, reveal } = useGamesApi()
  const userId = useAuthStore((s) => s.user?.id)

  const [session, setSession] = useState<EmojiGameSession | null>(null)
  const [joinId, setJoinId] = useState('')
  const [guessText, setGuessText] = useState('')
  const [feedback, setFeedback] = useState<'correct' | 'wrong' | null>(null)
  const [difficulty, setDifficulty] = useState<MovieDifficulty | ''>('')
  const [busy, setBusy] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  // No websockets in this app — poll the shared session so both players'
  // screens converge on join/opponent guesses without a manual refresh.
  const startPolling = useCallback(
    (id: number) => {
      stopPolling()
      pollRef.current = setInterval(async () => {
        try {
          const s = await getSession(id)
          setSession(s)
          if (s.status === 'finished') stopPolling()
        } catch {
          // transient poll failure — next tick retries, nothing to show the user
        }
      }, POLL_INTERVAL_MS)
    },
    [getSession, stopPolling]
  )

  useEffect(() => stopPolling, [stopPolling])

  async function handleCreate() {
    setBusy(true)
    try {
      const s = await createSession(difficulty || undefined)
      setSession(s)
      startPolling(s.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo crear la partida')
    } finally {
      setBusy(false)
    }
  }

  async function handleJoin() {
    const id = Number(joinId)
    if (!id || id <= 0) {
      toast.error('Ingresá un ID de partida válido')
      return
    }
    setBusy(true)
    try {
      const s = await joinSession(id)
      setSession(s)
      startPolling(s.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo unir a la partida')
    } finally {
      setBusy(false)
    }
  }

  async function handleGuess() {
    if (!session || !guessText.trim()) return
    setBusy(true)
    try {
      const result = await guess(session.id, guessText)
      setSession(result.session)
      setFeedback(result.correct ? 'correct' : 'wrong')
      if (result.correct) setGuessText('')
      if (result.session.status === 'finished') stopPolling()
      setTimeout(() => setFeedback(null), 1500)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo enviar la respuesta')
    } finally {
      setBusy(false)
    }
  }

  async function handleReveal() {
    if (!session) return
    setBusy(true)
    try {
      const result = await reveal(session.id)
      toast.info(`La respuesta era: ${result.answer}`)
      setSession(result.session)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo revelar la respuesta')
    } finally {
      setBusy(false)
    }
  }

  function handleNewGame() {
    stopPolling()
    setSession(null)
    setGuessText('')
    setFeedback(null)
  }

  if (!session) {
    return (
      <Card className="mx-auto max-w-md">
        <CardHeader>
          <CardTitle>Emoji Movies</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">
              Adiviná la película a partir de los emojis. Jugás contra otra persona.
            </p>
            <div className="flex gap-2">
              {DIFFICULTIES.map((d) => (
                <Button
                  key={d.value}
                  type="button"
                  variant={difficulty === d.value ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setDifficulty(difficulty === d.value ? '' : d.value)}
                >
                  {d.label}
                </Button>
              ))}
            </div>
            <Button onClick={handleCreate} disabled={busy} className="w-full">
              Crear partida
            </Button>
          </div>

          <div className="space-y-2 border-t pt-4">
            <Input placeholder="ID de la partida" value={joinId} onChange={(e) => setJoinId(e.target.value)} inputMode="numeric" />
            <Button onClick={handleJoin} disabled={busy} variant="secondary" className="w-full">
              Unirse
            </Button>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (session.status === 'waiting') {
    return (
      <Card className="mx-auto max-w-md">
        <CardHeader>
          <CardTitle>Esperando jugador...</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 text-center">
          <p className="text-sm text-muted-foreground">Compartí este ID con la otra persona para que se una:</p>
          <p className="text-3xl font-bold">{session.id}</p>
          <Button variant="outline" onClick={handleNewGame}>
            Cancelar
          </Button>
        </CardContent>
      </Card>
    )
  }

  if (session.status === 'finished') {
    const winner =
      session.p1Score > session.p2Score ? 'Jugador 1' : session.p2Score > session.p1Score ? 'Jugador 2' : 'Empate'
    return (
      <Card className="mx-auto max-w-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Trophy className="h-5 w-5 text-yellow-500" />
            ¡Partida terminada!
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 text-center">
          <p className="text-xl font-semibold">{winner === 'Empate' ? 'Empate' : `Ganó ${winner}`}</p>
          <p className="text-sm text-muted-foreground">
            {session.p1Score} - {session.p2Score}
          </p>
          <Button onClick={handleNewGame} className="w-full">
            Nueva partida
          </Button>
        </CardContent>
      </Card>
    )
  }

  const isPlayer1 = userId === session.player1Id

  return (
    <Card className="mx-auto max-w-md">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Emoji Movies</CardTitle>
          <div className="flex items-center gap-3 text-sm font-medium">
            <span className={isPlayer1 ? 'text-primary' : 'text-muted-foreground'}>P1: {session.p1Score}</span>
            <span className="text-muted-foreground">vs</span>
            <span className={!isPlayer1 ? 'text-primary' : 'text-muted-foreground'}>P2: {session.p2Score}</span>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="rounded-lg bg-muted py-10 text-center text-6xl">{session.currentEmoji}</div>

        {feedback && (
          <p className={`text-center font-medium ${feedback === 'correct' ? 'text-success' : 'text-destructive'}`}>
            {feedback === 'correct' ? '¡Correcto! 🎉' : 'Nop, otra vez'}
          </p>
        )}

        <form
          onSubmit={(e) => {
            e.preventDefault()
            handleGuess()
          }}
          className="flex gap-2"
        >
          <Input
            value={guessText}
            onChange={(e) => setGuessText(e.target.value)}
            placeholder="Tu respuesta..."
            disabled={busy}
            autoFocus
          />
          <Button type="submit" disabled={busy || !guessText.trim()}>
            Adivinar
          </Button>
        </form>

        <Button variant="ghost" size="sm" onClick={handleReveal} disabled={busy} className="w-full">
          Revelar respuesta
        </Button>
      </CardContent>
    </Card>
  )
}
