import { useEffect, useRef, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2, Tag, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { EmptyState } from '@/components/ui/empty-state'
import {
  useCategories,
  useCreateCategory,
  useUpdateCategory,
  useDeleteCategory,
} from '@/hooks/useCategories'
import type { Category } from '@/types/finance.types'

const UNDO_WINDOW_MS = 4500

// Normalizes free typing into the lowercase-hyphenated slug the backend
// expects for `name` — spaces (and any run of whitespace) become a single
// hyphen, everything else non [a-z0-9-] is stripped.
function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-+/g, '-')
}

const schema = z.object({
  name: z
    .string()
    .min(1, 'Requerido')
    .regex(/^[a-z0-9-]+$/, 'Solo minúsculas, números y guiones'),
  label: z.string().min(1, 'Requerido'),
})

type FormValues = z.infer<typeof schema>

interface CategoryFormProps {
  open: boolean
  onClose: () => void
  onSaved: () => void
  kind: 'expense' | 'income'
  editing?: Category | null
}

function CategoryForm({ open, onClose, onSaved, kind, editing }: CategoryFormProps) {
  const [saving, setSaving] = useState(false)
  const createCategory = useCreateCategory()
  const updateCategory = useUpdateCategory()

  const {
    register,
    handleSubmit,
    setValue,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: editing?.name ?? '', label: editing?.label ?? '' },
  })

  useEffect(() => {
    if (open) reset({ name: editing?.name ?? '', label: editing?.label ?? '' })
  }, [open, editing, reset])

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSaving(true)
    try {
      if (editing) {
        await updateCategory(editing.id, { name: values.name, kind, label: values.label })
        toast.success('Categoría actualizada')
      } else {
        await createCategory({ name: values.name, kind, label: values.label })
        toast.success('Categoría creada')
      }
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar la categoría')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? 'Editar categoría' : 'Nueva categoría'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Nombre (slug)</Label>
            <Input
              {...register('name', {
                onChange: (e) => setValue('name', slugify(e.target.value)),
              })}
              placeholder="Ej: comida-rapida"
            />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>
          <div className="space-y-1">
            <Label>Etiqueta</Label>
            <Input {...register('label')} placeholder="Ej: Comida rápida" />
            {errors.label && <p className="text-xs text-destructive">{errors.label.message}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving}>
              {saving && <Loader2 size={14} className="animate-spin" />}
              {saving ? 'Guardando…' : 'Guardar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface CategorySectionProps {
  title: string
  emptyLabel: string
  categories: Category[]
  hiddenIds: Set<number>
  onAdd: () => void
  onEdit: (c: Category) => void
  onDelete: (c: Category) => void
}

function CategorySection({
  title,
  emptyLabel,
  categories,
  hiddenIds,
  onAdd,
  onEdit,
  onDelete,
}: CategorySectionProps) {
  const visible = categories.filter((c) => !hiddenIds.has(c.id))

  return (
    <CozyCard className="animate-card-in">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-base font-semibold">{title}</CardTitle>
        <Button variant="ghost" size="icon" aria-label={`Agregar categoría de ${title.toLowerCase()}`} onClick={onAdd}>
          <Plus size={16} />
        </Button>
      </CardHeader>
      <CardContent>
        {visible.length === 0 ? (
          <EmptyState
            icon={<Tag className="h-8 w-8" />}
            title={emptyLabel}
            description="Agrega una para clasificar tus movimientos."
            action={{ label: 'Crear categoría', onClick: onAdd }}
          />
        ) : (
          <div className="space-y-1">
            {visible.map((c) => (
              <div
                key={c.id}
                className="flex items-center justify-between rounded-md px-2 py-1.5 hover:bg-muted/50"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{c.label}</p>
                  <p className="truncate text-xs text-muted-foreground">{c.name}</p>
                </div>
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Editar categoría ${c.label}`}
                    onClick={() => onEdit(c)}
                  >
                    <Pencil size={14} />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Eliminar categoría ${c.label}`}
                    onClick={() => onDelete(c)}
                  >
                    <Trash2 size={14} />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </CozyCard>
  )
}

export default function CategoriesPage() {
  const { data, isLoading, refetch } = useCategories()
  const deleteCategory = useDeleteCategory()

  const [formOpen, setFormOpen] = useState(false)
  const [formKind, setFormKind] = useState<'expense' | 'income'>('expense')
  const [editing, setEditing] = useState<Category | null>(null)

  // Deletes are optimistic-hide + undo toast, same pattern as PresupuestosTab /
  // TarjetasTab / SuscripcionesTab: the category disappears immediately and
  // the actual DELETE only fires once the undo window elapses.
  const [hiddenIds, setHiddenIds] = useState<Set<number>>(new Set())
  const pendingDeletes = useRef(new Map<number, number>())

  function openAdd(kind: 'expense' | 'income') {
    setFormKind(kind)
    setEditing(null)
    setFormOpen(true)
  }

  function openEdit(c: Category) {
    setFormKind(c.kind)
    setEditing(c)
    setFormOpen(true)
  }

  function unhide(id: number) {
    setHiddenIds((prev) => {
      const next = new Set(prev)
      next.delete(id)
      return next
    })
  }

  async function commitDelete(c: Category) {
    pendingDeletes.current.delete(c.id)
    try {
      await deleteCategory(c.id)
      await refetch()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar la categoría')
      unhide(c.id)
    }
  }

  function handleDelete(c: Category) {
    setHiddenIds((prev) => new Set(prev).add(c.id))

    const timer = window.setTimeout(() => commitDelete(c), UNDO_WINDOW_MS)
    pendingDeletes.current.set(c.id, timer)

    toast.success('Categoría eliminada', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(c.id)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(c.id)
          }
          unhide(c.id)
        },
      },
    })
  }

  return (
    <div className="space-y-6 pb-24 sm:pb-0">
      <h1 className="text-2xl font-bold">Categorías</h1>

      {isLoading ? (
        <p className="text-sm text-muted-foreground">Cargando…</p>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          <CategorySection
            title="Gastos"
            emptyLabel="Sin categorías de gastos"
            categories={data.expense}
            hiddenIds={hiddenIds}
            onAdd={() => openAdd('expense')}
            onEdit={openEdit}
            onDelete={handleDelete}
          />
          <CategorySection
            title="Ingresos"
            emptyLabel="Sin categorías de ingresos"
            categories={data.income}
            hiddenIds={hiddenIds}
            onAdd={() => openAdd('income')}
            onEdit={openEdit}
            onDelete={handleDelete}
          />
        </div>
      )}

      <CategoryForm
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={refetch}
        kind={formKind}
        editing={editing}
      />
    </div>
  )
}
