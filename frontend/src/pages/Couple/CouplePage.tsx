import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import {
  Pencil,
  Trash2,
  Star,
  MoreVertical,
  Heart,
  Link as LinkIcon,
  Plus,
  Check,
  UtensilsCrossed,
  Sandwich,
  Film,
  Plane,
  TreePine,
  Home,
  PartyPopper,
  Sparkles,
  type LucideIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import { DATE_CATEGORY_LABELS, type CoupleDate } from '@/types/couple.types'
import CoupleCover from './CoupleCover'
import DateForm from './DateForm'
import FloatingActionButton from './FloatingActionButton'

const UNDO_WINDOW_MS = 4500

// Small warm-toned family per category, all derived from tokens already
// established elsewhere in the app (brand terracotta, sage --success, and
// the cover's sakura/ink tones) rather than a new arbitrary palette — ties
// the card grid back to the cover's ink-wash mood.
const CATEGORY_ICONS: Record<string, LucideIcon> = {
  cena: UtensilsCrossed,
  almuerzo: Sandwich,
  cine: Film,
  viaje: Plane,
  aire_libre: TreePine,
  casa: Home,
  evento: PartyPopper,
  otro: Sparkles,
}

const CATEGORY_ACCENTS: Record<string, string> = {
  cena: '--primary',
  almuerzo: '--primary',
  cine: '--cover-sakura-dark',
  evento: '--cover-sakura-dark',
  aire_libre: '--success',
  casa: '--success',
  viaje: '--cover-ink-mid',
  otro: '--cover-ink-mid',
}

function CategoryChip({ category }: { category: string }) {
  const Icon = CATEGORY_ICONS[category] ?? Sparkles
  const accent = CATEGORY_ACCENTS[category] ?? '--cover-ink-mid'
  return (
    <span
      aria-hidden="true"
      className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
      style={{
        backgroundColor: `color-mix(in oklch, var(${accent}) 16%, var(--card))`,
        color: `var(${accent})`,
      }}
    >
      <Icon size={15} strokeWidth={2.25} />
    </span>
  )
}

// Small abstract accent ring on each date-entry card, tastefully echoing
// the cover's ink-wash mood (a "hanko stamp" gesture — deliberately
// abstract, not a literal red ink stamp graphic).
function HankoAccent() {
  return (
    <span
      aria-hidden="true"
      className="pointer-events-none absolute right-3 top-3 h-2.5 w-2.5 rounded-full border"
      style={{ borderColor: 'color-mix(in oklch, var(--cover-ink-near) 40%, transparent)' }}
    />
  )
}

interface DateCardProps {
  date: CoupleDate
  delay: number
  onEdit: () => void
  onDelete: () => void
  onMarkDone?: () => void
}

function DateCard({ date: d, delay, onEdit, onDelete, onMarkDone }: DateCardProps) {
  return (
    <CozyCard className="relative animate-card-in" style={{ animationDelay: `${delay}ms` }}>
      <HankoAccent />
      <CardContent className="space-y-2.5 p-5">
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-start gap-3">
            <CategoryChip category={d.category} />
            <div className="min-w-0">
              <div className="text-sm font-semibold">{d.place || '—'}</div>
              <div className="text-xs text-muted-foreground">{d.occurredOn}</div>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {onMarkDone && (
              <Button variant="ghost" size="icon" aria-label="Marcar como realizada" onClick={onMarkDone}>
                <Check className="h-4 w-4" />
              </Button>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="Más acciones">
                  <MoreVertical className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={onEdit}>
                  <Pencil className="h-4 w-4" />
                  Editar
                </DropdownMenuItem>
                <DropdownMenuItem onClick={onDelete} className="text-destructive focus:text-destructive">
                  <Trash2 className="h-4 w-4" />
                  Eliminar
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 pl-11">
          <Badge variant="secondary">{DATE_CATEGORY_LABELS[d.category] ?? d.category}</Badge>
          {d.rating != null && (
            <div className="flex items-center gap-0.5">
              {[1, 2, 3, 4, 5].map((n) => (
                <Star
                  key={n}
                  size={12}
                  className={cn(n <= (d.rating ?? 0) ? 'fill-yellow-400 text-yellow-400' : 'text-muted-foreground')}
                />
              ))}
            </div>
          )}
          {d.costCents != null && (
            <span className="ml-auto text-xs font-medium text-muted-foreground">{formatPEN(d.costCents)}</span>
          )}
        </div>

        {d.notes && <p className="pl-11 text-sm text-muted-foreground">{d.notes}</p>}

        {d.tiktokUrl && (
          <a
            href={d.tiktokUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 pl-11 text-xs text-muted-foreground hover:text-foreground hover:underline"
          >
            <LinkIcon className="h-3 w-3" />
            Ver en TikTok
          </a>
        )}
      </CardContent>
    </CozyCard>
  )
}

export default function CouplePage() {
  const [dates, setDates] = useState<CoupleDate[]>([])
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<CoupleDate | null>(null)
  const { listDates, updateDate, deleteDate } = useCoupleApi()
  const pendingDeletes = useRef(new Map<number, number>())

  const load = useCallback(async () => {
    try {
      setDates(await listDates())
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las citas')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as Finances/DbManager pages
    load()
  }, [load])

  async function commitDelete(id: number) {
    pendingDeletes.current.delete(id)
    try {
      await deleteDate(id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la cita')
      load()
    }
  }

  function handleDelete(d: CoupleDate) {
    let removedIndex = -1
    setDates((prev) => {
      removedIndex = prev.findIndex((x) => x.id === d.id)
      return prev.filter((x) => x.id !== d.id)
    })

    const timer = window.setTimeout(() => commitDelete(d.id), UNDO_WINDOW_MS)
    pendingDeletes.current.set(d.id, timer)

    toast.success('Cita eliminada', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(d.id)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(d.id)
          }
          setDates((prev) => {
            const next = [...prev]
            next.splice(Math.min(removedIndex, next.length), 0, d)
            return next
          })
        },
      },
    })
  }

  async function handleMarkDone(d: CoupleDate) {
    setDates((prev) => prev.map((x) => (x.id === d.id ? { ...x, status: 'done' } : x)))
    try {
      await updateDate(d.id, {
        occurredOn: d.occurredOn,
        place: d.place,
        category: d.category,
        notes: d.notes,
        costCents: d.costCents ?? null,
        rating: d.rating ?? null,
        tiktokUrl: d.tiktokUrl,
        status: 'done',
      })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo actualizar la cita')
      setDates((prev) => prev.map((x) => (x.id === d.id ? d : x)))
    }
  }

  const doneDates = dates.filter((d) => d.status === 'done')
  const plannedDates = dates
    .filter((d) => d.status === 'planned')
    .sort((a, b) => a.occurredOn.localeCompare(b.occurredOn))

  const rated = doneDates.filter((d) => d.rating != null)
  const avgRating = rated.length
    ? rated.reduce((sum, d) => sum + (d.rating ?? 0), 0) / rated.length
    : null
  const totalSpentCents = doneDates.reduce((sum, d) => sum + (d.costCents ?? 0), 0)

  return (
    <div className="space-y-6 pb-24 sm:pb-0">
      <CoupleCover
        onNewDate={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

      {/* Constrained to a reading-column width (Day One's centered journal
          feel) instead of stretching full viewport width — with only 1-3
          entries, a full-bleed grid left a large dead void below the cards
          that read as cold/unfinished. The cover above stays full width,
          untouched. */}
      <div className="mx-auto w-full max-w-4xl space-y-6">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          <CozyCard className="animate-card-in">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Citas registradas</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{dates.length}</div>
            </CardContent>
          </CozyCard>
          <CozyCard className="animate-card-in [animation-delay:60ms]">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Calificación promedio</CardTitle>
            </CardHeader>
            <CardContent>
              {avgRating != null ? (
                <div className="text-2xl font-bold">{avgRating.toFixed(1)}</div>
              ) : (
                <Star className="h-6 w-6 text-muted-foreground/35" strokeWidth={1.75} />
              )}
            </CardContent>
          </CozyCard>
          <CozyCard className="col-span-2 animate-card-in [animation-delay:120ms] sm:col-span-1">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Total invertido</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{formatPEN(totalSpentCents)}</div>
            </CardContent>
          </CozyCard>
        </div>

        {dates.length === 0 ? (
          <CozyCard className="animate-card-in">
            <CardContent className="flex flex-col items-center gap-2 py-12 text-center text-muted-foreground">
              <Heart className="h-8 w-8 animate-pulse" />
              <p>Todavía no registraste ninguna cita</p>
            </CardContent>
          </CozyCard>
        ) : (
          <>
            {plannedDates.length > 0 && (
              <div className="space-y-3">
                <h2 className="text-sm font-semibold text-muted-foreground">Planeadas</h2>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {plannedDates.map((d, i) => (
                    <DateCard
                      key={d.id}
                      date={d}
                      delay={Math.min(i * 40, 320)}
                      onEdit={() => {
                        setEditing(d)
                        setFormOpen(true)
                      }}
                      onDelete={() => handleDelete(d)}
                      onMarkDone={() => handleMarkDone(d)}
                    />
                  ))}
                </div>
              </div>
            )}

            <div className="space-y-3">
              {plannedDates.length > 0 && <h2 className="text-sm font-semibold text-muted-foreground">Realizadas</h2>}
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {doneDates.map((d, i) => (
                  <DateCard
                    key={d.id}
                    date={d}
                    delay={Math.min(i * 40, 320)}
                    onEdit={() => {
                      setEditing(d)
                      setFormOpen(true)
                    }}
                    onDelete={() => handleDelete(d)}
                  />
                ))}

                {dates.length < 4 && (
                  <button
                    type="button"
                    onClick={() => {
                      setEditing(null)
                      setFormOpen(true)
                    }}
                    className="animate-card-in flex min-h-[160px] flex-col items-center justify-center gap-2 rounded-[var(--radius)] border-2 border-dashed border-muted-foreground/25 p-5 text-muted-foreground/70 transition-colors hover:border-primary/40 hover:text-primary"
                    style={{ animationDelay: `${Math.min(doneDates.length * 40, 320)}ms` }}
                  >
                    <Plus className="h-5 w-5" />
                    <span className="text-sm font-medium">Agregar otra cita</span>
                  </button>
                )}
              </div>
            </div>
          </>
        )}
      </div>

      <FloatingActionButton
        label="Nueva cita"
        onClick={() => {
          setEditing(null)
          setFormOpen(true)
        }}
      />

      <DateForm open={formOpen} onClose={() => setFormOpen(false)} onSaved={load} editing={editing} />
    </div>
  )
}
