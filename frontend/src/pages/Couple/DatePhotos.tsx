import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import { toast } from 'sonner'
import { Loader2, Trash2, Upload, ImageOff, MoreVertical } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
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
  const [uploadProgress, setUploadProgress] = useState<{ done: number; total: number } | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [preview, setPreview] = useState<CoupleDatePhoto | null>(null)
  const [editingPhoto, setEditingPhoto] = useState<CoupleDatePhoto | null>(null)
  const [imageUrlInput, setImageUrlInput] = useState('')
  const [savingUrl, setSavingUrl] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { listPhotos, uploadPhoto, deletePhoto, updatePhoto } = useCoupleApi()

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

  // Multiple files upload concurrently rather than one-at-a-time — each is
  // an independent POST, so a slow/failing file doesn't block the rest.
  // Progress is a simple done/total counter rather than per-file percentage;
  // good enough for the handful of photos someone picks from a gallery at
  // once, and much simpler than tracking individual upload progress events.
  async function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? [])
    e.target.value = ''
    if (files.length === 0) return

    setUploadProgress({ done: 0, total: files.length })
    const results = await Promise.allSettled(
      files.map((file) =>
        uploadPhoto(dateId, file).then((photo) => {
          setPhotos((prev) => [photo, ...prev])
          setUploadProgress((prev) => (prev ? { ...prev, done: prev.done + 1 } : prev))
          return photo
        })
      )
    )
    setUploadProgress(null)

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

  function openImageUrlDialog(photo: CoupleDatePhoto) {
    // Show externalUrl if set, otherwise the current thumbnailUrl as the default.
    setImageUrlInput(photo.externalUrl ?? photo.thumbnailUrl)
    setEditingPhoto(photo)
  }

  function closeImageUrlDialog() {
    setEditingPhoto(null)
    setImageUrlInput('')
  }

  async function handleSaveImageUrl() {
    if (!editingPhoto) return
    setSavingUrl(true)
    try {
      const updated = await updatePhoto(dateId, editingPhoto.id, imageUrlInput.trim())
      setPhotos((prev) => prev.map((p) => (p.id === updated.id ? updated : p)))
      setPreview((prev) => (prev?.id === updated.id ? updated : prev))
      toast.success('Imagen actualizada')
      closeImageUrlDialog()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo actualizar la imagen')
    } finally {
      setSavingUrl(false)
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
          disabled={uploadProgress != null}
          onClick={() => fileInputRef.current?.click()}
        >
          {uploadProgress != null ? <Loader2 size={14} className="animate-spin" /> : <Upload className="h-4 w-4" />}
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
                <img
                  src={p.externalUrl ?? p.thumbnailUrl}
                  alt=""
                  className="h-full w-full object-cover"
                  loading="lazy"
                />
              </button>
              <div className="absolute right-1 top-1 flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 rounded-full bg-black/60 p-0 text-white hover:bg-black/70 hover:text-white"
                      aria-label="Más opciones"
                    >
                      <MoreVertical size={14} />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => openImageUrlDialog(p)}>
                      Cambiar imagen
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => handleDelete(p)}
                      className="text-destructive focus:text-destructive"
                      disabled={deletingId === p.id}
                    >
                      <Trash2 className="h-4 w-4" />
                      Eliminar
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Full-size preview dialog */}
      <Dialog open={preview != null} onOpenChange={(v) => !v && setPreview(null)}>
        <DialogContent className="sm:max-w-2xl">
          {preview && (
            <img src={preview.url} alt="" className="max-h-[70vh] w-full rounded-md object-contain" />
          )}
        </DialogContent>
      </Dialog>

      {/* Change image URL dialog */}
      <Dialog open={editingPhoto != null} onOpenChange={(v) => !v && closeImageUrlDialog()}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Cambiar imagen</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="image-url">URL de imagen</Label>
            <Input
              id="image-url"
              type="url"
              placeholder="https://..."
              value={imageUrlInput}
              onChange={(e) => setImageUrlInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  handleSaveImageUrl()
                }
              }}
            />
            <p className="text-xs text-muted-foreground">
              Ingresá una URL directa a una imagen (jpg, png, webp, gif). La imagen se muestra
              tal cual es — asegurate de que sea accesible públicamente.
            </p>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={closeImageUrlDialog} disabled={savingUrl}>
              Cancelar
            </Button>
            <Button onClick={handleSaveImageUrl} disabled={savingUrl}>
              {savingUrl && <Loader2 size={14} className="animate-spin" />}
              {savingUrl ? 'Guardando…' : 'Guardar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
