import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { ImageOff, Loader2 } from 'lucide-react'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import type { GalleryPhoto } from '@/types/couple.types'

// One row per date, in the same order the API returns them (occurredOn
// desc, matching couple_dates' own default order elsewhere in this
// module) — grouping is done here client-side from ListGalleryPhotos'
// flat list rather than server-side, same reasoning as CouplePage already
// grouping dates by status client-side.
interface DateGroup {
  dateId: number
  dateOccurredOn: string
  datePlace: string
  photos: GalleryPhoto[]
}

function groupByDate(photos: GalleryPhoto[]): DateGroup[] {
  const groups: DateGroup[] = []
  const byId = new Map<number, DateGroup>()
  for (const p of photos) {
    let g = byId.get(p.dateId)
    if (!g) {
      g = { dateId: p.dateId, dateOccurredOn: p.dateOccurredOn, datePlace: p.datePlace, photos: [] }
      byId.set(p.dateId, g)
      groups.push(g)
    }
    g.photos.push(p)
  }
  return groups
}

// Read-only cross-date photo gallery — no upload/delete here, those live on
// each date's own edit dialog (DatePhotos.tsx). This tab is purely for
// browsing everything at once, grouped by the cita it belongs to.
export default function GaleriaTab() {
  const [groups, setGroups] = useState<DateGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [preview, setPreview] = useState<GalleryPhoto | null>(null)
  const { listGallery } = useCoupleApi()

  useEffect(() => {
    async function load() {
      try {
        setGroups(groupByDate(await listGallery()))
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'No se pudo cargar la galería')
      } finally {
        setLoading(false)
      }
    }
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16 text-muted-foreground">
        <Loader2 size={20} className="animate-spin" />
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-[var(--radius)] border-2 border-dashed border-muted-foreground/25 py-16 text-center text-muted-foreground">
        <ImageOff className="h-8 w-8" />
        <p>Todavía no hay fotos en ninguna cita</p>
      </div>
    )
  }

  return (
    <>
      <div className="mx-auto w-full max-w-4xl space-y-6">
        {groups.map((g) => (
          <div key={g.dateId} className="space-y-2">
            <h2 className="text-sm font-semibold text-muted-foreground">
              {g.datePlace || '—'} · {g.dateOccurredOn}
            </h2>
            <div className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6">
              {g.photos.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  className="aspect-square overflow-hidden rounded-md border"
                  onClick={() => setPreview(p)}
                >
                  <img src={p.thumbnailUrl} alt="" className="h-full w-full object-cover" loading="lazy" />
                </button>
              ))}
            </div>
          </div>
        ))}
      </div>

      <Dialog open={preview != null} onOpenChange={(v) => !v && setPreview(null)}>
        <DialogContent className="sm:max-w-2xl">
          {preview && <img src={preview.url} alt="" className="max-h-[70vh] w-full rounded-md object-contain" />}
        </DialogContent>
      </Dialog>
    </>
  )
}
