import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import {
  ShowerHead,
  Coffee,
  Utensils,
  Heart,
  Moon,
  Lock,
  Check,
  RotateCcw,
  Flame,
  PartyPopper,
  Store,
  Coins,
  Snowflake,
  Pencil,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { usePetApi } from '@/hooks/usePetApi'
import { useAuthStore } from '@/store/authStore'
import type { CareAction, PetActionStatus, PetMood, PetState } from '@/types/pet.types'

const MOOD_LABEL: Record<PetMood, string> = {
  happy: '¡Está feliz!',
  neutral: 'Está bien',
  sad: 'Te extraña un poco...',
  hungry: 'Tiene hambre de cariño',
}

// Deliberately not exposing careScore as a number anywhere in this UI —
// the brief called for calm/minimalist, not a stat to optimize. Mood is
// the only signal the couple sees.
const MOOD_SPRITE: Record<PetMood, string> = {
  happy: '/pet/happy.png',
  neutral: '/pet/neutral.png',
  sad: '/pet/sad.png',
  hungry: '/pet/hungry.png',
}

const REACTION_MS = 400

// Mirrors backend/modules/pet/handlers.go's freezeCostSparks — kept in sync
// by hand. The backend is still the enforcement point (BuyFreeze re-checks
// the balance server-side), this is only used to disable the buy button
// early and show the price without a round trip.
const FREEZE_COST_SPARKS = 15

// Mirrors backend/modules/pet/handlers.go's renameCostSparks/maxNameLen.
const RENAME_COST_SPARKS = 10
const MAX_NAME_LEN = 20

// Hard-edged offset shadow, zero blur — the classic 8-bit/pixel-art "drop
// shadow block" (Stardew Valley/Earthbound-style panels), not a soft modern
// glow. A blurred shadow reads as smooth/modern and fights the pixel font
// and pixelated sprites; a flat offset block reads as the same material.
// Paired with a real border (crisp outline, not shadow-simulated) since
// retro game UI panels always draw both. `size` in px controls the offset.
function pixelShadow(accent: string, size = 3) {
  return `${size}px ${size}px 0 0 color-mix(in oklch, var(${accent}) 70%, transparent)`
}

// Neutral resting shadow for inactive/locked surfaces — no accent color to
// draw from, just enough of the same flat-block treatment to read as "off."
const NEUTRAL_PIXEL_SHADOW = '2px 2px 0 0 color-mix(in oklch, var(--foreground) 18%, transparent)'

interface CareActionConfig {
  key: CareAction
  label: string
  icon: typeof ShowerHead
  // A --token name from the app's existing palette (index.css), not a new
  // hue — same "wash" tint pattern as Couple/CategoryChip: color-mix over
  // --card for the surface, full-strength for the icon/label.
  accent: string
  // Shown for REACTION_MS instead of the persistent mood sprite — lets a
  // single tap feel like a real reaction (excited for food, wink for play,
  // startled for the shower) rather than the same face just bouncing.
  reactionSprite: string
  apiCall: () => Promise<{ pet: PetState }>
  successMessage: string
}

// A game-HUD-style reward popup for this feature only — bouncier and more
// colorful than the app's flat default toasts (sonner's <Toaster> elsewhere
// stays untouched; toast.custom() renders arbitrary JSX per-call instead of
// changing shared toast styling). Takes icon/accent/message directly rather
// than looking them up in a separate table — CARE_ACTIONS below already
// carries that metadata per action, no need for a second copy of it.
function petToast(Icon: typeof ShowerHead, accent: string, message: string) {
  toast.custom(() => (
    <div
      className="animate-pet-toast-in flex items-center gap-2 rounded-sm border-2 px-4 py-2.5"
      style={{
        backgroundColor: `color-mix(in oklch, var(${accent}) 24%, var(--card))`,
        borderColor: `color-mix(in oklch, var(${accent}) 70%, var(--border))`,
        color: `var(${accent})`,
        boxShadow: pixelShadow(accent),
      }}
    >
      <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
      <span className="font-pixel text-lg leading-none tracking-wide">{message}</span>
    </div>
  ))
}

export default function MascotaTab() {
  const {
    getPetState,
    bathePet,
    breakfastPet,
    lunchPet,
    playWithPet,
    dinnerPet,
    buyStreakFreeze,
    renamePet,
    resetPet,
  } = usePetApi()
  const role = useAuthStore((s) => s.user?.role)
  const canReset = role === 'admin'
  const [pet, setPet] = useState<PetState | null>(null)
  const [busy, setBusy] = useState(false)
  const [reactingAction, setReactingAction] = useState<CareAction | null>(null)
  const [resetDialogOpen, setResetDialogOpen] = useState(false)
  const [shopOpen, setShopOpen] = useState(false)
  const [nameInput, setNameInput] = useState('')
  // Whether the shop's "Cambiar nombre" item has been tapped open to reveal
  // its input — starts collapsed so the shop's item list stays scannable
  // (icon + label + price), same shape as every other item, instead of an
  // input field sitting there all the time.
  const [showRenameField, setShowRenameField] = useState(false)

  // Order matters here — it's the render order of the button grid below.
  // Rebuilt each render since it closes over the hook's care functions, but
  // those are useCallback-stable and this array is cheap, so that's fine.
  const CARE_ACTIONS: CareActionConfig[] = [
    {
      key: 'bathe',
      label: 'Bañar',
      icon: ShowerHead,
      accent: '--warning',
      reactionSprite: '/pet/react-clean.png',
      apiCall: bathePet,
      successMessage: 'Quedó reluciente',
    },
    {
      key: 'breakfast',
      label: 'Desayunar',
      icon: Coffee,
      accent: '--primary',
      reactionSprite: '/pet/react-feed.png',
      apiCall: breakfastPet,
      successMessage: 'Le diste el desayuno',
    },
    {
      key: 'lunch',
      label: 'Almorzar',
      icon: Utensils,
      accent: '--primary',
      reactionSprite: '/pet/react-lunch.png',
      apiCall: lunchPet,
      successMessage: 'Le diste el almuerzo',
    },
    {
      key: 'play',
      label: 'Jugar',
      icon: Heart,
      accent: '--success',
      reactionSprite: '/pet/react-play.png',
      apiCall: playWithPet,
      successMessage: '¡Jugaron juntos!',
    },
    {
      key: 'dinner',
      label: 'Cenar',
      icon: Moon,
      accent: '--primary',
      reactionSprite: '/pet/react-dinner.png',
      apiCall: dinnerPet,
      successMessage: 'Le diste la cena',
    },
  ]

  useEffect(() => {
    ;(async () => {
      try {
        const { pet: p } = await getPetState()
        setPet(p)
        setNameInput(p.name)
      } catch {
        toast.error('No se pudo cargar la mascota')
      }
    })()
  }, [getPetState])

  function playReaction(kind: CareAction) {
    setReactingAction(kind)
    setTimeout(() => setReactingAction(null), REACTION_MS)
  }

  async function handleCare(config: CareActionConfig, status: PetActionStatus) {
    if (status.done || status.locked || busy) return
    setBusy(true)
    const wasPerfectDay = pet?.perfectDay ?? false
    try {
      const { pet: p } = await config.apiCall()
      setPet(p)
      playReaction(config.key)
      petToast(config.icon, config.accent, config.successMessage)
      // Only fire on the specific false -> true transition, and only from
      // an action response, never from GetState on mount.
      if (p.perfectDay && !wasPerfectDay) {
        petToast(PartyPopper, '--success', '¡Día perfecto!')
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo completar la acción')
    } finally {
      setBusy(false)
    }
  }

  async function handleBuyFreeze() {
    if (busy || !pet || pet.sparks < FREEZE_COST_SPARKS) return
    setBusy(true)
    try {
      const { pet: p } = await buyStreakFreeze()
      setPet(p)
      toast.success('Congelador de racha comprado')
    } catch {
      // The backend's "not enough sparks" is an internal message, not meant
      // for display — the buy button is already disabled below that
      // balance, so reaching this catch means a stale/raced state, not
      // something the user needs the raw reason for.
      toast.error('No se pudo comprar')
    } finally {
      setBusy(false)
    }
  }

  async function handleRename() {
    const trimmed = nameInput.trim()
    // Mirrors the backend's Rename: the very first name is free, every
    // rename after that costs RENAME_COST_SPARKS.
    const cost = pet?.name ? RENAME_COST_SPARKS : 0
    if (busy || !pet || !trimmed || pet.sparks < cost) return
    setBusy(true)
    try {
      const { pet: p } = await renamePet(trimmed)
      setPet(p)
      setNameInput(p.name)
      setShowRenameField(false)
      toast.success(`Ahora se llama ${p.name}`)
    } catch {
      // Same reasoning as handleBuyFreeze's catch — the button is already
      // disabled below the price/empty-name cases the backend would reject.
      toast.error('No se pudo cambiar el nombre')
    } finally {
      setBusy(false)
    }
  }

  async function handleReset() {
    setBusy(true)
    try {
      const { pet: p } = await resetPet()
      setPet(p)
      setNameInput(p.name)
      setShowRenameField(false)
      setResetDialogOpen(false)
      toast.success('Estado reiniciado')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo reiniciar')
    } finally {
      setBusy(false)
    }
  }

  const reactionSprite = reactingAction
    ? CARE_ACTIONS.find((a) => a.key === reactingAction)?.reactionSprite
    : undefined

  // One action on stage at a time, in CARE_ACTIONS order — the first one
  // not yet done today, whether or not its window is open yet. If it's
  // open, it's the tappable turn; if not, it renders as "next up" so the
  // couple can see what's coming without a whole grid of buttons. Multiple
  // actions can be genuinely open at once (windows never re-lock — see
  // CareActionConfig), so finishing the current turn just reveals whichever
  // one is next in line, open or not.
  const currentTurn = pet ? CARE_ACTIONS.find((a) => !pet[a.key].done) : undefined

  return (
    <CozyCard className="mx-auto max-w-md animate-card-in">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="font-pixel text-2xl tracking-wide">{pet?.name || 'Nuestra mascota'}</CardTitle>
          <div className="flex items-center gap-2">
            {pet && <SparksBadge sparks={pet.sparks} />}
            {!!pet?.streakFreezes && pet.streakFreezes > 0 && <FreezeBadge count={pet.streakFreezes} />}
            {!!pet?.streak && pet.streak > 0 && <StreakBadge streak={pet.streak} />}
            {canReset && (
              <Button
                variant="ghost"
                size="icon"
                aria-label="Reiniciar estado"
                disabled={!pet || busy}
                onClick={() => setResetDialogOpen(true)}
              >
                <RotateCcw className="h-4 w-4 text-muted-foreground" />
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex flex-col items-center gap-3 py-4">
          {!pet ? (
            <Skeleton className="h-40 w-40 rounded-full" />
          ) : (
            <>
              <div className="relative h-40 w-40">
                {/* Ground shadow — pulses opposite pet-float's timing (same
                    3s duration, no JS sync needed) so it shrinks/fades as
                    the sprite rises and grows/darkens as it settles back
                    down, selling the float as an actual hover over ground. */}
                <div
                  aria-hidden="true"
                  className="animate-pet-shadow-pulse absolute bottom-1 left-1/2 h-4 w-20 -translate-x-1/2 rounded-full"
                  style={{ backgroundColor: 'color-mix(in oklch, var(--foreground) 30%, transparent)' }}
                />
                <img
                  key={pet.mood}
                  src={reactionSprite ?? MOOD_SPRITE[pet.mood]}
                  alt={MOOD_LABEL[pet.mood]}
                  width={160}
                  height={160}
                  style={{ imageRendering: 'pixelated' }}
                  className={cn(
                    'relative animate-in fade-in-0 duration-500',
                    reactionSprite ? 'animate-pet-pop' : 'animate-pet-float animate-pet-flicker'
                  )}
                />
              </div>
              <p className="font-pixel text-lg tracking-wide text-muted-foreground">{MOOD_LABEL[pet.mood]}</p>
              <div className="w-full space-y-1.5">
                <p className="font-pixel text-xs tracking-widest text-muted-foreground">ESTADO</p>
                <HealthBar careScore={pet.careScore} />
              </div>
            </>
          )}
        </div>

        {/* First naming is free and lives here, not in the shop — the shop's
            "Cambiar nombre" item only appears once there's already a name to
            change (see below), same distinction the backend's Rename makes. */}
        {pet && pet.name === '' && (
          <div
            className="flex items-center gap-2 rounded-sm border-2 p-3"
            style={{
              backgroundColor: `color-mix(in oklch, var(--chart-3) 12%, var(--card))`,
              borderColor: `color-mix(in oklch, var(--chart-3) 40%, var(--border))`,
              boxShadow: NEUTRAL_PIXEL_SHADOW,
            }}
          >
            <Pencil className="h-5 w-5 shrink-0" style={{ color: 'var(--chart-3)' }} />
            <Input
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value.slice(0, MAX_NAME_LEN))}
              placeholder="Ponle un nombre"
              maxLength={MAX_NAME_LEN}
              className="font-pixel"
            />
            <Button
              size="sm"
              className="font-pixel tracking-wide shrink-0"
              disabled={busy || !nameInput.trim()}
              onClick={handleRename}
            >
              Listo
            </Button>
          </div>
        )}

        {pet && (
          <div className="space-y-3">
            <TurnTracker actions={CARE_ACTIONS} pet={pet} />
            <div className="flex items-stretch gap-2">
              {currentTurn ? (
                <TurnCard
                  key={currentTurn.key}
                  config={currentTurn}
                  status={pet[currentTurn.key]}
                  busy={busy}
                  onAct={() => handleCare(currentTurn, pet[currentTurn.key])}
                />
              ) : (
                <div
                  className="animate-pet-toast-in flex flex-1 items-center gap-3 rounded-sm border-2 px-4 py-3"
                  style={{
                    backgroundColor: `color-mix(in oklch, var(--success) 24%, var(--card))`,
                    borderColor: `color-mix(in oklch, var(--success) 70%, var(--border))`,
                    color: `var(--success)`,
                    boxShadow: pixelShadow('--success'),
                  }}
                >
                  <PartyPopper className="h-6 w-6 shrink-0" />
                  <p className="font-pixel text-xl leading-none tracking-wide">Todo listo por hoy</p>
                </div>
              )}
              <button
                type="button"
                aria-label="Tienda"
                onClick={() => setShopOpen(true)}
                className="flex w-16 shrink-0 flex-col items-center justify-center gap-1 rounded-sm border-2 transition-transform active:scale-[0.98]"
                style={{
                  // --chart-3 (gold/mustard) rather than --warning — Bañar
                  // already owns --warning, the shop needs its own identity
                  // from the app's existing categorical palette, not a new hue.
                  backgroundColor: `color-mix(in oklch, var(--chart-3) 22%, var(--card))`,
                  borderColor: `color-mix(in oklch, var(--chart-3) 70%, var(--border))`,
                  color: `var(--chart-3)`,
                  boxShadow: pixelShadow('--chart-3'),
                }}
              >
                <Store className="h-5 w-5" />
                <span className="font-pixel text-xs tracking-wide">Tienda</span>
              </button>
            </div>
          </div>
        )}
      </CardContent>

      <Dialog open={resetDialogOpen} onOpenChange={setResetDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-pixel text-xl tracking-wide">Reiniciar mascota</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Vuelve al estado inicial — se pierde el progreso del cuidado de hoy. No se puede deshacer.
          </p>
          <DialogFooter>
            <Button variant="ghost" className="font-pixel tracking-wide" onClick={() => setResetDialogOpen(false)}>
              Cancelar
            </Button>
            <Button className="font-pixel tracking-wide" onClick={handleReset} disabled={busy}>
              {busy ? 'Reiniciando…' : 'Reiniciar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={shopOpen}
        onOpenChange={(open) => {
          setShopOpen(open)
          if (!open) {
            setShowRenameField(false)
            setNameInput(pet?.name ?? '')
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-pixel text-xl tracking-wide">Tienda</DialogTitle>
          </DialogHeader>
          {pet && (
            <>
              <div className="flex items-center gap-1.5 font-pixel text-sm text-muted-foreground">
                <Coins className="h-4 w-4" style={{ color: 'var(--chart-3)' }} />
                <span>Tienes {pet.sparks} chispas</span>
              </div>
              <div
                className="flex items-center gap-3 rounded-sm border-2 p-3"
                style={{
                  backgroundColor: `color-mix(in oklch, var(--chart-3) 12%, var(--card))`,
                  borderColor: `color-mix(in oklch, var(--chart-3) 40%, var(--border))`,
                  boxShadow: NEUTRAL_PIXEL_SHADOW,
                }}
              >
                <Snowflake className="h-8 w-8 shrink-0" style={{ color: 'var(--chart-3)' }} />
                <div className="flex-1">
                  <p className="font-pixel text-base leading-none tracking-wide">Congelador de racha</p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Si un día se pasa sin ninguna acción, protege la racha una vez en vez de reiniciarla.
                  </p>
                </div>
                <Button
                  size="sm"
                  className="font-pixel tracking-wide"
                  disabled={busy || pet.sparks < FREEZE_COST_SPARKS}
                  onClick={handleBuyFreeze}
                >
                  {FREEZE_COST_SPARKS} 🪙
                </Button>
              </div>
              {pet.name && (
                <div
                  className="flex items-center gap-3 rounded-sm border-2 p-3"
                  style={{
                    backgroundColor: `color-mix(in oklch, var(--chart-3) 12%, var(--card))`,
                    borderColor: `color-mix(in oklch, var(--chart-3) 40%, var(--border))`,
                    boxShadow: NEUTRAL_PIXEL_SHADOW,
                  }}
                >
                  <Pencil className="h-8 w-8 shrink-0" style={{ color: 'var(--chart-3)' }} />
                  {showRenameField ? (
                    <>
                      <Input
                        autoFocus
                        value={nameInput}
                        onChange={(e) => setNameInput(e.target.value.slice(0, MAX_NAME_LEN))}
                        placeholder="Nuevo nombre"
                        maxLength={MAX_NAME_LEN}
                        className="font-pixel flex-1"
                      />
                      <Button
                        size="sm"
                        className="font-pixel tracking-wide shrink-0"
                        disabled={busy || !nameInput.trim() || pet.sparks < RENAME_COST_SPARKS}
                        onClick={handleRename}
                      >
                        {RENAME_COST_SPARKS} 🪙
                      </Button>
                    </>
                  ) : (
                    <>
                      <div className="flex-1">
                        <p className="font-pixel text-base leading-none tracking-wide">Cambiar nombre</p>
                        <p className="mt-1 text-xs text-muted-foreground">Ponle otro nombre a la mascota.</p>
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        className="font-pixel tracking-wide shrink-0"
                        onClick={() => {
                          setNameInput(pet.name)
                          setShowRenameField(true)
                        }}
                      >
                        {RENAME_COST_SPARKS} 🪙
                      </Button>
                    </>
                  )}
                </div>
              )}
              <p className="text-xs text-muted-foreground">Más artículos, próximamente.</p>
            </>
          )}
        </DialogContent>
      </Dialog>
    </CozyCard>
  )
}

// Same shape as UltimaPreguntaTab's StreakBadge — consecutive days with at
// least one care action, only shown once there's something to show.
function StreakBadge({ streak }: { streak: number }) {
  return (
    <div
      className="flex items-center gap-1 rounded-sm border px-2 py-0.5"
      title={`Racha de ${streak} día${streak === 1 ? '' : 's'}`}
      style={{
        backgroundColor: `color-mix(in oklch, var(--primary) 18%, var(--card))`,
        borderColor: `color-mix(in oklch, var(--primary) 60%, var(--border))`,
        color: `var(--primary)`,
        boxShadow: pixelShadow('--primary', 2),
      }}
    >
      <Flame className="h-3.5 w-3.5 fill-primary" />
      <span className="font-pixel text-sm leading-none tabular-nums">{streak}</span>
    </div>
  )
}

// Shop currency balance — always shown (even at 0), unlike StreakBadge
// which only shows once there's something to show. A shop needs a visible
// balance to be legible at all; see the sparks comment in types.go for why
// this is a deliberate exception to the "no raw numbers" rule elsewhere.
function SparksBadge({ sparks }: { sparks: number }) {
  return (
    <div
      className="flex items-center gap-1 rounded-sm border px-2 py-0.5"
      title={`${sparks} chispas`}
      style={{
        backgroundColor: `color-mix(in oklch, var(--chart-3) 18%, var(--card))`,
        borderColor: `color-mix(in oklch, var(--chart-3) 60%, var(--border))`,
        color: `var(--chart-3)`,
        boxShadow: pixelShadow('--chart-3', 2),
      }}
    >
      <Coins className="h-3.5 w-3.5" />
      <span className="font-pixel text-sm leading-none tabular-nums">{sparks}</span>
    </div>
  )
}

// Owned streak-freeze count, same shape as StreakBadge — only shown once
// the team owns at least one, so an empty inventory doesn't clutter the
// header.
function FreezeBadge({ count }: { count: number }) {
  return (
    <div
      className="flex items-center gap-1 rounded-sm border px-2 py-0.5"
      title={`${count} congelador${count === 1 ? '' : 'es'} de racha`}
      style={{
        backgroundColor: `color-mix(in oklch, var(--chart-3) 18%, var(--card))`,
        borderColor: `color-mix(in oklch, var(--chart-3) 60%, var(--border))`,
        color: `var(--chart-3)`,
        boxShadow: pixelShadow('--chart-3', 2),
      }}
    >
      <Snowflake className="h-3.5 w-3.5" />
      <span className="font-pixel text-sm leading-none tabular-nums">{count}</span>
    </div>
  )
}

// Ambient status readout, not a stat to chase — same reasoning as
// MOOD_SPRITE above (careScore itself is never printed as a number here).
// Color bands mirror moodFor()'s thresholds in the backend exactly, so the
// bar and the mood text/sprite never disagree about how the pet is doing.
function healthTone(careScore: number): string {
  if (careScore >= 75) return '--success'
  if (careScore >= 50) return '--primary'
  if (careScore >= 25) return '--warning'
  return '--destructive'
}

function HealthBar({ careScore }: { careScore: number }) {
  const tone = healthTone(careScore)
  return (
    <div className="h-3 w-full overflow-hidden rounded-sm border-2 border-border bg-muted">
      <div
        className="h-full transition-all duration-500"
        style={{ width: `${careScore}%`, backgroundColor: `var(${tone})` }}
      />
    </div>
  )
}

// Compact "today so far" readout — the couple can still see every action's
// state and who did it (hover/tap for the name via the native title
// tooltip) even though only one action is ever the tappable turn below.
function TurnTracker({ actions, pet }: { actions: CareActionConfig[]; pet: PetState }) {
  return (
    <div className="flex items-center justify-center gap-1.5">
      {actions.map((a) => {
        const status = pet[a.key]
        const Icon = status.done ? Check : status.locked ? Lock : a.icon
        const title = status.done
          ? `${a.label} — ${status.by}`
          : status.locked
            ? `${a.label}: desde las ${status.unlocksAtHour}:00`
            : a.label
        return (
          <div
            key={a.key}
            title={title}
            className="flex h-7 w-7 items-center justify-center rounded-sm border"
            style={{
              backgroundColor: status.done ? `color-mix(in oklch, var(${a.accent}) 20%, var(--card))` : 'var(--muted)',
              borderColor: status.done ? `color-mix(in oklch, var(${a.accent}) 60%, var(--border))` : 'var(--border)',
              color: status.done ? `var(${a.accent})` : 'var(--muted-foreground)',
              opacity: status.locked ? 0.5 : 1,
              boxShadow: status.done ? pixelShadow(a.accent, 2) : NEUTRAL_PIXEL_SHADOW,
            }}
          >
            <Icon className="h-3.5 w-3.5" />
          </div>
        )
      })}
    </div>
  )
}

// The single on-stage action — same visual language as petToast (bouncy
// pet-toast-in entrance, accent-tinted card, pixel font) since this card
// IS effectively the reward prompt for whichever action is next. Muted and
// non-interactive while locked (`disabled`), full color and tappable once
// its window opens; the whole card is the tap target, not a small button
// inside it, to keep the touch target generous like the old CareButton.
function TurnCard({
  config,
  status,
  busy,
  onAct,
}: {
  config: CareActionConfig
  status: PetActionStatus
  busy: boolean
  onAct: () => void
}) {
  const Icon = status.locked ? Lock : config.icon
  return (
    <button
      type="button"
      disabled={busy || status.locked}
      onClick={onAct}
      className={cn(
        'animate-pet-toast-in flex flex-1 items-center gap-3 rounded-sm border-2 px-4 py-3 text-left transition-transform',
        !status.locked && 'active:scale-[0.98]',
        status.locked && 'cursor-not-allowed'
      )}
      style={{
        backgroundColor: `color-mix(in oklch, var(${config.accent}) ${status.locked ? 12 : 24}%, var(--card))`,
        borderColor: `color-mix(in oklch, var(${config.accent}) ${status.locked ? 35 : 70}%, var(--border))`,
        color: `var(${config.accent})`,
        boxShadow: status.locked ? NEUTRAL_PIXEL_SHADOW : pixelShadow(config.accent),
      }}
    >
      <Icon className="h-6 w-6 shrink-0" />
      <div className="flex-1">
        <p className="font-pixel text-xl leading-none tracking-wide">{config.label}</p>
        {status.locked && (
          <p className="mt-1 font-pixel text-sm tracking-wide text-muted-foreground">
            Disponible desde las {status.unlocksAtHour}:00
          </p>
        )}
      </div>
      {!status.locked && <span className="font-pixel text-sm opacity-70">Tocar</span>}
    </button>
  )
}
