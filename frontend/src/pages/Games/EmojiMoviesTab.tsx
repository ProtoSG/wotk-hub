import { useState } from 'react'
import { toast } from 'sonner'
import { Trophy, Copy, Users } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard, paperSurfaceStyle } from '@/components/ui/cozy-card'
import { useGamesApi } from '@/hooks/useGamesApi'
import { useGameSocket } from '@/hooks/useGameSocket'
import { useAuthStore } from '@/store/authStore'
import { cn } from '@/lib/utils'
import type { EmojiGameSession, MovieDifficulty } from '@/types/games.types'

const DIFFICULTIES: { value: MovieDifficulty; label: string }[] = [
  { value: 'easy', label: 'Fácil' },
  { value: 'medium', label: 'Media' },
  { value: 'hard', label: 'Difícil' },
]

// Compact score readout shared by the header (in-progress) and the recap
// (finished) — same "small muted label over a bold number" pattern as
// Couple/EstadisticasTab's stat tiles, just inline instead of its own card
// (avoids stacking a card inside this one).
function ScoreBlock({ label, score, lead }: { label: string; score: number; lead: boolean }) {
  return (
    <div className="text-center">
      <div className={cn('text-2xl font-bold tabular-nums', lead ? 'text-primary' : 'text-muted-foreground')}>
        {score}
      </div>
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
    </div>
  )
}

export default function EmojiMoviesTab() {
  const { createSession, joinSession, getSession, guess, reveal } = useGamesApi()
  const userId = useAuthStore((s) => s.user?.id)

  const [session, setSession] = useState<EmojiGameSession | null>(null)
  const [joinId, setJoinId] = useState('')
  const [guessText, setGuessText] = useState('')
  const [feedback, setFeedback] = useState<'correct' | 'wrong' | null>(null)
  const [difficulty, setDifficulty] = useState<MovieDifficulty | ''>('')
  const [busy, setBusy] = useState(false)

  // Live updates over WebSocket — replaces the old 6s poll. The hub
  // broadcasts to the whole team, not scoped to one session id, so only
  // apply an event that's actually about the session on screen; on
  // (re)connect reconcile once via REST in case anything was missed while
  // disconnected.
  useGameSocket({
    onMessage: (event) => {
      if (event.type !== 'emoji_movies.session') return
      if (session && event.session.id !== session.id) return
      setSession(event.session)
    },
    onOpen: () => {
      if (!session) return
      getSession(session.id)
        .then(setSession)
        .catch(() => {
          // transient reconcile failure — the next event/reconnect will catch up
        })
    },
  })

  async function handleCreate() {
    setBusy(true)
    try {
      const s = await createSession(difficulty || undefined)
      setSession(s)
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
    setSession(null)
    setGuessText('')
    setFeedback(null)
  }

  async function handleCopyId() {
    if (!session) return
    try {
      await navigator.clipboard.writeText(String(session.id))
      toast.success('ID copiado')
    } catch {
      toast.error('No se pudo copiar')
    }
  }

  if (!session) {
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle>Emoji Movies</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-3">
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

          <div className="space-y-3 border-t pt-4">
            <Input
              placeholder="ID de la partida"
              value={joinId}
              onChange={(e) => setJoinId(e.target.value)}
              inputMode="numeric"
            />
            <Button onClick={handleJoin} disabled={busy} variant="secondary" className="w-full">
              Unirse
            </Button>
          </div>
        </CardContent>
      </CozyCard>
    )
  }

  if (session.status === 'waiting') {
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Users className="h-5 w-5 animate-pulse text-muted-foreground" />
            Esperando jugador...
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5 text-center">
          <p className="text-sm text-muted-foreground">Compartí este ID con la otra persona para que se una:</p>
          <button
            type="button"
            onClick={handleCopyId}
            className="group mx-auto flex items-center gap-2.5 rounded-[var(--radius)] border border-border bg-secondary/60 px-6 py-3 transition-colors hover:border-primary/40"
          >
            <span className="text-3xl font-bold tabular-nums">{session.id}</span>
            <Copy className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-primary" />
          </button>
          <Button variant="outline" onClick={handleNewGame}>
            Cancelar
          </Button>
        </CardContent>
      </CozyCard>
    )
  }

  if (session.status === 'finished') {
    const winner =
      session.p1Score > session.p2Score ? 'Jugador 1' : session.p2Score > session.p1Score ? 'Jugador 2' : 'Empate'
    return (
      <CozyCard className="mx-auto max-w-md animate-card-in">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Trophy className="h-5 w-5 text-warning" />
            ¡Partida terminada!
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6 text-center">
          <p className="text-xl font-semibold">{winner === 'Empate' ? 'Empate' : `Ganó ${winner}`}</p>
          <div className="flex items-center justify-center gap-8">
            <ScoreBlock label="Jugador 1" score={session.p1Score} lead={session.p1Score > session.p2Score} />
            <div className="text-sm text-muted-foreground">vs</div>
            <ScoreBlock label="Jugador 2" score={session.p2Score} lead={session.p2Score > session.p1Score} />
          </div>
          <Button onClick={handleNewGame} className="w-full">
            Nueva partida
          </Button>
        </CardContent>
      </CozyCard>
    )
  }

  const isPlayer1 = userId === session.player1Id

  return (
    <CozyCard className="mx-auto max-w-md animate-card-in">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Emoji Movies</CardTitle>
          <div className="flex items-center gap-4">
            <ScoreBlock label="Vos" score={isPlayer1 ? session.p1Score : session.p2Score} lead />
            <ScoreBlock
              label="Rival"
              score={isPlayer1 ? session.p2Score : session.p1Score}
              lead={false}
            />
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <div
          className="rounded-[var(--radius)] border border-border py-12 text-center text-7xl"
          style={paperSurfaceStyle}
        >
          {session.currentEmoji}
        </div>

        {feedback && (
          <p className={cn('text-center font-medium', feedback === 'correct' ? 'text-success' : 'text-destructive')}>
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
    </CozyCard>
  )
}
