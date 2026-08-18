import type { ReactNode } from 'react'
import { paperSurfaceStyle } from '@/components/ui/cozy-card'

interface Props {
  label?: ReactNode
  children: ReactNode
}

/**
 * Shared "cozy paper" surface for Recharts tooltips — same treatment as
 * CozyCard, extracted so every chart's tooltip chrome stays part of the
 * cozy system instead of Recharts' stock bordered box. Each chart still
 * owns its own row formatting (series colors, value formatting); this only
 * wraps the box itself.
 */
export function ChartTooltip({ label, children }: Props) {
  return (
    <div
      className="rounded-[var(--radius)] px-3 py-2 text-sm shadow-[0_1px_2px_oklch(0.35_0.03_40/0.07),0_12px_28px_-10px_oklch(0.35_0.06_40/0.18)]"
      style={paperSurfaceStyle}
    >
      {label && <p className="mb-1 font-medium text-foreground">{label}</p>}
      {children}
    </div>
  )
}
