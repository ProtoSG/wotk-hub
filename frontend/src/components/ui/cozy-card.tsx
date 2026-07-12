import * as React from 'react'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/utils'

/**
 * Cozy/"confortante" card surface — warm paper-tinted background with a
 * subtle diagonal fiber grain (pure CSS, no image assets) and a warm-tinted
 * diffused shadow, replacing the default flat border + gray shadow. Built
 * entirely from tokens already in index.css (--primary, --card, --radius);
 * no new global tokens introduced here.
 *
 * Originally a bespoke, page-local treatment on the Citas page (see
 * .impeccable.md and CouplePage.tsx history), promoted here so it can be
 * shared across the rest of the app without duplicating the class string /
 * style object per page. Deliberately NOT applied to DB Manager, which
 * stays utilitarian/dense.
 */
export const COZY_CARD_CLASS =
  'border-0 rounded-[var(--radius)] shadow-[0_1px_2px_oklch(0.35_0.03_40/0.07),0_12px_28px_-10px_oklch(0.35_0.06_40/0.18)] transition-shadow duration-300 hover:shadow-[0_2px_4px_oklch(0.35_0.03_40/0.09),0_18px_36px_-10px_oklch(0.35_0.07_40/0.22)]'

// eslint-disable-next-line react-refresh/only-export-components -- shadcn/ui convention: variants/constants exported alongside the component
export const paperSurfaceStyle: React.CSSProperties = {
  backgroundColor: 'color-mix(in oklch, var(--primary) 3%, var(--card))',
  backgroundImage:
    'repeating-linear-gradient(115deg, oklch(0.3 0.02 40 / 0.025) 0px, oklch(0.3 0.02 40 / 0.025) 1px, transparent 1px, transparent 4px)',
}

/**
 * Drop-in replacement for `Card` that applies the cozy paper + shadow
 * treatment automatically. Use together with the existing `CardHeader`,
 * `CardContent`, `CardTitle`, `CardDescription`, `CardFooter` — only the
 * outer surface changes.
 */
const CozyCard = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, style, ...props }, ref) => (
    <Card ref={ref} className={cn(COZY_CARD_CLASS, className)} style={{ ...paperSurfaceStyle, ...style }} {...props} />
  )
)
CozyCard.displayName = 'CozyCard'

export { CozyCard }
