import { Plus } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FloatingActionButtonProps {
  onClick: () => void
  label: string
  className?: string
}

/**
 * Shared mobile-only FAB (fixed, bottom-right, sm:hidden). Consolidates what
 * used to be three near-identical implementations across Finances and
 * Couple. `bottom` positioning defaults to the safe-area inset but can be
 * overridden per-caller via `className` (merged with `cn()`), e.g. callers
 * that need extra clearance above a bottom nav.
 */
export function FloatingActionButton({ onClick, label, className }: FloatingActionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className={cn(
        'fixed right-4 bottom-[env(safe-area-inset-bottom)] z-40 flex h-14 w-14 items-center justify-center rounded-full bg-foreground text-background shadow-lg transition-transform duration-200 ease-out hover:scale-105 active:scale-95 sm:hidden',
        className
      )}
    >
      <Plus className="h-6 w-6" />
    </button>
  )
}
