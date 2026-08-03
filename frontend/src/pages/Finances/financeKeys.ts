export function cardsKey() {
  return ['finances', 'cards'] as const
}

export function summaryKey(month: string) {
  return ['finances', 'summary', month] as const
}

export function subscriptionsKey() {
  return ['finances', 'subscriptions'] as const
}

export function goalsKey() {
  return ['finances', 'goals'] as const
}

export function budgetsKey(month: string) {
  return ['finances', 'budgets', month] as const
}

export function transactionsKey(month: string, typeFilter: string, categoryFilter: string) {
  return ['finances', 'transactions', month, typeFilter, categoryFilter] as const
}
