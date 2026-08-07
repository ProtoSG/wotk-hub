import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Heart, Lightbulb, Clock, Trophy, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useGamesApi } from '@/hooks/useGamesApi'
import { useAuthStore } from '@/store/authStore'
import type { DailyRiddle, RiddleGameSession, RiddleGuessResult } from '@/types/games.types'

const POLL_INTERVAL_MS = 2000

const DIFFICULTY_LABELS: Record<string, string> = {
  easy: 'Fácil',
  medium: 'Media',
  hard: 'Difícil',
}

const POINTS_LABELS: Record<number, string> = {
  100: '¡100 pts! ⚡',
  75: '¡75 pts! 🔥',
  60: '¡60 pts! ✨',
  50: '¡50 pts! 🎉',
}

// Countdown from 24 hours (time remaining for today's riddle)
function useCountdown(publishedOn: string) {
  const [timeLeft, setTimeLeft] = useState('')
  useEffect(() => {
    const deadline = new Date(publishedOn)
    deadline.setHours(deadline.getHours() + 24)
    const tick = () => {
      const diff = deadline.getTime() - Date.now()
      if (diff <= 0) {
        setTimeLeft('00:00:00')
        return
      }
      const h = Math.floor(diff / 3_600_000)
      const m = Math.floor((diff % 3_600_000) / 60_000)
      const s = Math.floor((diff % 60_000) / 1_000)
      setTimeLeft(`${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`)
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [publishedOn])
  return timeLeft
}

function Hearts({ lives }: { lives: number }) {
  return (
    <div className="flex gap-1">
      {[0, 1, 2].map((i) => (
        <Heart key={i} className={`h-5 w-5 ${i < lives ? 'fill-red-500 text-red-500' : 'text-muted'}`} />
      ))}
    </div>
  )
}

export default function UltimaPreguntaTab() {
  const { getRiddleToday, getRiddleSession, submitRiddleGuess, getRiddleHistory } = useGamesApi()
  const userId = useAuthStore((s) => s.user?.id)

  const [riddle, setRiddle] = useState<DailyRiddle | null>(null)
  const [session, setSession] = useState<RiddleGameSession | null>(null)
  const [history, setHistory] = useState<{ riddleId: number; question: string; answer: string; solvedBy: string; solvedAt: string; pointsEarned: number; expired: boolean }[]>([])
  const [guessText, setGuessText] = useState('')
  const [feedback, setFeedback] = useState<'correct' | 'wrong' | null>(null)
  const [showHint, setShowHint] = useState(false)
  const [busy, setBusy] = useState(false)
  const [view, setView] = useState<'play' | 'history'>('play')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Must be called before any early returns — hooks can't be conditional
  const countdown = useCountdown(riddle?.publishedOn ?? new Date().toISOString().split('T')[0])

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  // Poll session for live updates — no websockets, same pattern as EmojiMoviesTab.
  const startPolling = useCallback(() => {
    stopPolling()
    pollRef.current = setInterval(async () => {
      try {
        const { session: s } = await getRiddleSession()
        setSession(s)
        if (s?.status === 'active' || s?.status === 'solved' || s?.status === 'gameover') {
          stopPolling()
        }
      } catch {
        // transient poll failure — next tick retries
      }
    }, POLL_INTERVAL_MS)
  }, [getRiddleSession, stopPolling])

  useEffect(() => () => stopPolling(), [stopPolling])

  // Initial load
  useEffect(() => {
    ;(async () => {
      try {
        const [{ riddle: r }, { session: s }] = await Promise.all([getRiddleToday(), getRiddleSession()])
        setRiddle(r)
        setSession(s)
        if (!s || s.status === 'active') {
          startPolling()
        }
      } catch {
        toast.error('No se pudo cargar el juego')
      }
    })()
  }, [getRiddleToday, getRiddleSession, startPolling])

  async function handleGuess() {
    if (!riddle || !guessText.trim()) return
    setBusy(true)
    try {
      const result = await submitRiddleGuess(guessText)
      handleResult(result)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo enviar la respuesta')
    } finally {
      setBusy(false)
    }
  }

  function handleResult(result: RiddleGuessResult) {
    setFeedback(result.correct ? 'correct' : 'wrong')
    setSession(result.session)
    if (result.correct) {
      setGuessText('')
      toast.success(POINTS_LABELS[result.pointsEarned] ?? `¡Correcto! +${result.pointsEarned} pts`)
      stopPolling()
      // Auto-advance after 5s
      setTimeout(() => {
        setFeedback(null)
        reloadToday()
      }, 5000)
    } else {
      setTimeout(() => setFeedback(null), 1500)
    }
  }

  async function reloadToday() {
    try {
      const [{ riddle: r }, { session: s }] = await Promise.all([getRiddleToday(), getRiddleSession()])
      setRiddle(r)
      setSession(s)
      if (s?.status === 'active') startPolling()
    } catch {
      // ignore
    }
  }

  async function handleStartGame() {
    setBusy(true)
    try {
      const { session: s } = await getRiddleSession()
      setSession(s)
      if (s?.status === 'active') startPolling()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo iniciar')
    } finally {
      setBusy(false)
    }
  }

  async function handleViewHistory() {
    try {
      const { history: h } = await getRiddleHistory()
      setHistory(h)
      setView('history')
    } catch {
      toast.error('No se pudo cargar el historial')
    }
  }

  // ── Derive UI state ──────────────────────────────────────────────────────────
  if (view === 'history') {
    return (
      <Card className="mx-auto max-w-md">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Historial</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setView('play')}>
              Volver
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {history.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-8">Aún no hay historial.</p>
          ) : (
            history.map((h) => (
              <div key={h.riddleId} className="rounded-lg border p-3 text-sm">
                <p className="font-medium">{h.question}</p>
                <p className="text-muted-foreground mt-1">Respuesta: {h.answer}</p>
                <p className="text-xs text-muted-foreground mt-1">
                  {h.solvedBy} · {POINTS_LABELS[h.pointsEarned] ?? `${h.pointsEarned} pts`}
                </p>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    )
  }

  // No session — show lives and scores CTA
  if (!session || session.status === 'gameover') {
    return (
      <Card className="mx-auto max-w-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <AlertCircle className="h-5 w-5 text-red-500" />
            {session?.status === 'gameover' ? '¡Perdieron!' : 'La Última Pregunta'}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          {session?.status === 'gameover' && (
            <p className="text-center text-muted-foreground">Se acabaron las vidas. 😢</p>
          )}
          <div className="flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-sm text-muted-foreground">Vidas</span>
              <Hearts lives={session?.livesRemaining ?? 3} />
            </div>
            <div className="flex flex-col gap-1 text-center">
              <span className="text-sm text-muted-foreground">Pareja 1</span>
              <span className="text-xl font-bold">{session?.p1Score ?? 0} pts</span>
            </div>
            <div className="flex flex-col gap-1 text-center">
              <span className="text-sm text-muted-foreground">Pareja 2</span>
              <span className="text-xl font-bold">{session?.p2Score ?? 0} pts</span>
            </div>
          </div>
          <div className="space-y-2">
            <Button onClick={handleStartGame} disabled={busy} className="w-full">
              Resolver la pregunta de hoy
            </Button>
            <Button variant="ghost" size="sm" onClick={handleViewHistory} className="w-full">
              Ver historial
            </Button>
          </div>
        </CardContent>
      </Card>
    )
  }

  // Riddle is solved (by either partner)
  if (session.status === 'solved') {
    return (
      <Card className="mx-auto max-w-md border-green-500">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-green-600">
            <Trophy className="h-5 w-5" />
            ¡La resolvieron!
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg bg-green-50 p-4 text-center dark:bg-green-950">
            <p className="text-sm text-muted-foreground mb-2">La respuesta era:</p>
            <p className="text-xl font-bold text-green-700 dark:text-green-400">{riddle?.hint.split('.')[0] ?? ''}</p>
          </div>
          <div className="flex items-center justify-between text-sm">
            <Hearts lives={session.livesRemaining} />
            <span>P1: {session.p1Score} pts</span>
            <span>P2: {session.p2Score} pts</span>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={handleViewHistory} className="flex-1">
              Historial
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setView('play')
                reloadToday()
              }}
              className="flex-1"
            >
              Nueva pregunta
            </Button>
          </div>
        </CardContent>
      </Card>
    )
  }

  // Expired (24h passed, no one solved — loses a life)
  if (session.status === 'expired') {
    return (
      <Card className="mx-auto max-w-md border-yellow-500">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-yellow-600">
            <Clock className="h-5 w-5" />
            Se acabó el tiempo
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg bg-yellow-50 p-4 text-center dark:bg-yellow-950">
            <p className="text-sm text-muted-foreground mb-2">La respuesta era:</p>
            <p className="text-xl font-bold text-yellow-700 dark:text-yellow-400">{riddle?.hint.split('.')[0] ?? ''}</p>
          </div>
          <div className="flex items-center justify-center gap-2 text-yellow-600">
            <Heart className="h-4 w-4 fill-yellow-500" />
            <span>-1 vida</span>
          </div>
          <Hearts lives={session.livesRemaining} />
          <Button onClick={() => reloadToday()} className="w-full">
            Siguiente pregunta
          </Button>
        </CardContent>
      </Card>
    )
  }

  // Active daily riddle
  const isP1 = userId === session.teamId

  return (
    <Card className="mx-auto max-w-md">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>La Última Pregunta</CardTitle>
          <Hearts lives={session.livesRemaining} />
        </div>
        <div className="flex items-center justify-between text-xs text-muted-foreground mt-1">
          <span>
            {riddle ? DIFFICULTY_LABELS[riddle.difficulty] : ''}
          </span>
          <span className="flex items-center gap-1">
            <Clock className="h-3 w-3" />
            {countdown}
          </span>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Score bar */}
        <div className="flex items-center justify-between rounded-lg bg-muted px-4 py-2 text-sm">
          <span className={isP1 ? 'font-bold text-primary' : 'text-muted-foreground'}>
            P1: {session.p1Score} pts
          </span>
          <span className="text-muted-foreground">vs</span>
          <span className={!isP1 ? 'font-bold text-primary' : 'text-muted-foreground'}>
            P2: {session.p2Score} pts
          </span>
        </div>

        {/* Question */}
        {riddle && (
          <div className="rounded-lg bg-card border p-4 text-center">
            <p className="text-lg font-medium">{riddle.question}</p>
          </div>
        )}

        {/* Hint toggle */}
        {riddle?.hint && (
          <div className="space-y-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowHint((v) => !v)}
              className="w-full flex items-center gap-2"
            >
              <Lightbulb className="h-4 w-4" />
              {showHint ? 'Ocultar pista' : 'Mostrar pista'}
            </Button>
            {showHint && (
              <p className="rounded bg-yellow-50 p-2 text-sm text-yellow-800 dark:bg-yellow-950 dark:text-yellow-400">
                💡 {riddle.hint}
              </p>
            )}
          </div>
        )}

        {/* Feedback */}
        {feedback && (
          <p className={`text-center font-medium ${feedback === 'correct' ? 'text-green-600' : 'text-red-500'}`}>
            {feedback === 'correct' ? '¡Correcto! 🎉' : 'Nop, otra vez'}
          </p>
        )}

        {/* Guess input */}
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
            Enviar
          </Button>
        </form>

        {/* History link */}
        <Button variant="ghost" size="sm" onClick={handleViewHistory} className="w-full">
          Ver historial
        </Button>
      </CardContent>
    </Card>
  )
}
