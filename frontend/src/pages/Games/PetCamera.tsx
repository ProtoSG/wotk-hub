import { useEffect, useRef, useState, useCallback } from 'react'
import { toast } from 'sonner'
import { Camera, X, Move, Image, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
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

  const [activeOverlay, setActiveOverlay] = useState(OVERLAY_OPTIONS[0])
  const [overlayPos, setOverlayPos] = useState({ x: 50, y: 50 }) // percentage
  const [isDragging, setIsDragging] = useState(false)
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 })
  const [cameraReady, setCameraReady] = useState(false)
  const [cameraError, setCameraError] = useState<string | null>(null)
  const [capturing, setCapturing] = useState(false)
  const [uploading, setUploading] = useState(false)

  // Start camera when dialog opens
  useEffect(() => {
    if (!open) return
    // Defer state updates to avoid cascading renders within the same effect
    const initCamera = () => {
      setCameraError(null)
      setCameraReady(false)
      setActiveOverlay(OVERLAY_OPTIONS.find(o => o.src === MOOD_SPRITE[mood]) ?? OVERLAY_OPTIONS[0])
      setOverlayPos({ x: 50, y: 50 })
    }
    queueMicrotask(initCamera)

    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: 'environment' } })
      .then((stream) => {
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
        setCameraError('No se pudo acceder a la cámara')
      })

    return () => {
      streamRef.current?.getTracks().forEach((t) => t.stop())
      streamRef.current = null
    }
  }, [open, mood])

  // Drag handling on the overlay
  const handleOverlayMouseDown = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      e.preventDefault()
      const rect = e.currentTarget.parentElement?.getBoundingClientRect()
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
      const video = videoRef.current?.parentElement
      if (!video) return
      const rect = video.getBoundingClientRect()
      const newX = Math.max(0, Math.min(100, ((e.clientX - rect.left - dragOffset.x) / rect.width) * 100))
      const newY = Math.max(0, Math.min(100, ((e.clientY - rect.top - dragOffset.y) / rect.height) * 100))
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
      const touch = e.touches[0]
      const rect = e.currentTarget.parentElement?.getBoundingClientRect()
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
    const onMove = (e: TouchEvent) => {
      const touch = e.touches[0]
      const video = videoRef.current?.parentElement
      if (!video) return
      const rect = video.getBoundingClientRect()
      const newX = Math.max(0, Math.min(100, ((touch.clientX - rect.left - dragOffset.x) / rect.width) * 100))
      const newY = Math.max(0, Math.min(100, ((touch.clientY - rect.top - dragOffset.y) / rect.height) * 100))
      setOverlayPos({ x: newX, y: newY })
    }
    const onEnd = () => setIsDragging(false)
    window.addEventListener('touchmove', onMove)
    window.addEventListener('touchend', onEnd)
    return () => {
      window.removeEventListener('touchmove', onMove)
      window.removeEventListener('touchend', onEnd)
    }
  }, [isDragging, dragOffset])

  async function handleCapture() {
    const video = videoRef.current
    const canvas = canvasRef.current
    if (!video || !canvas || !cameraReady) return
    setCapturing(true)

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    canvas.width = video.videoWidth || 640
    canvas.height = video.videoHeight || 480

    // Draw camera frame
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height)

    // Draw overlay at its dragged position
    const overlayImg = document.createElement('img')
    overlayImg.crossOrigin = 'anonymous'
    overlayImg.src = activeOverlay.src
    await new Promise<void>((resolve) => {
      overlayImg.onload = () => resolve()
      overlayImg.onerror = () => resolve() // proceed even if GIF fails
    })

    const maxOverlayW = canvas.width * 0.45
    const scale = maxOverlayW / overlayImg.width
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
        const res = await fetch('/api/pet/photos', {
          method: 'POST',
          credentials: 'include',
          body: formData,
        })
        if (!res.ok) {
          const err = await res.json().catch(() => ({ message: 'Error uploading photo' }))
          throw new Error(err.message || 'Error uploading photo')
        }
        const data = await res.json()
        toast.success('¡Foto guardada!')
        onPhotoTaken(data.url)
        onOpenChange(false)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'No se pudo subir la foto')
      } finally {
        setUploading(false)
      }
    }, 'image/jpeg', 0.9)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg p-0 gap-0 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between border-b-2 px-4 py-3">
          <div className="flex items-center gap-2">
            <Camera className="h-5 w-5 text-[var(--chart-3)]" />
            <span className="font-pixel text-lg tracking-wide">Foto con la mascota</span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onOpenChange(false)}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Camera preview with draggable overlay */}
        <div className="relative bg-black" style={{ aspectRatio: '4/3' }}>
          {cameraError ? (
            <div className="flex h-full items-center justify-center">
              <p className="text-center text-sm text-muted-foreground p-4">{cameraError}</p>
            </div>
          ) : (
            <>
              <video
                ref={videoRef}
                className="h-full w-full object-cover"
                playsInline
                muted
              />
              {/* Overlay sprite — draggable */}
              <div
                className="absolute inset-0 select-none"
                style={{ cursor: isDragging ? 'grabbing' : 'grab' }}
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
                    <div className="absolute -top-6 left-1/2 -translate-x-1/2 flex items-center gap-1 text-white/70 text-xs whitespace-nowrap">
                      <Move className="h-3 w-3" />
                      <span className="font-pixel">Arrastra</span>
                    </div>
                  </div>
                </div>
              </div>
              {/* Watermark hint */}
              <div className="absolute bottom-2 right-2 opacity-50">
                <span className="font-pixel text-xs text-white">+{activeOverlay.label}</span>
              </div>
            </>
          )}
        </div>

        {/* GIF/overlay selector */}
        <div className="border-t-2 px-4 py-3">
          <div className="flex items-center gap-1 mb-2">
            <Image className="h-4 w-4 text-muted-foreground" />
            <span className="font-pixel text-xs tracking-wide text-muted-foreground">Elegir expresión</span>
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
                    ? 'border-[var(--chart-3)] bg-[var(--chart-3)]/10'
                    : 'border-transparent bg-muted/50 hover:border-border',
                )}
                title={opt.label}
              >
                <img
                  src={opt.src}
                  alt={opt.label}
                  className="block"
                  style={{
                    width: 36,
                    height: 36,
                    objectFit: 'contain',
                    imageRendering: 'pixelated',
                  }}
                />
              </button>
            ))}
          </div>
        </div>

        {/* Reset position + capture */}
        <div className="flex items-center gap-2 border-t-2 px-4 py-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setOverlayPos({ x: 50, y: 50 })}
            className="gap-1.5 font-pixel text-xs tracking-wide"
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

        {/* Hidden canvas for capture composition */}
        <canvas ref={canvasRef} className="hidden" />
      </DialogContent>
    </Dialog>
  )
}
