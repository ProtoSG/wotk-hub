import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import { toast } from 'sonner'
import { Loader2, Pause, Play, Trash2, Upload, VideoOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { FloatingActionButton } from '@/components/ui/floating-action-button'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import type { CoupleDate, Video } from '@/types/couple.types'

// Matches the backend's allowedVideoTypes in modules/couple/videos.go.
const ACCEPTED_TYPES = 'video/mp4,video/quicktime,video/x-msvideo,video/webm'

interface VideoEntry {
  video: Video
  date: CoupleDate
}

function formatDuration(totalSeconds: number): string {
  const seconds = Math.max(0, Math.round(totalSeconds))
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

interface Props {
  canManage: boolean
}

// TikTok/Reels-style vertical scroll feed — one full-width video per snap point.
// autoplay is intentionally NOT used on scroll: it causes expensive network requests
// and jarring audio interruptions. Users tap to play/pause.
export default function VideosTab({ canManage }: Props) {
  const [entries, setEntries] = useState<VideoEntry[]>([])
  const [dates, setDates] = useState<CoupleDate[]>([])
  const [loading, setLoading] = useState(true)
  const [activeIndex, setActiveIndex] = useState(0)
  const [playingId, setPlayingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [selectedDateId, setSelectedDateId] = useState('')
  const [selectedFileName, setSelectedFileName] = useState('')
  const [uploading, setUploading] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const videoRefs = useRef<Map<number, HTMLVideoElement>>(new Map())

  const { listDates, listVideos, uploadVideo, deleteVideo } = useCoupleApi()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const allDates = await listDates()
      setDates(allDates)
      const perDate = await Promise.all(
        allDates.map(async (date) => {
          const videos = await listVideos(date.id)
          return videos.map((video) => ({ video, date }))
        }),
      )
      const flat = perDate
        .flat()
        .sort((a, b) => b.video.createdAt.localeCompare(a.video.createdAt))
      setEntries(flat)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar los videos')
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount
    load()
  }, [load])

  // Sync playing state with active index
  useEffect(() => {
    // Pause all videos not at the active index
    videoRefs.current.forEach((video, id) => {
      const entry = entries.find((e) => e.video.id === id)
      if (!entry) return
      const entryIndex = entries.indexOf(entry)
      if (entryIndex !== activeIndex && video && !video.paused) {
        video.pause()
      }
    })
  }, [activeIndex, entries])

  function onScroll() {
    if (!scrollRef.current) return
    const el = scrollRef.current
    const scrollTop = el.scrollTop
    const itemHeight = el.clientHeight
    if (itemHeight === 0) return
    const idx = Math.round(scrollTop / itemHeight)
    setActiveIndex(idx)
  }

  function togglePlay(entry: VideoEntry) {
    const video = videoRefs.current.get(entry.video.id)
    if (!video) return
    if (video.paused) {
      video.play()
      setPlayingId(entry.video.id)
    } else {
      video.pause()
      setPlayingId(null)
    }
  }

  async function handleUpload() {
    const file = fileInputRef.current?.files?.[0]
    const dateId = Number(selectedDateId)
    if (!file || !dateId) return

    setUploading(true)
    try {
      const video = await uploadVideo(dateId, file)
      const date = dates.find((d) => d.id === dateId)
      if (date) {
        setEntries((prev) => [{ video, date }, ...prev])
      }
      toast.success('¡Video subido!')
      setUploadOpen(false)
      setSelectedDateId('')
      setSelectedFileName('')
      if (fileInputRef.current) fileInputRef.current.value = ''
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo subir el video')
    } finally {
      setUploading(false)
    }
  }

  async function handleDelete(entry: VideoEntry) {
    setDeletingId(entry.video.id)
    try {
      await deleteVideo(entry.date.id, entry.video.id)
      setEntries((prev) => prev.filter((e) => e.video.id !== entry.video.id))
      toast.success('Video eliminado')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el video')
    } finally {
      setDeletingId(null)
    }
  }

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    setSelectedFileName(e.target.files?.[0]?.name ?? '')
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <Loader2 size={24} className="animate-spin" />
      </div>
    )
  }

  if (entries.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-center text-muted-foreground">
        <VideoOff className="h-10 w-10" />
        <p className="text-base font-medium">Todavía no hay videos</p>
        {canManage && <p className="text-sm">Usá el botón + para subir el primero</p>}
        {canManage && (
          <FloatingActionButton
            label="Subir video"
            onClick={() => {
              setSelectedDateId('')
              setSelectedFileName('')
              setUploadOpen(true)
            }}
          />
        )}
        <UploadDialog
          uploadOpen={uploadOpen}
          setUploadOpen={setUploadOpen}
          selectedDateId={selectedDateId}
          setSelectedDateId={setSelectedDateId}
          selectedFileName={selectedFileName}
          fileInputRef={fileInputRef}
          onFileChange={handleFileChange}
          uploading={uploading}
          onUpload={handleUpload}
          dates={dates}
        />
      </div>
    )
  }

  return (
    <>
      {/* Vertical snap scroll — TikTok/Reels style */}
      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="h-full snap-y snap-mandatory overflow-y-scroll overscroll-none"
        style={{ scrollSnapType: 'y mandatory' }}
      >
        {entries.map((entry, index) => {
          const isPlaying = playingId === entry.video.id

          return (
            <div
              key={entry.video.id}
              className="relative h-screen w-full shrink-0 snap-start"
              style={{ scrollSnapAlign: 'start' }}
            >
              {/* Thumbnail fills the card */}
              <img
                src={entry.video.thumbnailUrl}
                alt=""
                className="absolute inset-0 h-full w-full object-cover"
                loading={index <= 1 ? 'eager' : 'lazy'}
              />

              {/* Dark scrim so caption is always readable */}
              <div className="absolute inset-0 bg-gradient-to-b from-black/30 via-transparent to-black/60" />

              {/* Play/pause tap target — entire card */}
              <button
                type="button"
                className="absolute inset-0 flex items-center justify-center"
                onClick={() => togglePlay(entry)}
                aria-label={isPlaying ? 'Pausar video' : 'Reproducir video'}
              >
                {/* Play indicator — shown when paused */}
                {!isPlaying && (
                  <div className="flex h-16 w-16 items-center justify-center rounded-full bg-black/40 backdrop-blur-sm transition-transform hover:scale-110">
                    <Play className="h-8 w-8 fill-white text-white" strokeWidth={0} />
                  </div>
                )}
              </button>

              {/* Hidden video — activated on play */}
              <video
                ref={(el) => {
                  if (el) videoRefs.current.set(entry.video.id, el)
                  else videoRefs.current.delete(entry.video.id)
                }}
                src={entry.video.url}
                className="absolute inset-0 h-full w-full object-cover"
                preload="none"
                playsInline
                muted
                loop
                onEnded={() => setPlayingId(null)}
              />

              {/* Bottom overlay: caption + duration */}
              <div className="absolute bottom-0 left-0 right-0 p-4 pb-6">
                <div className="flex items-end justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-white drop-shadow-md">
                      {entry.date.place || '—'}
                    </p>
                    <p className="truncate text-xs text-white/70 drop-shadow-sm">
                      {entry.date.notes || entry.date.occurredOn}
                    </p>
                  </div>
                  <div className="flex flex-col items-center gap-3">
                    {/* Duration badge */}
                    <span className="rounded bg-black/60 px-2 py-0.5 text-xs font-medium text-white backdrop-blur-sm">
                      {formatDuration(entry.video.durationSeconds)}
                    </span>
                    {/* Play state indicator */}
                    {isPlaying && (
                      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-black/40 backdrop-blur-sm">
                        <Pause className="h-5 w-5 fill-white text-white" strokeWidth={0} />
                      </div>
                    )}
                    {/* Admin delete */}
                    {canManage && (
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          handleDelete(entry)
                        }}
                        disabled={deletingId === entry.video.id}
                        className="flex h-10 w-10 items-center justify-center rounded-full bg-black/40 text-white backdrop-blur-sm transition-colors hover:bg-red-500/60 disabled:opacity-50"
                        aria-label="Eliminar video"
                      >
                        {deletingId === entry.video.id ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <Trash2 className="h-4 w-4" />
                        )}
                      </button>
                    )}
                  </div>
                </div>
              </div>

              {/* Scroll indicator for first video */}
              {index === 0 && !isPlaying && (
                <div className="pointer-events-none absolute bottom-20 left-1/2 -translate-x-1/2 animate-bounce">
                  <div className="flex flex-col items-center gap-1 text-white/60">
                    <span className="text-xs">deslizá</span>
                    <div className="h-4 w-px bg-white/40" />
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Video counter pill */}
      <div className="pointer-events-none absolute top-4 left-1/2 z-10 -translate-x-1/2 rounded-full bg-black/60 px-3 py-1 text-xs font-medium text-white backdrop-blur-sm">
        {activeIndex + 1} / {entries.length}
      </div>

      {/* Upload FAB */}
      {canManage && (
        <FloatingActionButton
          label="Subir video"
          onClick={() => {
            setSelectedDateId('')
            setSelectedFileName('')
            setUploadOpen(true)
          }}
        />
      )}

      {/* Upload dialog */}
      <UploadDialog
        uploadOpen={uploadOpen}
        setUploadOpen={setUploadOpen}
        selectedDateId={selectedDateId}
        setSelectedDateId={setSelectedDateId}
        selectedFileName={selectedFileName}
        fileInputRef={fileInputRef}
        onFileChange={handleFileChange}
        uploading={uploading}
        onUpload={handleUpload}
        dates={dates}
      />
    </>
  )
}

// Extracted to keep VideosTab readable — pure presentational dialog
function UploadDialog({
  uploadOpen,
  setUploadOpen,
  selectedDateId,
  setSelectedDateId,
  selectedFileName,
  fileInputRef,
  onFileChange,
  uploading,
  onUpload,
  dates,
}: {
  uploadOpen: boolean
  setUploadOpen: (v: boolean) => void
  selectedDateId: string
  setSelectedDateId: (v: string) => void
  selectedFileName: string
  fileInputRef: React.RefObject<HTMLInputElement | null>
  onFileChange: (e: ChangeEvent<HTMLInputElement>) => void
  uploading: boolean
  onUpload: () => void
  dates: CoupleDate[]
}) {
  return (
    <Dialog open={uploadOpen} onOpenChange={(v) => !uploading && setUploadOpen(v)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Subir video</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1">
            <Label>Cita</Label>
            <Select value={selectedDateId} onValueChange={setSelectedDateId}>
              <SelectTrigger>
                <SelectValue placeholder="Selecciona una cita" />
              </SelectTrigger>
              <SelectContent>
                {dates.map((d) => (
                  <SelectItem key={d.id} value={String(d.id)}>
                    {d.place || '—'} · {d.occurredOn}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Archivo</Label>
            <input
              ref={fileInputRef}
              type="file"
              accept={ACCEPTED_TYPES}
              onChange={onFileChange}
              disabled={uploading}
              className="block w-full text-sm text-muted-foreground file:mr-3 file:rounded-md file:border-0 file:bg-secondary file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-secondary-foreground hover:file:bg-secondary/80"
            />
            {selectedFileName && (
              <p className="truncate text-xs text-muted-foreground">{selectedFileName}</p>
            )}
          </div>
          {uploading && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Subiendo video…
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setUploadOpen(false)} disabled={uploading}>
            Cancelar
          </Button>
          <Button
            onClick={onUpload}
            disabled={uploading || !selectedDateId || !selectedFileName}
            className="gap-2"
          >
            {uploading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
            Subir
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
