import { useEffect, useRef, useState, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { toast } from 'sonner'
import { Camera, X, Move, Image, RotateCcw, SwitchCamera } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import api from '@/lib/axios'
import { MOOD_SPRITE } from './petSprites'
import type { PetMood } from '@/types/pet.types'

// GIF frames to pick from — the mood sprites plus a few reaction GIFs
// so the overlay feels expressive rather than just the idle loop.
const OVERLAY_OPTIONS: { label: string; src: string }[] = [
  { label: 'Feliz', src: MOOD_SPRITE.happy },
  { label: 'Neutral', src: MOOD_SPRITE.neutral },
  { label: 'Triste', src: MOOD_SPRITE.sad },
  { label: 'Con hambre', src: MOOD_SPRITE.hungry },
  { label: 'Jugando', src: '/pet/react-play.gif' },
  { label: 'Comiendo', src: '/pet/react-eat.gif' },
  { label: 'Bañando', src: '/pet/react-clean.gif' },
]

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

interface PetCameraProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  mood: PetMood
  onPhotoTaken: (photoUrl: string) => void
}

export default function PetCamera({ open, onOpenChange, mood, onPhotoTaken }: PetCameraProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  // The fullscreen preview container — single source of truth for the
  // on-screen viewport rect, used both for drag math and (at capture time)
  // to replicate exactly what object-cover cropped out of the native frame.
  const wrapperRef = useRef<HTMLDivElement>(null)
  const overlayImgRef = useRef<HTMLImageElement>(null)

  const [activeOverlay, setActiveOverlay] = useState(OVERLAY_OPTIONS[0])
  const [overlayPos, setOverlayPos] = useState({ x: 50, y: 50 }) // percentage
  const [isDragging, setIsDragging] = useState(false)
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 })
  const [cameraReady, setCameraReady] = useState(false)
  const [cameraError, setCameraError] = useState<string | null>(null)
  const [capturing, setCapturing] = useState(false)
  const [uploading, setUploading] = useState(false)
  // Back camera by default (photographing what's in front of you, mascot
  // composited on top) — user-toggleable via the flip button instead of
  // hardcoded, same idea as any native camera app.
  const [facingMode, setFacingMode] = useState<'environment' | 'user'>('environment')

  // Read via ref, not a dep, in the open-only effect below — see its
  // comment. Kept current via its own effect rather than a direct render-
  // body assignment (refs shouldn't be written during render).
  const moodRef = useRef(mood)
  useEffect(() => {
    moodRef.current = mood
  })

  // Acquire/release the camera stream. Deps are [open, facingMode] only —
  // NOT mood. mood used to be a dep here, which meant the pet's background
  // 8s status poll (MascotaTab) could flip mood while this dialog was open
  // and silently restart the whole camera + wipe out whatever drag position
  // the user had just set. facingMode is a deliberate user action (the flip
  // button), so restarting the stream for that is correct.
  useEffect(() => {
    if (!open) return
    let cancelled = false
    // Deferred to a microtask — setting state synchronously in an effect
    // body triggers a same-pass cascading re-render; queueMicrotask pushes
    // it just past that, same pattern the original camera-init effect used.
    queueMicrotask(() => {
      if (cancelled) return
      setCameraError(null)
      setCameraReady(false)
    })

    navigator.mediaDevices
      .getUserMedia({ video: { facingMode } })
      .then((stream) => {
        // The dialog may have already closed (or facingMode flipped again)
        // by the time this resolves — without this guard the stream gets
        // assigned and kept alive with nothing left to ever stop it, so the
        // camera indicator light stays on indefinitely.
        if (cancelled) {
          stream.getTracks().forEach((t) => t.stop())
          return
        }
        streamRef.current = stream
        if (videoRef.current) {
          videoRef.current.srcObject = stream
          videoRef.current.onloadedmetadata = () => {
            videoRef.current?.play()
            setCameraReady(true)
          }
        }
      })
      .catch(() => {
        if (!cancelled) setCameraError('No se pudo acceder a la cámara')
      })

    return () => {
      cancelled = true
      streamRef.current?.getTracks().forEach((t) => t.stop())
      streamRef.current = null
    }
  }, [open, facingMode])

  // Reset the overlay pick + position once per open, not on every mood
  // change (see the camera effect's comment above for why mood isn't a dep).
  // moodRef, not mood, so this genuinely only depends on `open`.
  useEffect(() => {
    if (!open) return
    queueMicrotask(() => {
      setActiveOverlay(OVERLAY_OPTIONS.find((o) => o.src === MOOD_SPRITE[moodRef.current]) ?? OVERLAY_OPTIONS[0])
      setOverlayPos({ x: 50, y: 50 })
    })
  }, [open])

  // Half-width/half-height of the overlay, in percent of the wrapper —
  // clamping the drag range to this instead of raw [0,100] keeps the whole
  // sprite inside the preview. Without it, dragging near an edge pushes the
  // pivot (which is CSS-centered via translate(-50%,-50%)) far enough that
  // half the sprite lands outside the wrapper and gets clipped — reads as
  // the mascot "shrinking" as you drag it left/up, though it never actually
  // resizes.
  function overlayHalfSizePercent(): { halfW: number; halfH: number } {
    const wrapperRect = wrapperRef.current?.getBoundingClientRect()
    const overlayRect = overlayImgRef.current?.getBoundingClientRect()
    if (!wrapperRect || !overlayRect || wrapperRect.width === 0 || wrapperRect.height === 0) {
      return { halfW: 0, halfH: 0 }
    }
    return {
      halfW: (overlayRect.width / 2 / wrapperRect.width) * 100,
      halfH: (overlayRect.height / 2 / wrapperRect.height) * 100,
    }
  }

  // Drag handling on the overlay
  const handleOverlayMouseDown = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      e.preventDefault()
      const rect = wrapperRef.current?.getBoundingClientRect()
      if (!rect) return
      setDragOffset({
        x: e.clientX - (rect.left + (overlayPos.x / 100) * rect.width),
        y: e.clientY - (rect.top + (overlayPos.y / 100) * rect.height),
      })
      setIsDragging(true)
    },
    [overlayPos],
  )

  useEffect(() => {
    if (!isDragging) return
    const onMove = (e: MouseEvent) => {
      const rect = wrapperRef.current?.getBoundingClientRect()
      if (!rect) return
      const { halfW, halfH } = overlayHalfSizePercent()
      const newX = clamp(((e.clientX - rect.left - dragOffset.x) / rect.width) * 100, halfW, 100 - halfW)
      const newY = clamp(((e.clientY - rect.top - dragOffset.y) / rect.height) * 100, halfH, 100 - halfH)
      setOverlayPos({ x: newX, y: newY })
    }
    const onUp = () => setIsDragging(false)
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [isDragging, dragOffset])

  // Touch drag support
  const handleOverlayTouchStart = useCallback(
    (e: React.TouchEvent<HTMLDivElement>) => {
      e.preventDefault()
      const touch = e.touches[0]
      const rect = wrapperRef.current?.getBoundingClientRect()
      if (!rect) return
      setDragOffset({
        x: touch.clientX - (rect.left + (overlayPos.x / 100) * rect.width),
        y: touch.clientY - (rect.top + (overlayPos.y / 100) * rect.height),
      })
      setIsDragging(true)
    },
    [overlayPos],
  )

  useEffect(() => {
    if (!isDragging) return
    // { passive: false } + preventDefault: without it, dragging the overlay
    // on a touchscreen also scrolls whatever's behind it (the page has no
    // way to know this touch is "spoken for" otherwise).
    const onMove = (e: TouchEvent) => {
      e.preventDefault()
      const touch = e.touches[0]
      const rect = wrapperRef.current?.getBoundingClientRect()
      if (!rect) return
      const { halfW, halfH } = overlayHalfSizePercent()
      const newX = clamp(((touch.clientX - rect.left - dragOffset.x) / rect.width) * 100, halfW, 100 - halfW)
      const newY = clamp(((touch.clientY - rect.top - dragOffset.y) / rect.height) * 100, halfH, 100 - halfH)
      setOverlayPos({ x: newX, y: newY })
    }
    const onEnd = () => setIsDragging(false)
    window.addEventListener('touchmove', onMove, { passive: false })
    window.addEventListener('touchend', onEnd)
    return () => {
      window.removeEventListener('touchmove', onMove)
      window.removeEventListener('touchend', onEnd)
    }
  }, [isDragging, dragOffset])

  // Close on Escape — the only keyboard affordance a Dialog would've given
  // us for free; everything else about this being a bespoke fullscreen
  // portal (not <Dialog>) is intentional, see the render below.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onOpenChange(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onOpenChange])

  async function handleCapture() {
    const video = videoRef.current
    const canvas = canvasRef.current
    const wrapper = wrapperRef.current
    if (!video || !canvas || !wrapper || !cameraReady) return
    setCapturing(true)

    const ctx = canvas.getContext('2d')
    if (!ctx) {
      setCapturing(false)
      return
    }

    const vw = video.videoWidth
    const vh = video.videoHeight
    const wrapperRect = wrapper.getBoundingClientRect()
    const cw = wrapperRect.width
    const ch = wrapperRect.height

    // Replicate the <video>'s object-cover crop in native pixel space, so
    // the still matches exactly what was on screen (and so overlayPos's 0-
    // 100% maps 1:1 onto it) — same reasoning as the couple-videos feed's
    // object-contain fix, mirrored for a `cover` preview instead of
    // `contain`. Without this, the capture used the full uncropped native
    // frame while the preview showed a cropped subset of it, so the mascot
    // landed in a different spot in the saved photo than where it was
    // dragged on screen.
    const coverScale = vw > 0 && vh > 0 && cw > 0 && ch > 0 ? Math.max(cw / vw, ch / vh) : 1
    const cropW = coverScale > 0 ? cw / coverScale : vw
    const cropH = coverScale > 0 ? ch / coverScale : vh
    const cropX = (vw - cropW) / 2
    const cropY = (vh - cropH) / 2

    canvas.width = Math.max(1, Math.round(cropW))
    canvas.height = Math.max(1, Math.round(cropH))

    ctx.save()
    if (facingMode === 'user') {
      // Preview is mirrored (scaleX(-1) below) for a natural selfie feel —
      // mirror the capture the same way so the still matches the preview.
      ctx.translate(canvas.width, 0)
      ctx.scale(-1, 1)
    }
    ctx.drawImage(video, cropX, cropY, cropW, cropH, 0, 0, canvas.width, canvas.height)
    ctx.restore()

    // Draw overlay at its dragged position — never mirrored, the sprite's
    // art has a fixed facing regardless of which camera is active.
    const overlayImg = document.createElement('img')
    overlayImg.crossOrigin = 'anonymous'
    overlayImg.src = activeOverlay.src
    await new Promise<void>((resolve) => {
      overlayImg.onload = () => resolve()
      overlayImg.onerror = () => resolve() // proceed even if GIF fails
    })

    const maxOverlayW = canvas.width * 0.45
    const scale = overlayImg.width > 0 ? maxOverlayW / overlayImg.width : 1
    const ow = overlayImg.width * scale
    const oh = overlayImg.height * scale
    const ox = (overlayPos.x / 100) * canvas.width - ow / 2
    const oy = (overlayPos.y / 100) * canvas.height - oh / 2

    ctx.drawImage(overlayImg, ox, oy, ow, oh)

    canvas.toBlob(async (blob) => {
      setCapturing(false)
      if (!blob) {
        toast.error('No se pudo capturar la foto')
        return
      }

      setUploading(true)
      try {
        const formData = new FormData()
        formData.append('photo', blob, 'pet-photo.jpg')
        const res = await api.post<{ url: string }>('/api/pet/photos', formData, { timeout: 120000 })
        // onPhotoTaken (MascotaTab) already shows its own success toast —
        // showing one here too meant every capture toasted twice.
        onPhotoTaken(res.data.url)
        onOpenChange(false)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'No se pudo subir la foto')
      } finally {
        setUploading(false)
      }
    }, 'image/jpeg', 0.9)
  }

  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 bg-black">
      {/* Fullscreen live preview — no dialog chrome, no boxed aspect ratio.
          Portaled straight to <body> for the same reason as the couple-
          videos feed: this needs to sit above AppLayout's padded <main>/
          TopBar entirely to actually feel like a native camera app. */}
      <div ref={wrapperRef} className="absolute inset-0">
        {cameraError ? (
          <div className="flex h-full items-center justify-center">
            <p className="p-4 text-center text-sm text-white/70">{cameraError}</p>
          </div>
        ) : (
          <>
            <video
              ref={videoRef}
              className="absolute inset-0 h-full w-full object-cover"
              style={{ transform: facingMode === 'user' ? 'scaleX(-1)' : undefined }}
              playsInline
              muted
            />
            {/* Overlay sprite — draggable */}
            <div
              className="absolute inset-0 select-none"
              style={{ cursor: isDragging ? 'grabbing' : 'grab', touchAction: 'none' }}
            >
              <div
                className="absolute pointer-events-none"
                style={{
                  left: `${overlayPos.x}%`,
                  top: `${overlayPos.y}%`,
                  transform: 'translate(-50%, -50%)',
                }}
                onMouseDown={handleOverlayMouseDown}
                onTouchStart={handleOverlayTouchStart}
              >
                <div className="relative">
                  <img
                    ref={overlayImgRef}
                    src={activeOverlay.src}
                    alt={activeOverlay.label}
                    className="block"
                    style={{
                      width: 'min(160px, 45vw)',
                      height: 'auto',
                      imageRendering: 'pixelated',
                      pointerEvents: 'auto',
                      cursor: isDragging ? 'grabbing' : 'grab',
                    }}
                    draggable={false}
                  />
                  {/* Drag hint */}
                  <div className="absolute -top-6 left-1/2 flex -translate-x-1/2 items-center gap-1 whitespace-nowrap text-xs text-white/70">
                    <Move className="h-3 w-3" />
                    <span className="font-pixel">Arrastra</span>
                  </div>
                </div>
              </div>
            </div>
            {/* Watermark hint */}
            <div className="pointer-events-none absolute bottom-40 right-3 opacity-50">
              <span className="font-pixel text-xs text-white">+{activeOverlay.label}</span>
            </div>
          </>
        )}
      </div>

      {/* Close — top-left, same treatment as the couple-videos feed's back
          button, for the same reason: no other chrome around to fall back
          on once this covers the whole screen. */}
      <button
        type="button"
        onClick={() => onOpenChange(false)}
        aria-label="Cerrar cámara"
        className="fixed left-4 top-[calc(env(safe-area-inset-top)+1rem)] z-20 flex h-10 w-10 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-md transition-colors hover:bg-black/70"
      >
        <X className="h-5 w-5" />
      </button>

      {/* Flip camera — top-right */}
      {!cameraError && (
        <button
          type="button"
          onClick={() => setFacingMode((m) => (m === 'environment' ? 'user' : 'environment'))}
          aria-label="Cambiar de cámara"
          title="Cambiar de cámara"
          className="fixed right-4 top-[calc(env(safe-area-inset-top)+1rem)] z-20 flex h-10 w-10 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-md transition-colors hover:bg-black/70"
        >
          <SwitchCamera className="h-5 w-5" />
        </button>
      )}

      {/* Bottom control bar: expression picker + centrar/capture — a solid
          backdrop over the live video so it stays legible regardless of
          what's behind it, same idea as a native camera app's bottom sheet. */}
      <div className="absolute inset-x-0 bottom-0 z-10 bg-black/60 pb-[env(safe-area-inset-bottom)] backdrop-blur-sm">
        <div className="px-4 pt-3">
          <div className="mb-2 flex items-center gap-1">
            <Image className="h-4 w-4 text-white/70" />
            <span className="font-pixel text-xs tracking-wide text-white/70">Elegir expresión</span>
          </div>
          <div className="flex gap-2 overflow-x-auto pb-1">
            {OVERLAY_OPTIONS.map((opt) => (
              <button
                key={opt.src}
                type="button"
                onClick={() => setActiveOverlay(opt)}
                className={cn(
                  'flex shrink-0 flex-col items-center gap-1 rounded-sm border-2 p-1.5 transition-transform active:scale-95',
                  activeOverlay.src === opt.src
                    ? 'border-[var(--chart-3)] bg-[var(--chart-3)]/20'
                    : 'border-transparent bg-white/10 hover:border-white/30',
                )}
                title={opt.label}
              >
                <img
                  src={opt.src}
                  alt={opt.label}
                  className="block"
                  style={{ width: 36, height: 36, objectFit: 'contain', imageRendering: 'pixelated' }}
                />
              </button>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-2 px-4 py-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setOverlayPos({ x: 50, y: 50 })}
            className="gap-1.5 font-pixel text-xs tracking-wide text-white hover:bg-white/10 hover:text-white"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            Centrar
          </Button>
          <div className="flex-1" />
          <Button
            onClick={handleCapture}
            disabled={!cameraReady || capturing || uploading}
            className="gap-2 font-pixel tracking-wide"
          >
            {uploading ? 'Subiendo…' : capturing ? 'Capturando…' : (
              <>
                <Camera className="h-4 w-4" />
                Capturar
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Hidden canvas for capture composition */}
      <canvas ref={canvasRef} className="hidden" />
    </div>,
    document.body,
  )
}
