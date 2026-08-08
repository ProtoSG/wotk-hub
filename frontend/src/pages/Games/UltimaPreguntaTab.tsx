import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Heart, Lightbulb, Clock, Trophy, AlertCircle, History, Flame, Dices } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard, paperSurfaceStyle } from '@/components/ui/cozy-card'
import { useGamesApi } from '@/hooks/useGamesApi'
import { useAuthStore } from '@/store/authStore'
import { cn } from '@/lib/utils'
import type { DailyRiddle, RiddleGameSession, RiddleGuessResult } from '@/types/games.types'

const POLL_INTERVAL_MS = 6000

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

// Countdown to expiresAt, an absolute instant the backend computes in Lima
// local time — deliberately not derived from publishedOn + 24h client-side.
// That used to parse publishedOn's date-only portion as UTC midnight and
// add 24h, which silently disagreed with the server's actual Lima-midnight
// day boundary by exactly the 5h UTC offset.
function useCountdown(expiresAt: string) {
  const [timeLeft, setTimeLeft] = useState('')
  useEffect(() => {
    const deadline = new Date(expiresAt)
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
  }, [expiresAt])
  return timeLeft
}

// Exact same treatment as Couple's DateCard rating hearts — fill-primary,
// not a red/destructive tone. Consistency with Couple won over a "danger"
// framing for lives.
function Hearts({ lives }: { lives: number }) {
  return (
    <div className="flex gap-1">
      {[0, 1, 2].map((i) => (
        <Heart
          key={i}
          size={16}
          className={cn(i < lives ? 'fill-primary text-primary' : 'text-muted-foreground/30')}
        />
      ))}
    </div>
  )
}

// Streak indicator — only rendered once there's something to show (0 reads
// as noise, not motivating). Primary-tinted, not --warning, so it doesn't
// visually collide with the hint/countdown's warning tone.
function StreakBadge({ streak }: { streak: number }) {
  if (streak <= 0) return null
  return (
    <div className="flex items-center gap-1 text-primary" title={`Racha de ${streak} día${streak === 1 ? '' : 's'}`}>
      <Flame className="h-4 w-4 fill-primary" />
      <span className="text-sm font-bold tabular-nums">{streak}</span>
    </div>
  )
}

// Small muted-label-over-bold-number readout — same pattern as
// Couple/EstadisticasTab's stat tiles and EmojiMoviesTab's score blocks.
function ScoreBlock({ label, score, lead }: { label: string; score: number; lead: boolean }) {
  return (
    <div className="text-center">
      <div className={cn('text-xl font-bold tabular-nums', lead ? 'text-primary' : 'text-muted-foreground')}>
        {score}
      </div>
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
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
  // Gates the guess input behind an explicit opt-in on bonus days — a wrong
  // guess wipes the score, so typing shouldn't be able to trigger that by
  // accident. Tracks WHICH riddle was confirmed (not a plain boolean) so a
  // new riddle loading automatically requires reconfirmation — no effect
  // needed to reset it.
  const [confirmedBonusRiddleId, setConfirmedBonusRiddleId] = useState<number | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Must be called before any early returns — hooks can't be conditional
  const countdown = useCountdown(riddle?.expiresAt ?? new Date().toISOString())

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  // Poll session for live updates — no websockets, same pattern as EmojiMoviesTab.
  // Keep polling while the session is 'active' (waiting on either partner);
  // stop only once it reaches a terminal state.
  const startPolling = useCallback(() => {
    stopPolling()
    pollRef.current = setInterval(async () => {
      try {
        const { session: s } = await getRiddleSession()
        setSession(s)
        if (s?.status === 'solved' || s?.status === 'gameover') {
          stopPolling()
        }
      } catch {
        // transient poll failure — next tick retries
      }
    }, POLL_INTERVAL_MS)
  }, [getRiddleSession, stopPolling])

  useEffect(() => () => stopPolling(), [stopPolling])

  // Pause polling while the tab isn't visible — no point burning requests
  // for a screen nobody's looking at. Resume on return if session is still live.
  useEffect(() => {
    function handleVisibility() {
      if (document.hidden) {
        stopPolling()
      } else if (session?.status === 'active') {
        startPolling()
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [session?.status, startPolling, stopPolling])

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
      const base = riddle?.isBonus
        ? `¡Doblaste tus puntos! 🎲 +${result.pointsEarned} pts`
        : (POINTS_LABELS[result.pointsEarned] ?? `¡Correcto! +${result.pointsEarned} pts`)
      toast.success(!riddle?.isBonus && result.session.streak > 1 ? `${base} · 🔥 racha de ${result.session.streak} días` : base)
      stopPolling()
      // Auto-advance after 5s
      setTimeout(() => {
        setFeedback(null)
        reloadToday()
      }, 5000)
    } else {
      if (riddle?.isBonus) toast.error('Perdiste tu apuesta 😬 — tus puntos volvieron a 0')
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
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <History className="h-5 w-5 text-muted-foreground" />
              Historial
            </CardTitle>
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
              <div key={h.riddleId} className="rounded-[var(--radius)] border border-border p-3 text-sm">
                <p className="font-medium">{h.question}</p>
                <p className="text-muted-foreground mt-1">Respuesta: {h.answer}</p>
                <p className="text-xs text-muted-foreground mt-1">
                  {h.solvedBy} · {POINTS_LABELS[h.pointsEarned] ?? `${h.pointsEarned} pts`}
                </p>
              </div>
            ))
          )}
        </CardContent>
      </CozyCard>
    )
  }

  // No session yet — friendly starting CTA
  if (!session) {
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Heart className="h-5 w-5 text-primary" />
            La Última Pregunta
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-xs font-medium text-muted-foreground">Vidas</span>
              <Hearts lives={3} />
            </div>
            <ScoreBlock label="Pareja 1" score={0} lead={false} />
            <ScoreBlock label="Pareja 2" score={0} lead={false} />
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
      </CozyCard>
    )
  }

  // Lives ran out
  if (session.status === 'gameover') {
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-destructive">
            <AlertCircle className="h-5 w-5" />
            ¡Perdieron!
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <p className="text-center text-muted-foreground">Se acabaron las vidas. 😢</p>
          <div className="flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-xs font-medium text-muted-foreground">Vidas</span>
              <Hearts lives={0} />
            </div>
            <ScoreBlock label="Pareja 1" score={session.p1Score} lead={session.p1Score > session.p2Score} />
            <ScoreBlock label="Pareja 2" score={session.p2Score} lead={session.p2Score > session.p1Score} />
          </div>
          <div className="space-y-2">
            <Button onClick={handleStartGame} disabled={busy} className="w-full">
              Jugar de nuevo
            </Button>
            <Button variant="ghost" size="sm" onClick={handleViewHistory} className="w-full">
              Ver historial
            </Button>
          </div>
        </CardContent>
      </CozyCard>
    )
  }

  // Riddle is solved (by either partner)
  if (session.status === 'solved') {
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-success">
            <Trophy className="h-5 w-5" />
            ¡La resolvieron!
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <div
            className="rounded-[var(--radius)] p-4 text-center"
            style={{ backgroundColor: 'color-mix(in oklch, var(--success) 12%, var(--card))' }}
          >
            <p className="text-sm text-muted-foreground mb-1">La respuesta era:</p>
            <p className="text-xl font-bold text-success">{session.answer ?? riddle?.hint.split('.')[0] ?? ''}</p>
          </div>
          {session.streak > 0 && (
            <div className="flex justify-center">
              <StreakBadge streak={session.streak} />
            </div>
          )}
          <div className="flex items-center justify-between">
            <Hearts lives={session.livesRemaining} />
            <ScoreBlock label="Pareja 1" score={session.p1Score} lead={session.p1Score > session.p2Score} />
            <ScoreBlock label="Pareja 2" score={session.p2Score} lead={session.p2Score > session.p1Score} />
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
      </CozyCard>
    )
  }

  // Expired (24h passed, no one solved — loses a life)
  if (session.status === 'expired') {
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-warning">
            <Clock className="h-5 w-5" />
            Se acabó el tiempo
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <div
            className="rounded-[var(--radius)] p-4 text-center"
            style={{ backgroundColor: 'color-mix(in oklch, var(--warning) 12%, var(--card))' }}
          >
            <p className="text-sm text-muted-foreground mb-1">La respuesta era:</p>
            <p className="text-xl font-bold text-warning">{session.answer ?? riddle?.hint.split('.')[0] ?? ''}</p>
          </div>
          <div className="flex items-center justify-center gap-2 text-warning text-sm font-medium">
            <Heart className="h-4 w-4 fill-warning" />
            <span>-1 vida</span>
          </div>
          <div className="flex justify-center">
            <Hearts lives={session.livesRemaining} />
          </div>
          <Button onClick={() => reloadToday()} className="w-full">
            Siguiente pregunta
          </Button>
        </CardContent>
      </CozyCard>
    )
  }

  // Active daily riddle
  const isP1 = userId === session.teamId
  const isBonus = riddle?.isBonus ?? false
  const myScore = isP1 ? session.p1Score : session.p2Score

  return (
    <CozyCard className={cn('mx-auto max-w-md animate-card-in', isBonus && 'ring-2 ring-primary/40')}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            {isBonus && <Dices className="h-5 w-5 text-primary" />}
            {isBonus ? 'Pregunta Bonus' : 'La Última Pregunta'}
          </CardTitle>
          <div className="flex items-center gap-3">
            <StreakBadge streak={session.streak} />
            <Hearts lives={session.livesRemaining} />
          </div>
        </div>
        <div className="flex items-center justify-between text-xs text-muted-foreground mt-1">
          <span>{isBonus ? 'Doble o nada' : riddle ? DIFFICULTY_LABELS[riddle.difficulty] : ''}</span>
          <span className="flex items-center gap-1">
            <Clock className="h-3 w-3" />
            {countdown}
          </span>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Score bar */}
        <div className="flex items-center justify-center gap-8">
          <ScoreBlock label="Pareja 1" score={session.p1Score} lead={isP1} />
          <div className="text-sm text-muted-foreground">vs</div>
          <ScoreBlock label="Pareja 2" score={session.p2Score} lead={!isP1} />
        </div>

        {isBonus && confirmedBonusRiddleId !== riddle?.id ? (
          // Explicit opt-in gate — a wrong guess wipes myScore, so typing an
          // answer shouldn't be able to trigger that without a deliberate
          // confirm first.
          <div
            className="rounded-[var(--radius)] p-4 text-center space-y-3"
            style={{ backgroundColor: 'color-mix(in oklch, var(--primary) 10%, var(--card))' }}
          >
            <p className="text-sm">
              Es domingo — pregunta bonus. Si acertás, <strong>duplicás</strong> tus {myScore} pts.
              Si fallás, los <strong>perdés todos</strong>.
            </p>
            <Button onClick={() => setConfirmedBonusRiddleId(riddle?.id ?? null)} className="w-full">
              Apostar {myScore} pts
            </Button>
          </div>
        ) : (
          <>
            {/* Question */}
            {riddle && (
              <div
                className="rounded-[var(--radius)] border border-border p-5 text-center"
                style={paperSurfaceStyle}
              >
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
                  <p
                    className="rounded-[var(--radius)] p-3 text-sm text-warning"
                    style={{ backgroundColor: 'color-mix(in oklch, var(--warning) 12%, var(--card))' }}
                  >
                    💡 {riddle.hint}
                  </p>
                )}
              </div>
            )}

            {/* Feedback */}
            {feedback && (
              <p
                className={cn('text-center font-medium', feedback === 'correct' ? 'text-success' : 'text-destructive')}
              >
                {feedback === 'correct'
                  ? isBonus
                    ? '¡Doblaste tus puntos! 🎲'
                    : '¡Correcto! 🎉'
                  : isBonus
                    ? 'Perdiste tu apuesta 😬'
                    : 'Nop, otra vez'}
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
          </>
        )}

        {/* History link */}
        <Button variant="ghost" size="sm" onClick={handleViewHistory} className="w-full">
          Ver historial
        </Button>
      </CardContent>
    </CozyCard>
  )
}
