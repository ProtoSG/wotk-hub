import { Check, Flame, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { formatKg, parseKgInput } from '@/lib/weight'
import NumericField from './NumericField'
import { withWorkingNumbers, type SetRow } from './setRows'

interface SetGridProps {
  rows: SetRow[]
  onChange: (rows: SetRow[]) => void
  disabled?: boolean
}

/** Shared column template so the header and the rows can't drift apart. */
const COLUMNS = 'grid grid-cols-[2rem_1fr_1fr_2.75rem_2.25rem] items-center gap-2'

/**
 * The logging surface: one row per set, edited locally and pushed up as a
 * whole list (the API replaces sets in bulk). Set numbers come from row order,
 * so removing a middle row never needs renumbering.
 */
export default function SetGrid({ rows, onChange, disabled }: SetGridProps) {
  const update = (key: string, patch: Partial<SetRow>) => {
    onChange(rows.map((row) => (row.key === key ? { ...row, ...patch } : row)))
  }

  const numbered = withWorkingNumbers(rows)

  return (
    <div className="space-y-1">
      <div className={cn(COLUMNS, 'px-1 text-xs font-medium text-muted-foreground')}>
        <span className="text-center">#</span>
        <span>Kg</span>
        <span>Reps</span>
        <span className="sr-only">Completada</span>
        <span className="sr-only">Quitar</span>
      </div>

      {numbered.map(({ row, workingNumber }) => {
        const label = row.isWarmup ? 'calentamiento' : `serie ${workingNumber}`

        return (
          <div
            key={row.key}
            className={cn(
              COLUMNS,
              'rounded-md px-1 py-1 transition-colors duration-200',
              row.completed && 'bg-success/10',
            )}
          >
            <button
              type="button"
              disabled={disabled}
              onClick={() => update(row.key, { isWarmup: !row.isWarmup })}
              aria-label={row.isWarmup ? 'Convertir en serie normal' : 'Convertir en calentamiento'}
              aria-pressed={row.isWarmup}
              className={cn(
                'flex h-9 w-8 items-center justify-center rounded-md text-sm font-medium tabular-nums transition-colors',
                row.isWarmup ? 'text-primary' : 'text-muted-foreground',
                !disabled && 'hover:bg-muted',
              )}
            >
              {row.isWarmup ? <Flame className="h-4 w-4" /> : workingNumber}
            </button>

            <NumericField
              value={row.weightGrams === 0 ? '' : formatKg(row.weightGrams)}
              parse={parseKgInput}
              onCommit={(weightGrams) => update(row.key, { weightGrams })}
              label={`Peso de la ${label}`}
              decimal
              disabled={disabled}
            />

            <NumericField
              value={row.reps === 0 ? '' : String(row.reps)}
              parse={parseReps}
              onCommit={(reps) => update(row.key, { reps })}
              label={`Repeticiones de la ${label}`}
              disabled={disabled}
            />

            <button
              type="button"
              disabled={disabled}
              onClick={() => update(row.key, { completed: !row.completed })}
              aria-label={`Marcar la ${label} como ${row.completed ? 'pendiente' : 'completada'}`}
              aria-pressed={row.completed}
              className={cn(
                'flex h-11 w-11 items-center justify-center rounded-md border transition-colors duration-200',
                row.completed
                  ? 'border-success/40 bg-success/15 text-success'
                  : 'border-input text-muted-foreground',
                !disabled && !row.completed && 'hover:bg-muted',
              )}
            >
              <Check className="h-5 w-5" />
            </button>

            <Button
              type="button"
              variant="ghost"
              size="icon"
              disabled={disabled}
              onClick={() => onChange(rows.filter((r) => r.key !== row.key))}
              aria-label={`Quitar la ${label}`}
              className="h-9 w-9 text-muted-foreground"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        )
      })}
    </div>
  )
}

function parseReps(raw: string): number | null {
  const trimmed = raw.trim()
  if (trimmed === '') return 0
  const reps = Number(trimmed)
  if (!Number.isFinite(reps) || reps < 0) return null
  return Math.round(reps)
}
