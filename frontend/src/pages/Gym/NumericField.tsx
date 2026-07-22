import { useState } from 'react'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

interface NumericFieldProps {
  /** Formatted value owned by the parent. */
  value: string
  /** Parses the typed text; return null to reject and revert on blur. */
  parse: (raw: string) => number | null
  onCommit: (parsed: number) => void
  label: string
  decimal?: boolean
  disabled?: boolean
  className?: string
}

/**
 * A number input that keeps the raw text while focused — so a half-typed
 * "82," or a cleared field isn't fought by reformatting mid-keystroke — and
 * commits on blur. Values changed elsewhere (a save round-trip) sync in only
 * while the field is idle, so the server can never overwrite what is being
 * typed.
 */
export default function NumericField({
  value,
  parse,
  onCommit,
  label,
  decimal,
  disabled,
  className,
}: NumericFieldProps) {
  const [draft, setDraft] = useState(value)
  const [focused, setFocused] = useState(false)
  const [lastValue, setLastValue] = useState(value)

  // Adjusting state during render (React's documented pattern for "a prop
  // changed and derived state must follow"), not in an effect: this way the
  // field never renders one frame with a stale draft.
  if (value !== lastValue) {
    setLastValue(value)
    if (!focused) setDraft(value)
  }

  return (
    <Input
      type="text"
      inputMode={decimal ? 'decimal' : 'numeric'}
      disabled={disabled}
      value={draft}
      aria-label={label}
      placeholder="0"
      onFocus={() => setFocused(true)}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => {
        setFocused(false)
        const parsed = parse(draft)
        if (parsed === null) {
          setDraft(value)
          return
        }
        onCommit(parsed)
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter') e.currentTarget.blur()
      }}
      className={cn('h-11 text-center tabular-nums', className)}
    />
  )
}
