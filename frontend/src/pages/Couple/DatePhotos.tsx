import { ImageOff, Loader2, Trash2, Upload } from 'lucide-react'
import { useRef, useState, type ChangeEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import type { CoupleDatePhoto } from '@/types/couple.types'
import { photosKey } from './coupleKeys'

// Matches the backend's allowedPhotoTypes in modules/couple/photos.go.
const ACCEPTED_TYPES = 'image/jpeg,image/png,image/webp,image/gif'

interface Props {
  dateId: number
}

// Gallery + upload control for a couple date's photos. Only rendered for an
// existing date (needs a dateId) — a new, unsaved date has nowhere to
// attach photos to yet. No lightbox library: clicking a thumbnail opens the
// same full-size image in a plain dialog, since this app doesn't need one.
export default function DatePhotos({ dateId }: Props) {
  const [uploadProgress, setUploadProgress] = useState<{ done: number; total: number } | null>(null)
  const [preview, setPreview] = useState<CoupleDatePhoto | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { listPhotos, uploadPhoto, deletePhoto } = useCoupleApi()
  const queryClient = useQueryClient()

  const { data: photos = [], isPending } = useQuery({
    queryKey: photosKey(dateId),
    queryFn: () => listPhotos(dateId),
  })

  // Multiple files upload concurrently rather than one-at-a-time — each is
  // an independent POST, so a slow/failing file doesn't block the rest.
  // Progress is a simple done/total counter rather than per-file percentage;
  // good enough for the handful of photos someone picks from a gallery at
  // once, and much simpler than tracking individual upload progress events.
  const uploadMutation = useMutation({
    mutationFn: async (files: File[]) => {
      setUploadProgress({ done: 0, total: files.length })
      return Promise.allSettled(
        files.map((file) =>
          uploadPhoto(dateId, file).then((photo) => {
            queryClient.setQueryData<CoupleDatePhoto[]>(photosKey(dateId), (prev = []) => [photo, ...prev])
            setUploadProgress((prev) => (prev ? { ...prev, done: prev.done + 1 } : prev))
            return photo
          })
        )
      )
    },
    onSettled: () => setUploadProgress(null),
  })

  const deleteMutation = useMutation({
    mutationFn: (photo: CoupleDatePhoto) => deletePhoto(dateId, photo.id),
    onSuccess: (_data, photo) => {
      queryClient.setQueryData<CoupleDatePhoto[]>(photosKey(dateId), (prev = []) =>
        prev.filter((p) => p.id !== photo.id)
      )
      setPreview((prev) => (prev?.id === photo.id ? null : prev))
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la foto')
    },
  })

  async function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? [])
    e.target.value = ''
    if (files.length === 0) return

    const results = await uploadMutation.mutateAsync(files)

    const failed = results.filter((r) => r.status === 'rejected').length
    if (failed > 0) {
      const detail =
        failed === 1 && results.length === 1 && results[0].status === 'rejected'
          ? results[0].reason instanceof Error
            ? results[0].reason.message
            : undefined
          : undefined
      toast.error(
        detail ??
          (failed === results.length
            ? 'No se pudo subir ninguna foto'
            : `${failed} de ${results.length} fotos no se pudieron subir`)
      )
    }
  }

  function handleDelete(photo: CoupleDatePhoto) {
    deleteMutation.mutate(photo)
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>Fotos</Label>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={uploadProgress != null}
          onClick={() => fileInputRef.current?.click()}
        >
          {uploadProgress != null ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Upload className="h-4 w-4" />}
          {uploadProgress != null ? `Subiendo ${uploadProgress.done}/${uploadProgress.total}…` : 'Agregar fotos'}
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          accept={ACCEPTED_TYPES}
          multiple
          className="hidden"
          onChange={handleFileChange}
        />
      </div>

      {isPending ? (
        <div className="flex items-center justify-center py-6 text-muted-foreground">
          <Loader2 className="h-[18px] w-[18px] animate-spin" />
        </div>
      ) : photos.length === 0 ? (
        <div className="flex flex-col items-center gap-1 rounded-md border border-dashed py-6 text-muted-foreground">
          <ImageOff className="h-5 w-5" />
          <p className="text-xs">Todavía no hay fotos</p>
        </div>
      ) : (
        <div className="grid grid-cols-4 gap-2">
          {photos.map((p) => (
            <div key={p.id} className="group relative aspect-square overflow-hidden rounded-md border">
              <button type="button" className="h-full w-full" onClick={() => setPreview(p)}>
                <img src={p.thumbnailUrl} alt="" className="h-full w-full object-cover" loading="lazy" />
              </button>
              <button
                type="button"
                aria-label="Eliminar foto"
                disabled={deleteMutation.isPending && deleteMutation.variables?.id === p.id}
                onClick={() => handleDelete(p)}
                className="absolute right-1 top-1 rounded-full bg-black/60 p-1 text-white opacity-0 transition-opacity group-hover:opacity-100 disabled:opacity-100"
              >
                {deleteMutation.isPending && deleteMutation.variables?.id === p.id ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Trash2 className="h-3 w-3" />
                )}
              </button>
            </div>
          ))}
        </div>
      )}

      <Dialog open={preview != null} onOpenChange={(v) => !v && setPreview(null)}>
        <DialogContent className="sm:max-w-2xl">
          {preview && (
            <img src={preview.url} alt="" className="max-h-[70vh] w-full rounded-md object-contain" />
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
