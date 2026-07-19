import type { Budget } from '@/types/finance.types'

const DANGER_STRIPE_STYLE = {
  backgroundImage:
    'repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.2)_4px,rgba(255,255,255,0.2)_8px)',
}

export function getBudgetStatus(budget: Budget) {
  const pct = budget.monthlyLimitCents > 0 ? (budget.spentCents / budget.monthlyLimitCents) * 100 : 0
  const over = budget.spentCents > budget.monthlyLimitCents
  const isDanger = over || pct >= 80
  const indicatorColor = over ? 'bg-destructive' : pct >= 80 ? 'bg-warning' : 'bg-primary'
  const stripeStyle = isDanger ? DANGER_STRIPE_STYLE : {}

  return { pct, over, isDanger, indicatorColor, stripeStyle }
}
