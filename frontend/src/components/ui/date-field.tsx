import * as React from 'react'
import { cn } from '@/lib/utils'

export interface DateFieldProps
  extends Omit<React.ComponentProps<'input'>, 'type' | 'value' | 'onChange'> {
  value: string
  onChange: (value: string) => void
}

function formatDisplay(iso: string): string {
  if (!iso) return ''
  const date = new Date(`${iso}T00:00:00`)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleDateString('es-PE', { day: 'numeric', month: 'short', year: 'numeric' })
}

/**
 * Drop-in replacement for `<Input type="date">`.
 *
 * iOS Safari never lets CSS remove the native chrome (border/shadow/
 * radius) on input[type=date] — `appearance: none` only gets you
 * height/width parity, the control keeps its own paint regardless of
 * author CSS (confirmed WebKit limitation, no CSS-only fix exists).
 * So instead of chasing that further, we render our own box that matches
 * the sibling Input pixel-for-pixel, and stack the real
 * `<input type="date">` on top of it, fully transparent — it still gets
 * the tap/click and opens the native picker, we just never show its
 * native paint.
 *
 * Controlled by design (`value`/`onChange`, not `register()`): RHF's
 * `reset()` writes straight to the DOM input without firing a change
 * event, which an uncontrolled version would miss on edit-mode forms.
 * Wire it the same way the Select fields in these forms already are —
 * `value={watch('field')}` / `onChange={(v) => setValue('field', v)}`.
 */
const DateField = React.forwardRef<HTMLInputElement, DateFieldProps>(
  ({ className, value, onChange, ...props }, ref) => (
    <div
      className={cn(
        'relative flex h-9 w-full items-center rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors',
        className
      )}
    >
      <span className={cn('pointer-events-none select-none', !value && 'text-muted-foreground')}>
        {formatDisplay(value) || 'dd/mm/aaaa'}
      </span>
      <input
        type="date"
        ref={ref}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
        {...props}
      />
    </div>
  )
)
DateField.displayName = 'DateField'

export { DateField }
