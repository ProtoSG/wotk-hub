import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import { toast } from 'sonner'
import { Loader2, Trash2, Upload, ImageOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import type { CoupleDatePhoto } from '@/types/couple.types'

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
  const [photos, setPhotos] = useState<CoupleDatePhoto[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [preview, setPreview] = useState<CoupleDatePhoto | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { listPhotos, uploadPhoto, deletePhoto } = useCoupleApi()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setPhotos(await listPhotos(dateId))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las fotos')
    } finally {
      setLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dateId])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as CouplePage
    load()
  }, [load])

  async function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setUploading(true)
    try {
      const photo = await uploadPhoto(dateId, file)
      setPhotos((prev) => [photo, ...prev])
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo subir la foto')
    } finally {
      setUploading(false)
    }
  }

  async function handleDelete(photo: CoupleDatePhoto) {
    setDeletingId(photo.id)
    try {
      await deletePhoto(dateId, photo.id)
      setPhotos((prev) => prev.filter((p) => p.id !== photo.id))
      setPreview((prev) => (prev?.id === photo.id ? null : prev))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la foto')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>Fotos</Label>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={uploading}
          onClick={() => fileInputRef.current?.click()}
        >
          {uploading ? <Loader2 size={14} className="animate-spin" /> : <Upload className="h-4 w-4" />}
          {uploading ? 'Subiendo…' : 'Agregar foto'}
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          accept={ACCEPTED_TYPES}
          className="hidden"
          onChange={handleFileChange}
        />
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-6 text-muted-foreground">
          <Loader2 size={18} className="animate-spin" />
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
                disabled={deletingId === p.id}
                onClick={() => handleDelete(p)}
                className="absolute right-1 top-1 rounded-full bg-black/60 p-1 text-white opacity-0 transition-opacity group-hover:opacity-100 disabled:opacity-100"
              >
                {deletingId === p.id ? <Loader2 size={12} className="animate-spin" /> : <Trash2 size={12} />}
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
