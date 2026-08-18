import { Heart, Loader2 } from 'lucide-react'
import { useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import { solesToCents, centsToSoles } from '@/lib/currency'
import { cn } from '@/lib/utils'
import {
  DATE_CATEGORIES,
  DATE_CATEGORY_LABELS,
  type CoupleDate,
  type CoupleDateInput,
  type DateStatus,
} from '@/types/couple.types'
import { datesKey } from './coupleKeys'
import DatePhotos from './DatePhotos'

const schema = z.object({
  occurredOn: z.string().min(1, 'Requerido'),
  place: z.string(),
  category: z.string().min(1, 'Requerido'),
  notes: z.string(),
  cost: z.number().min(0, 'No puede ser negativo').nullable(),
  rating: z.number().min(1).max(5).nullable(),
  tiktokUrl: z
    .string()
    .refine(
      (v) => v === '' || /^https:\/\/([a-z0-9-]+\.)*tiktok\.com\//i.test(v),
      'Debe ser un link de TikTok (https://www.tiktok.com/...)'
    ),
  status: z.enum(['planned', 'done']),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onClose: () => void
  editing?: CoupleDate | null
}

function defaults(editing?: CoupleDate | null): FormValues {
  return editing
    ? {
        occurredOn: editing.occurredOn,
        place: editing.place,
        category: editing.category,
        notes: editing.notes,
        cost: editing.costCents != null ? centsToSoles(editing.costCents) : null,
        rating: editing.rating ?? null,
        tiktokUrl: editing.tiktokUrl,
        status: editing.status,
      }
    : {
        occurredOn: new Date().toISOString().slice(0, 10),
        place: '',
        category: 'cena',
        notes: '',
        cost: null,
        rating: null,
        tiktokUrl: '',
        status: 'done',
      }
}

export default function DateForm({ open, onClose, editing }: Props) {
  const { createDate, updateDate } = useCoupleApi()
  const queryClient = useQueryClient()

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaults(editing),
  })

  const mutation = useMutation({
    mutationFn: (input: CoupleDateInput) => (editing ? updateDate(editing.id, input) : createDate(input)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: datesKey() })
      toast.success(editing ? 'Cita actualizada' : 'Cita registrada')
      reset()
      onClose()
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar la cita')
    },
  })

  useEffect(() => {
    if (open) reset(defaults(editing))
  }, [open, editing, reset])

  const category = watch('category')
  const status = watch('status')
  const rating = watch('rating')

  // New entries default to 'done', but picking a future date almost always
  // means it's being planned ahead of time rather than logged after the fact.
  function handleOccurredOnChange(value: string) {
    setValue('occurredOn', value)
    if (!editing && value > new Date().toISOString().slice(0, 10)) {
      setValue('status', 'planned')
    }
  }

  const onSubmit: SubmitHandler<FormValues> = (values) => {
    const input: CoupleDateInput = {
      occurredOn: values.occurredOn,
      place: values.place,
      category: values.category,
      notes: values.notes,
      costCents: values.cost != null ? solesToCents(values.cost) : null,
      rating: values.rating,
      tiktokUrl: values.tiktokUrl,
      status: values.status,
    }
    mutation.mutate(input)
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md" onOpenAutoFocus={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>{editing ? 'Editar cita' : 'Nueva cita'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Fecha</Label>
              <Input
                type="date"
                {...register('occurredOn', { onChange: (e) => handleOccurredOnChange(e.target.value) })}
              />
              {errors.occurredOn && <p className="text-xs text-destructive">{errors.occurredOn.message}</p>}
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Categoría</Label>
              <Select value={category} onValueChange={(v) => setValue('category', v)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DATE_CATEGORIES.map((c) => (
                    <SelectItem key={c} value={c}>
                      {DATE_CATEGORY_LABELS[c] ?? c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Lugar</Label>
              <Input placeholder="Restaurante, cine, parque..." {...register('place')} />
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Estado</Label>
              <Select value={status} onValueChange={(v) => setValue('status', v as DateStatus)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="planned">Planeada</SelectItem>
                  <SelectItem value="done">Realizada</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="min-w-0 space-y-1">
              <Label>Costo (S/, opcional)</Label>
              <Input
                type="number"
                step="0.01"
                min="0"
                {...register('cost', {
                  setValueAs: (v) => (v === '' ? null : Number(v)),
                })}
              />
              {errors.cost && <p className="text-xs text-destructive">{errors.cost.message}</p>}
            </div>
            <div className="min-w-0 space-y-1">
              <Label>Calificación</Label>
              <div className="flex h-9 items-center gap-1">
                {[1, 2, 3, 4, 5].map((n) => (
                  <button
                    key={n}
                    type="button"
                    aria-label={`${n} corazón${n > 1 ? 'es' : ''}`}
                    onClick={() => setValue('rating', rating === n ? null : n, { shouldDirty: true })}
                    className="p-2 transition-transform duration-150 ease-out hover:scale-110 active:scale-95"
                  >
                    <Heart
                      className={cn(
                        'h-[18px] w-[18px] transition-colors duration-150',
                        rating != null && n <= rating
                          ? 'fill-primary text-primary'
                          : 'text-muted-foreground'
                      )}
                    />
                  </button>
                ))}
              </div>
            </div>
          </div>
          <div className="space-y-1">
            <Label>Link de TikTok (opcional)</Label>
            <Input
              type="url"
              placeholder="https://www.tiktok.com/@usuario/video/..."
              {...register('tiktokUrl')}
            />
            {errors.tiktokUrl && <p className="text-xs text-destructive">{errors.tiktokUrl.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Notas</Label>
            <Textarea placeholder="¿Cómo la pasaron?" {...register('notes')} />
          </div>
          {/* Photos only make sense once the date exists (needs an id to
              attach uploads to) — a brand-new, unsaved date has nothing to
              attach photos to yet. */}
          {editing && <DatePhotos dateId={editing.id} />}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
              {mutation.isPending ? 'Guardando…' : 'Guardar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
