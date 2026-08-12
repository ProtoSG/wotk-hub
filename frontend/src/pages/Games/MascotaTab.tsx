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
  MessageCircle,
  Camera,
  Images,
  Trash2,
  Loader2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { usePetApi } from '@/hooks/usePetApi'
import type { PetPhoto } from '@/hooks/usePetApi'
import { useAuthStore } from '@/store/authStore'
import type { CareAction, PetActionStatus, PetMood, PetState } from '@/types/pet.types'
import PetChat from './PetChat'
import PetCamera from './PetCamera'
import { MOOD_SPRITE } from './petSprites'

const MOOD_LABEL: Record<PetMood, string> = {
  happy: '¡Está feliz!',
  neutral: 'Está bien',
  sad: 'Te extraña un poco...',
  hungry: 'Tiene hambre de cariño',
}

// Deliberately not exposing careScore as a number anywhere in this UI —
// the brief called for calm/minimalist, not a stat to optimize. Mood is
// the only signal the couple sees.
//
// These are animated GIFs (PixelLab "Breathing Idle" exports), not static
// PNGs — the browser animates them natively, so the CSS pet-float/flicker
// fake-motion classes are no longer applied to this image (see the render
// below). CARE_ACTIONS' reactionSprite entries are animated GIFs too now.
// MOOD_SPRITE now lives in petSprites.ts (imported above) so PetChat.tsx can
// reuse the same mood->sprite mapping for its header avatar without
// duplicating this table or tripping react-refresh's
// only-export-components rule on this component file.

const SLEEP_SPRITE = '/pet/sleep.gif'
const SLEEP_START_HOUR = 22
const SLEEP_END_HOUR = 7

// Scattered star positions behind the sleeping sprite — hand-placed rather
// than random so they don't overlap the sprite's own face, each with its
// own twinkle delay so they don't all pulse in lockstep. Pixel squares
// (no border-radius), matching the rest of this feature's flat/hard-edged
// pixel-art material instead of soft dots.
const SLEEP_STARS = [
  { top: '12%', left: '18%', size: 3, delay: '0s' },
  { top: '20%', left: '78%', size: 2, delay: '0.4s' },
  { top: '55%', left: '10%', size: 2, delay: '0.9s' },
  { top: '15%', left: '48%', size: 2, delay: '1.3s' },
  { top: '70%', left: '85%', size: 3, delay: '0.6s' },
  { top: '75%', left: '30%', size: 2, delay: '1.1s' },
]

// Lima hour via Intl instead of a hardcoded UTC-5 offset — same fixed-offset
// fact the backend relies on (Peru has no DST), but reading it through the
// IANA zone means this doesn't silently drift if that fact ever stopped
// being true, and it's one less duplicated offset constant. Purely
// cosmetic (which sprite to show), so no need to keep it live-updating
// every second — it's re-read on every render, and the component already
// re-renders at least every 8s from polling.
function isSleepingNow(): boolean {
  const hour = Number(new Intl.DateTimeFormat('en-US', { timeZone: 'America/Lima', hour: 'numeric', hour12: false }).format(new Date()))
  return hour >= SLEEP_START_HOUR || hour < SLEEP_END_HOUR
}

// The reaction GIFs (react-play/react-eat/react-clean) are 9 frames at
// 200ms each — 1.8s for a full loop. 4000ms plays that loop through twice
// plus a buffer, instead of cutting off mid-first-pass.
const REACTION_MS = 4000

// fidget.gif is 17 frames at 200ms — its one natural loop, in ms.
const FIDGET_MS = 17 * 200
// Gap between fidgets: random 8-15s. Frequent enough to feel alive on a
// screen someone's actually looking at, rare enough to still read as a
// deliberate flourish instead of a nervous tic.
const FIDGET_MIN_GAP_MS = 8000
const FIDGET_MAX_GAP_MS = 15000

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
  // single tap feel like a real reaction rather than the same face just
  // bouncing. All 5 actions have one now: breakfast/lunch/dinner share one
  // "eating a cookie" GIF (same underlying action, no reason to generate 3
  // near-identical clips), bathe and play each have their own. Still
  // optional in the type — an action without one just shows the current
  // mood GIF with the pop bounce instead, which is what let this roll out
  // sprite-by-sprite instead of needing all 5 before shipping any.
  reactionSprite?: string
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
    listPetPhotos,
    deletePetPhoto,
    clearPetPhotos,
  } = usePetApi()
  const role = useAuthStore((s) => s.user?.role)
  // Named after the reset button originally — reused below to gate the
  // gallery's delete/clear actions too, same admin-only semantics.
  const canReset = role === 'admin'
  const [pet, setPet] = useState<PetState | null>(null)
  const [busy, setBusy] = useState(false)
  const [reactingAction, setReactingAction] = useState<CareAction | null>(null)
  // An occasional flourish (tossing a spark, catching it) shown instead of
  // the plain breathing loop — keeps a long-idle pet from feeling static.
  // Generated against the happy face specifically, so it's only shown for
  // that mood; other moods just keep their own breathing loop.
  const [fidgeting, setFidgeting] = useState(false)
  const [resetDialogOpen, setResetDialogOpen] = useState(false)
  const [shopOpen, setShopOpen] = useState(false)
  const [chatOpen, setChatOpen] = useState(false)
  const [cameraOpen, setCameraOpen] = useState(false)
  const [galleryOpen, setGalleryOpen] = useState(false)
  const [galleryPhotos, setGalleryPhotos] = useState<PetPhoto[] | null>(null)
  const [previewPhoto, setPreviewPhoto] = useState<PetPhoto | null>(null)
  const [deletingPhotoId, setDeletingPhotoId] = useState<number | null>(null)
  const [clearGalleryDialogOpen, setClearGalleryDialogOpen] = useState(false)
  const [clearingGallery, setClearingGallery] = useState(false)
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
      reactionSprite: '/pet/react-clean.gif',
      apiCall: bathePet,
      successMessage: 'Quedó reluciente',
    },
    {
      key: 'breakfast',
      label: 'Desayunar',
      icon: Coffee,
      accent: '--primary',
      reactionSprite: '/pet/react-eat.gif',
      apiCall: breakfastPet,
      successMessage: 'Le diste el desayuno',
    },
    {
      key: 'lunch',
      label: 'Almorzar',
      icon: Utensils,
      accent: '--primary',
      reactionSprite: '/pet/react-eat.gif',
      apiCall: lunchPet,
      successMessage: 'Le diste el almuerzo',
    },
    {
      key: 'play',
      label: 'Jugar',
      icon: Heart,
      accent: '--success',
      reactionSprite: '/pet/react-play.gif',
      apiCall: playWithPet,
      successMessage: '¡Jugaron juntos!',
    },
    {
      key: 'dinner',
      label: 'Cenar',
      icon: Moon,
      accent: '--primary',
      reactionSprite: '/pet/react-eat.gif',
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

  // Poll for the partner's changes — there's no push mechanism (no
  // websocket/SSE in this app yet), so without this, one partner's care
  // actions only show up for the other after a reload. 8s balances "feels
  // live enough" against extra requests; this feature is a handful of taps
  // a day, not a fast game, so a few seconds of staleness is fine. Skipped
  // while `busy` so a poll response can't land between an action's request
  // and its own (fresher) response. Deliberately does NOT touch nameInput
  // — that's local edit-in-progress state; only the initial load and the
  // rename/reset actions above are allowed to overwrite it.
  useEffect(() => {
    const interval = setInterval(async () => {
      if (busy) return
      try {
        const { pet: p } = await getPetState()
        setPet(p)
      } catch {
        // Silent — a failed background poll isn't worth interrupting
        // whoever's looking at the screen with an error toast; the next
        // tick tries again, and any explicit action still surfaces its own
        // errors normally.
      }
    }, 8000)
    return () => clearInterval(interval)
  }, [getPetState, busy])

  // Schedules the occasional idle fidget on a random loop. Re-runs whenever
  // mood changes so it starts scheduling the moment mood becomes 'happy'
  // and stops cleanly (via the cleanup) the moment it stops being happy —
  // the render below double-checks mood too, so a fidget that was already
  // mid-display when mood flips away just finishes invisibly rather than
  // popping a mismatched face.
  useEffect(() => {
    if (!pet || pet.mood !== 'happy') return
    let gapTimer: ReturnType<typeof setTimeout>
    let revertTimer: ReturnType<typeof setTimeout>
    function scheduleNext() {
      const gap = FIDGET_MIN_GAP_MS + Math.random() * (FIDGET_MAX_GAP_MS - FIDGET_MIN_GAP_MS)
      gapTimer = setTimeout(() => {
        setFidgeting(true)
        revertTimer = setTimeout(() => setFidgeting(false), FIDGET_MS)
        scheduleNext()
      }, gap)
    }
    scheduleNext()
    return () => {
      clearTimeout(gapTimer)
      clearTimeout(revertTimer)
    }
    // Deliberately depending on pet?.mood only, not all of `pet` — care_score/
    // streak/sparks changing shouldn't tear down and reschedule this timer
    // chain, only an actual mood transition should. Nothing past the initial
    // guard reads `pet` again, so there's no stale-closure risk either.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pet?.mood])

  // Fetch gallery photos when the gallery dialog opens.
  useEffect(() => {
    if (!galleryOpen) return
    let cancelled = false
    async function load() {
      try {
        const photos = await listPetPhotos()
        if (!cancelled) setGalleryPhotos(photos)
      } catch {
        if (!cancelled) toast.error('No se pudieron cargar las fotos')
      }
    }
    load()
    return () => { cancelled = true }
  }, [galleryOpen, listPetPhotos])

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

  async function handleDeletePhoto(photo: PetPhoto) {
    setDeletingPhotoId(photo.id)
    try {
      await deletePetPhoto(photo.id)
      setGalleryPhotos((prev) => (prev ? prev.filter((p) => p.id !== photo.id) : prev))
      if (previewPhoto?.id === photo.id) setPreviewPhoto(null)
      toast.success('Foto eliminada')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la foto')
    } finally {
      setDeletingPhotoId(null)
    }
  }

  async function handleClearGallery() {
    setClearingGallery(true)
    try {
      await clearPetPhotos()
      setGalleryPhotos([])
      setPreviewPhoto(null)
      setClearGalleryDialogOpen(false)
      toast.success('Galería vaciada')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo vaciar la galería')
    } finally {
      setClearingGallery(false)
    }
  }

  const reactionSprite = reactingAction
    ? CARE_ACTIONS.find((a) => a.key === reactingAction)?.reactionSprite
    : undefined
  // A reaction always wins over the fidget if both happen to be true at
  // once (see the scheduling effect's comment on why that's not worth
  // preventing outright).
  const showFidget = fidgeting && pet?.mood === 'happy' && !reactionSprite
  // Sleep beats the fidget (no tossing sparks around at 3am) but not a
  // reaction — tapping a still-open action late at night should still show
  // its real reaction, not the sleeping sprite.
  const showSleep = !reactionSprite && isSleepingNow()

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
      {/* HUD layout: badges top-left in normal flow, action buttons
          absolutely pinned top-right instead — up to 4 stacked HudIconButton
          (~194px) is taller than the badge row (~30px), so keeping both in
          the same flex row (the previous approach) stretched CardHeader to
          the buttons' height and pushed the pet sprite below down by that
          difference. Absolute takes the button stack out of layout flow
          entirely: CardHeader's height now comes from the badges alone, and
          the buttons float over the top of CardContent instead (no z-index
          needed — positioned descendants paint over in-flow siblings by
          default). They don't visually collide with the sprite since it's
          horizontally centered while the buttons hug the right edge. */}
      <CardHeader className="relative">
        <div className="flex flex-wrap items-center gap-1.5 pr-14">
          {pet && <SparksBadge sparks={pet.sparks} />}
          {!!pet?.streakFreezes && pet.streakFreezes > 0 && <FreezeBadge count={pet.streakFreezes} />}
          {!!pet?.streak && pet.streak > 0 && <StreakBadge streak={pet.streak} />}
        </div>
        <div className="absolute right-6 top-6 flex flex-col items-center gap-1.5">
          <HudIconButton
            icon={MessageCircle}
            label="Chatear con la mascota"
            accent="--chart-2"
            disabled={!pet}
            onClick={() => setChatOpen(true)}
          />
          <HudIconButton
            icon={Camera}
            label="Tomar foto con la mascota"
            accent="--chart-4"
            disabled={!pet}
            onClick={() => setCameraOpen(true)}
          />
          <HudIconButton
            icon={Images}
            label="Ver galería de fotos"
            accent="--chart-7"
            onClick={() => setGalleryOpen(true)}
          />
          {canReset && (
            <HudIconButton
              icon={RotateCcw}
              label="Reiniciar estado"
              accent="--destructive"
              disabled={!pet || busy}
              onClick={() => setResetDialogOpen(true)}
            />
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex flex-col items-center gap-3 py-4">
          {!pet ? (
            <Skeleton className="h-40 w-40 rounded-full" />
          ) : (
            <>
              <div className="relative flex h-40 w-40 items-center justify-center">
                {showSleep && (
                  <div
                    aria-hidden="true"
                    className="absolute inset-0 overflow-hidden rounded-sm"
                    style={{ backgroundColor: 'color-mix(in oklch, var(--chart-2) 55%, black)' }}
                  >
                    {SLEEP_STARS.map((star, i) => (
                      <div
                        key={i}
                        className="animate-pet-star-twinkle absolute rounded-none bg-white"
                        style={{
                          top: star.top,
                          left: star.left,
                          width: star.size,
                          height: star.size,
                          animationDelay: star.delay,
                        }}
                      />
                    ))}
                  </div>
                )}
                {/* Ground shadow — a standalone ambient pulse, not driven by
                    the sprite anymore (the mood GIFs below animate
                    themselves now, no CSS float/flicker needed to fake
                    motion on a static image like before). Left running on
                    its own timing since it still sells a sense of life under
                    the sprite regardless of what that sprite is doing. */}
                <div
                  aria-hidden="true"
                  className="animate-pet-shadow-pulse absolute bottom-1 left-1/2 h-4 w-20 -translate-x-1/2 rounded-full"
                  style={{ backgroundColor: 'color-mix(in oklch, var(--foreground) 30%, transparent)' }}
                />
                <img
                  key={pet.mood}
                  src={reactionSprite ?? (showSleep ? SLEEP_SPRITE : showFidget ? '/pet/fidget.gif' : MOOD_SPRITE[pet.mood])}
                  alt={showSleep ? 'Está durmiendo' : MOOD_LABEL[pet.mood]}
                  // Smaller while sleeping — leaves visible night-sky margin
                  // around it inside the same box instead of enlarging the
                  // box itself (which overlapped the health bar below it).
                  width={showSleep ? 112 : 160}
                  height={showSleep ? 112 : 160}
                  style={{ imageRendering: 'pixelated' }}
                  className={cn(
                    'relative animate-in fade-in-0 duration-500',
                    // Pop plays for every reaction, sprite-swap or not — the
                    // 4 actions without a dedicated reaction GIF still get
                    // the bounce on the mood sprite, just no image change.
                    reactingAction && 'animate-pet-pop'
                  )}
                />
              </div>
              <p className="font-pixel text-lg tracking-wide text-muted-foreground">
                {showSleep ? 'Está durmiendo 💤' : MOOD_LABEL[pet.mood]}
              </p>
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
                  <PriceTag cost={FREEZE_COST_SPARKS} />
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
                        <PriceTag cost={RENAME_COST_SPARKS} />
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
                        <PriceTag cost={RENAME_COST_SPARKS} />
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

      {pet && (
        <PetChat open={chatOpen} onOpenChange={setChatOpen} mood={pet.mood} petName={pet.name} />
      )}
      {pet && (
        <PetCamera
          open={cameraOpen}
          onOpenChange={setCameraOpen}
          mood={pet.mood}
          onPhotoTaken={() => {
            // Future: could show a toast or gallery — for now just close
            toast.success('¡Foto guardada!')
          }}
        />
      )}

      <Dialog open={galleryOpen} onOpenChange={setGalleryOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <div className="flex items-center justify-between gap-2 pr-6">
              <DialogTitle className="font-pixel text-xl tracking-wide">Galería de fotos</DialogTitle>
              {canReset && !!galleryPhotos?.length && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 font-pixel text-xs tracking-wide text-destructive hover:bg-destructive/10 hover:text-destructive"
                  onClick={() => setClearGalleryDialogOpen(true)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  Vaciar
                </Button>
              )}
            </div>
          </DialogHeader>
          {galleryPhotos === null ? (
            <div className="grid grid-cols-3 gap-2">
              {[...Array(6)].map((_, i) => (
                <Skeleton key={i} className="aspect-square rounded-sm" />
              ))}
            </div>
          ) : galleryPhotos.length === 0 ? (
            <p className="py-8 text-center font-pixel text-sm text-muted-foreground">
              Aún no hay fotos. ¡Tómate una con la cámara!
            </p>
          ) : (
            <div className="grid grid-cols-3 gap-2">
              {galleryPhotos.map((photo) => (
                <div key={photo.id} className="relative">
                  <button
                    type="button"
                    onClick={() => setPreviewPhoto(photo)}
                    className="block w-full overflow-hidden rounded-sm border-2 transition-transform active:scale-[0.97]"
                    style={{
                      borderColor: 'color-mix(in oklch, var(--border) 70%, transparent)',
                      boxShadow: '2px 2px 0 0 color-mix(in oklch, var(--foreground) 15%, transparent)',
                    }}
                  >
                    <img
                      src={photo.url}
                      alt={`Foto ${photo.id}`}
                      className="aspect-square h-full w-full object-cover"
                      style={{ imageRendering: 'pixelated' }}
                    />
                  </button>
                  {canReset && (
                    <button
                      type="button"
                      aria-label="Eliminar foto"
                      title="Eliminar foto"
                      disabled={deletingPhotoId === photo.id}
                      onClick={(e) => {
                        e.stopPropagation()
                        handleDeletePhoto(photo)
                      }}
                      className="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-sm bg-black/60 text-white backdrop-blur-sm transition-colors hover:bg-destructive disabled:opacity-50"
                    >
                      {deletingPhotoId === photo.id ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Trash2 className="h-3.5 w-3.5" />
                      )}
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={clearGalleryDialogOpen} onOpenChange={(open) => !clearingGallery && setClearGalleryDialogOpen(open)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="font-pixel text-xl tracking-wide">Vaciar galería</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Elimina todas las fotos de la galería para siempre. No se puede deshacer.
          </p>
          <DialogFooter>
            <Button
              variant="ghost"
              className="font-pixel tracking-wide"
              onClick={() => setClearGalleryDialogOpen(false)}
              disabled={clearingGallery}
            >
              Cancelar
            </Button>
            <Button
              className="font-pixel tracking-wide"
              variant="destructive"
              onClick={handleClearGallery}
              disabled={clearingGallery}
            >
              {clearingGallery ? 'Vaciando…' : 'Vaciar galería'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!previewPhoto} onOpenChange={(open) => !open && setPreviewPhoto(null)}>
        <DialogContent className="flex max-w-fit flex-col items-center gap-3 p-4">
          {previewPhoto && (
            <>
              <img
                src={previewPhoto.url}
                alt={`Foto ${previewPhoto.id}`}
                className="max-h-[80vh] max-w-full object-contain"
                style={{ imageRendering: 'pixelated' }}
              />
              {canReset && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1.5 self-end font-pixel text-xs tracking-wide text-destructive hover:bg-destructive/10 hover:text-destructive"
                  disabled={deletingPhotoId === previewPhoto.id}
                  onClick={() => handleDeletePhoto(previewPhoto)}
                >
                  {deletingPhotoId === previewPhoto.id ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Trash2 className="h-3.5 w-3.5" />
                  )}
                  Eliminar
                </Button>
              )}
            </>
          )}
        </DialogContent>
      </Dialog>
    </CozyCard>
  )
}

// HUD action button — chat/camera/gallery/reset in the header's top-right
// stack. Same pixel-game material as TurnCard/the Tienda button (color-mix
// wash, hard-edged pixelShadow, border-2) instead of the plain ghost Button
// these used before, so the whole header reads as one consistent HUD
// instead of mixing a shadcn-default icon button in with the rest.
function HudIconButton({
  icon: Icon,
  label,
  accent,
  onClick,
  disabled,
}: {
  icon: typeof MessageCircle
  label: string
  accent: string
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className="flex h-11 w-11 shrink-0 items-center justify-center rounded-sm border-2 transition-transform active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50"
      style={{
        backgroundColor: `color-mix(in oklch, var(${accent}) 22%, var(--card))`,
        borderColor: `color-mix(in oklch, var(${accent}) 70%, var(--border))`,
        color: `var(${accent})`,
        boxShadow: pixelShadow(accent, 2),
      }}
    >
      <Icon className="h-5 w-5" />
    </button>
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

// Same Coins icon used everywhere else money shows up (SparksBadge, the
// shop balance line) — a price tag on a buy button is the same currency,
// it should never look like a different coin. Lucide icon rather than the
// 🪙 emoji this used to be: emoji glyphs render differently (sometimes
// wildly so) across browsers/OS/fonts, an SVG icon doesn't.
function PriceTag({ cost }: { cost: number }) {
  return (
    <span className="inline-flex items-center gap-1">
      {cost}
      <Coins className="h-3.5 w-3.5" />
    </span>
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
