import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { cardsKey, summaryKey, subscriptionsKey, goalsKey } from './financeKeys'

// Resumen's data (summary/subscriptions/goals) used to be fetched inside
// ResumenTab itself, which only mounts once the onboarding gate resolves — a
// serialized extra round trip on the LCP path (gate fetch, then wait, then
// this fetch). Fetching it here starts it in parallel with the cards query
// instead. subscriptionsKey()/goalsKey() are shared with SuscripcionesTab
// and MetasTab so switching tabs reuses this cache instead of re-fetching.
export function useFinancesPageData(month: string) {
  const { listCards, getSummary, listSubscriptions, listGoals } = useFinanceApi()
  const queryClient = useQueryClient()

  // isPending is true while cards are still loading (no cached data yet) so
  // callers don't flash the onboarding gate before the first listCards resolves.
  const { data: cards, isPending: cardsPending } = useQuery({
    queryKey: cardsKey(),
    queryFn: () => listCards(),
  })
  const { data: summary, isPending: summaryPending } = useQuery({
    queryKey: summaryKey(month),
    queryFn: () => getSummary(month),
  })
  const { data: subscriptionsData, isPending: subscriptionsPending } = useQuery({
    queryKey: subscriptionsKey(),
    queryFn: () => listSubscriptions(),
  })
  const { data: goals = [], isPending: goalsPending } = useQuery({
    queryKey: goalsKey(),
    queryFn: () => listGoals(),
  })

  return {
    cardsList: cards ?? [],
    cardsPending,
    summary: summary ?? null,
    committed: subscriptionsData?.monthlyCommittedCents ?? 0,
    goals,
    resumenLoading: summaryPending || subscriptionsPending || goalsPending,
    invalidateCards: () => queryClient.invalidateQueries({ queryKey: cardsKey() }),
  }
}
