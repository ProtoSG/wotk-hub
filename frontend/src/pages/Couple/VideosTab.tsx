import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
// No reasonable Solar Linear equivalent for a "no video" glyph exists
// (verified: no video/camera icon with an off/slash/cross/remove variant
// in @iconify-json/solar) — left on lucide-react per the rare-exception rule.
import { ChevronLeft, Loader2, Pause, Play, Trash2, Upload, VideoOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { FloatingActionButton } from '@/components/ui/floating-action-button'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import type { CoupleDate, Video } from '@/types/couple.types'
import { datesKey, videoFeedKey } from './coupleKeys'

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
  // Returns to the "citas" tab. VideosTab renders through a portal straight
  // to <body>, so it sits above AppLayout's Sidebar/TopBar/padding entirely —
  // there's no surrounding chrome left to provide a way back, so the feed
  // needs its own back button wired to this.
  onExit: () => void
}

// TikTok/Reels-style vertical scroll feed — one full-screen video per snap point.
// Rendered through a portal to document.body (not inline in the tab tree):
// AppLayout wraps routes in <main className="p-6"> under a TopBar, and
// CouplePage's TabsContent adds its own mt-4/animate-in transform — every one
// of those clips or insets a plain `fixed`/`100dvh` element, since a CSS
// transform on any ancestor (even the identity one animate-in leaves behind)
// creates a new containing block. Portaling to <body> sidesteps all of it and
// gets a true edge-to-edge overlay, same as native TikTok/Reels.
export default function VideosTab({ canManage, onExit }: Props) {
  const [activeIndex, setActiveIndex] = useState(0)
  const [playingId, setPlayingId] = useState<number | null>(null)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [selectedDateId, setSelectedDateId] = useState('')
  const [selectedFileName, setSelectedFileName] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const videoRefs = useRef<Map<number, HTMLVideoElement>>(new Map())

  const { listDates, listVideos, uploadVideo, deleteVideo } = useCoupleApi()
  const queryClient = useQueryClient()

  const { data: dates = [], isPending: datesPending } = useQuery({
    queryKey: datesKey(),
    queryFn: () => listDates(),
  })

  const { data: entries = [], isPending: entriesPending } = useQuery({
    queryKey: videoFeedKey(),
    queryFn: async () => {
      const perDate = await Promise.all(
        dates.map(async (date) => {
          const videos = await listVideos(date.id)
          return videos.map((video) => ({ video, date }))
        }),
      )
      return perDate.flat().sort((a, b) => b.video.createdAt.localeCompare(a.video.createdAt))
    },
    enabled: dates.length > 0,
  })

  const loading = datesPending || (dates.length > 0 && entriesPending)

  const uploadMutation = useMutation({
    mutationFn: ({ dateId, file }: { dateId: number; file: File }) => uploadVideo(dateId, file),
    onSuccess: (video, { dateId }) => {
      const date = dates.find((d) => d.id === dateId)
      if (date) {
        queryClient.setQueryData<VideoEntry[]>(videoFeedKey(), (prev = []) => [{ video, date }, ...prev])
      }
      toast.success('¡Video subido!')
      setUploadOpen(false)
      setSelectedDateId('')
      setSelectedFileName('')
      if (fileInputRef.current) fileInputRef.current.value = ''
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo subir el video')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (entry: VideoEntry) => deleteVideo(entry.date.id, entry.video.id),
    onSuccess: (_data, entry) => {
      queryClient.setQueryData<VideoEntry[]>(videoFeedKey(), (prev = []) =>
        prev.filter((e) => e.video.id !== entry.video.id)
      )
      toast.success('Video eliminado')
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el video')
    },
  })

  // Pause videos that are no longer in view
  useEffect(() => {
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
      video.play().catch(() => {
        toast.error('No se pudo reproducir el video')
      })
      setPlayingId(entry.video.id)
    } else {
      video.pause()
      setPlayingId(null)
    }
  }

  function handleUpload() {
    const file = fileInputRef.current?.files?.[0]
    const dateId = Number(selectedDateId)
    if (!file || !dateId) return
    uploadMutation.mutate({ dateId, file })
  }

  function handleDelete(entry: VideoEntry) {
    deleteMutation.mutate(entry)
  }

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    setSelectedFileName(e.target.files?.[0]?.name ?? '')
  }

  if (loading) {
    return createPortal(
      <div className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-3 bg-black text-muted-foreground">
        <BackButton onExit={onExit} />
        <Loader2 className="h-6 w-6 animate-spin" />
      </div>,
      document.body
    )
  }

  if (entries.length === 0) {
    return createPortal(
      <div className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-3 bg-black text-center text-muted-foreground">
        <BackButton onExit={onExit} />
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
          uploading={uploadMutation.isPending}
          onUpload={handleUpload}
          dates={dates}
        />
      </div>,
      document.body
    )
  }

  return createPortal(
    <div className="fixed inset-0 z-50 bg-black">
      <BackButton onExit={onExit} />

      {/* Vertical snap scroll — TikTok/Reels style, truly full-screen with 100dvh */}
      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="h-full overflow-y-scroll overscroll-none"
        style={{ scrollSnapType: 'y mandatory' }}
      >
        {entries.map((entry, index) => {
          const isPlaying = playingId === entry.video.id

          return (
            <div
              key={entry.video.id}
              className="relative w-full shrink-0 snap-start"
              style={{ height: '100dvh', scrollSnapAlign: 'start' }}
            >
              {/*
                VIDEO — always rendered, full-screen.
                It's UNDER the thumbnail layer in z-order when paused.
                When isPlaying, thumbnail lifts (opacity 0) so video shows through.
                preload="metadata" fetches just enough to show first frame fast.
              */}
              <video
                ref={(el) => {
                  if (el) videoRefs.current.set(entry.video.id, el)
                  else videoRefs.current.delete(entry.video.id)
                }}
                src={entry.video.url}
                // object-contain, not object-cover: backend only scales
                // height to 720 (scale=-2:720 in transcode720p), it never
                // forces a portrait canvas, so a landscape upload stays
                // landscape. Cover would crop its sides off to fill a 9:16
                // screen; contain letterboxes instead — matches native
                // TikTok/Reels behavior for non-vertical clips.
                className="absolute inset-0 h-full w-full object-contain"
                preload="metadata"
                playsInline
                // Not muted: play() only ever runs from togglePlay, itself
                // fired by a user click on the play button — a real user
                // gesture, so browser autoplay-with-sound restrictions don't
                // apply here.
                loop
                onEnded={() => setPlayingId(null)}
                onLoadedData={(e) => {
                  // Once first frame loads, show thumbnail as backdrop
                  ;(e.target as HTMLVideoElement).poster = entry.video.thumbnailUrl
                }}
              />

              {/*
                THUMBNAIL — on top of the video when paused.
                When playing, fades out to reveal the video underneath.
              */}
              <img
                src={entry.video.thumbnailUrl}
                alt=""
                className={`absolute inset-0 h-full w-full object-contain transition-opacity duration-300 ${
                  isPlaying ? 'pointer-events-none opacity-0' : 'opacity-100'
                }`}
                loading={index <= 2 ? 'eager' : 'lazy'}
              />

              {/* Dark scrim for caption legibility */}
              <div className="absolute inset-0 bg-gradient-to-b from-black/20 via-transparent to-black/70" />

              {/* Play button overlay — visible when paused */}
              {!isPlaying && (
                <button
                  type="button"
                  className="absolute inset-0 flex items-center justify-center"
                  onClick={() => togglePlay(entry)}
                  aria-label="Reproducir video"
                >
                  <div className="flex h-20 w-20 items-center justify-center rounded-full bg-black/50 backdrop-blur-md transition-transform hover:scale-110">
                    <Play className="h-10 w-10 fill-white text-white" />
                  </div>
                </button>
              )}

              {/* Bottom overlay: caption + controls */}
              <div className="absolute bottom-0 left-0 right-0 p-4 pb-8">
                <div className="flex items-end justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-white drop-shadow-lg">
                      {entry.date.place || '—'}
                    </p>
                    <p className="truncate text-xs text-white/70 drop-shadow-sm">
                      {entry.date.notes || entry.date.occurredOn}
                    </p>
                  </div>

                  <div className="flex flex-col items-center gap-3 shrink-0">
                    {/* Duration */}
                    <span className="rounded bg-black/60 px-2 py-0.5 text-xs font-medium text-white backdrop-blur-sm">
                      {formatDuration(entry.video.durationSeconds)}
                    </span>

                    {/* Pause button when playing */}
                    {isPlaying && (
                      <button
                        type="button"
                        onClick={() => togglePlay(entry)}
                        className="flex h-12 w-12 items-center justify-center rounded-full bg-black/50 backdrop-blur-md transition-colors hover:bg-black/70"
                        aria-label="Pausar video"
                      >
                        <Pause className="h-6 w-6 fill-white text-white" />
                      </button>
                    )}

                    {/* Admin delete */}
                    {canManage && (
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          handleDelete(entry)
                        }}
                        disabled={deleteMutation.isPending && deleteMutation.variables?.video.id === entry.video.id}
                        className="flex h-12 w-12 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-md transition-colors hover:bg-red-500/60 disabled:opacity-50"
                        aria-label="Eliminar video"
                      >
                        {deleteMutation.isPending && deleteMutation.variables?.video.id === entry.video.id ? (
                          <Loader2 className="h-5 w-5 animate-spin" />
                        ) : (
                          <Trash2 className="h-5 w-5" />
                        )}
                      </button>
                    )}
                  </div>
                </div>
              </div>

              {/* Scroll hint — only on first video, when paused */}
              {index === 0 && !isPlaying && (
                <div className="pointer-events-none absolute bottom-28 left-1/2 -translate-x-1/2 animate-bounce">
                  <div className="flex flex-col items-center gap-1 text-white/50">
                    <span className="text-xs">deslizá</span>
                    <div className="h-5 w-px bg-white/40" />
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Counter pill */}
      <div className="pointer-events-none fixed top-[calc(env(safe-area-inset-top)+1rem)] left-1/2 z-10 -translate-x-1/2 rounded-full bg-black/60 px-3 py-1 text-xs font-medium text-white backdrop-blur-sm">
        {activeIndex + 1} / {entries.length}
      </div>

      {/* Upload FAB — parked top-right (mirrors BackButton on the left) instead
          of its usual bottom-right spot: down there it collides with the
          per-video action rail (pause/delete), which claims that corner in
          this feed. */}
      {canManage && (
        <FloatingActionButton
          label="Subir video"
          className="top-[calc(env(safe-area-inset-top)+1rem)] bottom-auto"
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
        uploading={uploadMutation.isPending}
        onUpload={handleUpload}
        dates={dates}
      />
    </div>,
    document.body
  )
}

// Back button — top-left, mirrors the close affordance of native
// TikTok/Reels. Needed because the fullscreen portal covers TopBar/Sidebar,
// so there's no other way back to "citas" while a video is open.
function BackButton({ onExit }: { onExit: () => void }) {
  return (
    <button
      type="button"
      onClick={onExit}
      aria-label="Volver"
      className="fixed left-4 top-[calc(env(safe-area-inset-top)+1rem)] z-20 flex h-10 w-10 items-center justify-center rounded-full bg-black/50 text-white backdrop-blur-md transition-colors hover:bg-black/70"
    >
      <ChevronLeft className="h-6 w-6" />
    </button>
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
