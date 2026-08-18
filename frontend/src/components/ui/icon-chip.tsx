import type { ComponentType } from 'react'

interface IconChipProps {
  icon: ComponentType<{ className?: string }>
  // A design-token name (e.g. '--primary') or a raw CSS color (e.g. a hex
  // string from a user-picked palette like Finances' GOAL_COLORS). Token
  // names are resolved through var(); anything else is used as-is.
  accent: string
  size?: 'xs' | 'sm' | 'md'
  className?: string
}

function resolveAccent(accent: string) {
  return accent.startsWith('--') ? `var(${accent})` : accent
}

// Sizes match the identity-icon badges already established in the app: 'md'
// is CategoryChip's original size (Couple date category badges), 'sm' is the
// card/section-header badge size used e.g. on the YtDlp page, 'xs' fits
// inside a compact table header row (h-10) without pushing it taller.
const CONTAINER_SIZE: Record<'xs' | 'sm' | 'md', string> = {
  xs: 'h-6 w-6',
  md: 'h-8 w-8',
  sm: 'h-9 w-9',
}

const ICON_SIZE: Record<'xs' | 'sm' | 'md', string> = {
  xs: 'h-3.5 w-3.5',
  md: 'h-[15px] w-[15px]',
  sm: 'h-[18px] w-[18px]',
}

export function IconChip({ icon: Icon, accent, size = 'md', className }: IconChipProps) {
  return (
    <span
      aria-hidden="true"
      className={`inline-flex shrink-0 items-center justify-center rounded-full ${CONTAINER_SIZE[size]} ${className ?? ''}`}
      style={{
        backgroundColor: `color-mix(in oklch, ${resolveAccent(accent)} 16%, var(--card))`,
        color: resolveAccent(accent),
      }}
    >
      <Icon className={ICON_SIZE[size]} />
    </span>
  )
}
