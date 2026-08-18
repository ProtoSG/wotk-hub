import type { Card } from '@/types/finance.types'

export function getCardUtilization(card: Card, cardsLength: number) {
  const hasCreditLimit = card.creditLimitCents > 0
  const utilization =
    hasCreditLimit && card.creditLimitCents > 0 ? card.usedCreditCents / card.creditLimitCents : 0
  // Same thresholds as budgetStatus.ts's over/near-limit split, so a "cerca
  // del límite" reads the same whether it's a budget or a card.
  const isOverLimit = hasCreditLimit && card.usedCreditCents >= card.creditLimitCents
  const isNearLimit = hasCreditLimit && !isOverLimit && utilization >= 0.8
  const utilizationColor = isOverLimit ? 'bg-destructive' : isNearLimit ? 'bg-warning' : 'bg-success'
  // The backend rejects archiving your last active card with 409
  // (cards.go DeleteCard). Disable the affordance here too so the
  // user doesn't trip the failure — the helpful title explains why.
  const isLastCard = cardsLength === 1

  return { hasCreditLimit, utilization, utilizationColor, isOverLimit, isNearLimit, isLastCard }
}
