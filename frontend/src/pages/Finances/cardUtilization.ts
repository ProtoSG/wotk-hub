import type { Card } from '@/types/finance.types'

export function getCardUtilization(card: Card, cardsLength: number) {
  const hasCreditLimit = card.creditLimitCents > 0
  const utilization =
    hasCreditLimit && card.creditLimitCents > 0 ? card.usedCreditCents / card.creditLimitCents : 0
  const utilizationColor =
    utilization > 0.8 ? 'bg-destructive' : utilization > 0.5 ? 'bg-warning' : 'bg-success'
  // The backend rejects archiving your last active card with 409
  // (cards.go DeleteCard). Disable the affordance here too so the
  // user doesn't trip the failure — the helpful title explains why.
  const isLastCard = cardsLength === 1

  return { hasCreditLimit, utilization, utilizationColor, isLastCard }
}
