import { useEffect, useState } from 'react'
import { useForm, type SubmitHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { useCategories } from '@/hooks/useCategories'
import { solesToCents, centsToSoles } from '@/lib/currency'
import type { Budget } from '@/types/finance.types'

const schema = z.object({
  category: z.string().min(1, 'Requerido'),
  limit: z.number().positive('Debe ser mayor a 0'),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editing?: Budget | null
  usedCategories: string[]
}

export default function BudgetForm({ open, onClose, onSaved, editing, usedCategories }: Props) {
  const [saving, setSaving] = useState(false)
  const { upsertBudget } = useFinanceApi()
  const { data: categoriesByKind, isLoading: categoriesLoading } = useCategories()

  const available = editing
    ? categoriesByKind.expense.filter((c) => c.name === editing.category)
    : categoriesByKind.expense.filter((c) => !usedCategories.includes(c.name))

  const {
    handleSubmit,
    setValue,
    watch,
    reset,
    register,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      category: editing?.category ?? available[0]?.name ?? '',
      limit: editing ? centsToSoles(editing.monthlyLimitCents) : 0,
    },
  })

  useEffect(() => {
    if (open) {
      reset({
        category: editing?.category ?? available[0]?.name ?? '',
        limit: editing ? centsToSoles(editing.monthlyLimitCents) : 0,
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing, reset])

  const category = watch('category')

  const onSubmit: SubmitHandler<FormValues> = async (values) => {
    setSaving(true)
    try {
      await upsertBudget(values.category, solesToCents(values.limit))
      toast.success('Presupuesto guardado')
      reset()
      onSaved()
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo guardar el presupuesto')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? 'Editar presupuesto' : 'Nuevo presupuesto'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1">
            <Label>Categoría</Label>
            <Select
              value={category}
              onValueChange={(v) => setValue('category', v)}
              disabled={!!editing || categoriesLoading}
            >
              <SelectTrigger>
                <SelectValue placeholder={categoriesLoading ? 'Cargando…' : undefined} />
              </SelectTrigger>
              <SelectContent>
                {available.map((c) => (
                  <SelectItem key={c.id} value={c.name}>
                    {c.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <Label>Límite mensual (S/)</Label>
            <Input type="number" step="0.01" min="0" {...register('limit', { valueAsNumber: true })} />
            {errors.limit && <p className="text-xs text-destructive">{errors.limit.message}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={saving || !category}>
              {saving && <Loader2 size={14} className="animate-spin" />}
              {saving ? 'Guardando…' : 'Guardar'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
