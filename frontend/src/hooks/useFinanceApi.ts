import api from '@/lib/axios'
import type {
  Transaction,
  TransactionInput,
  TransactionFilters,
  Subscription,
  SubscriptionInput,
  Budget,
  FinanceSummary,
  Card,
  CardInput,
  CardReload,
  CardReloadInput,
  SavingsGoal,
  SavingsGoalInput,
  SavingsContribution,
  SavingsContributionInput,
} from '@/types/finance.types'

export function useFinanceApi() {
  async function listTransactions(filters: TransactionFilters = {}): Promise<Transaction[]> {
    const res = await api.get<{ transactions: Transaction[] }>('/api/finances/transactions', {
      params: filters,
    })
    return res.data.transactions
  }

  async function createTransaction(input: TransactionInput): Promise<Transaction> {
    const res = await api.post<Transaction>('/api/finances/transactions', input)
    return res.data
  }

  async function updateTransaction(id: number, input: TransactionInput): Promise<Transaction> {
    const res = await api.put<Transaction>(`/api/finances/transactions/${id}`, input)
    return res.data
  }

  async function deleteTransaction(id: number): Promise<void> {
    await api.delete(`/api/finances/transactions/${id}`)
  }

  async function refundTransaction(id: number): Promise<Transaction> {
    const res = await api.post<Transaction>(`/api/finances/transactions/${id}/refund`)
    return res.data
  }

  async function listSubscriptions(): Promise<{
    subscriptions: Subscription[]
    monthlyCommittedCents: number
  }> {
    const res = await api.get<{ subscriptions: Subscription[]; monthlyCommittedCents: number }>(
      '/api/finances/subscriptions'
    )
    return res.data
  }

  async function createSubscription(input: SubscriptionInput): Promise<Subscription> {
    const res = await api.post<Subscription>('/api/finances/subscriptions', input)
    return res.data
  }

  async function updateSubscription(id: number, input: SubscriptionInput): Promise<Subscription> {
    const res = await api.put<Subscription>(`/api/finances/subscriptions/${id}`, input)
    return res.data
  }

  async function deleteSubscription(id: number): Promise<void> {
    await api.delete(`/api/finances/subscriptions/${id}`)
  }

  async function listBudgets(month?: string): Promise<Budget[]> {
    const res = await api.get<{ budgets: Budget[] }>('/api/finances/budgets', {
      params: month ? { month } : {},
    })
    return res.data.budgets
  }

  async function upsertBudget(category: string, monthlyLimitCents: number): Promise<Budget> {
    const res = await api.put<Budget>(`/api/finances/budgets/${category}`, { monthlyLimitCents })
    return res.data
  }

  async function deleteBudget(category: string): Promise<void> {
    await api.delete(`/api/finances/budgets/${category}`)
  }

  async function getSummary(month?: string): Promise<FinanceSummary> {
    const res = await api.get<FinanceSummary>('/api/finances/summary', {
      params: month ? { month } : {},
    })
    return res.data
  }

  async function listCards(): Promise<Card[]> {
    const res = await api.get<{ cards: Card[] }>('/api/finances/cards')
    return res.data.cards
  }

  async function createCard(input: CardInput): Promise<Card> {
    const res = await api.post<Card>('/api/finances/cards', input)
    return res.data
  }

  async function updateCard(id: number, input: CardInput): Promise<Card> {
    const res = await api.put<Card>(`/api/finances/cards/${id}`, input)
    return res.data
  }

  async function deleteCard(id: number): Promise<void> {
    await api.delete(`/api/finances/cards/${id}`)
  }

  async function listReloads(cardId: number): Promise<CardReload[]> {
    const res = await api.get<{ reloads: CardReload[] }>(`/api/finances/cards/${cardId}/reloads`)
    return res.data.reloads
  }

  async function createReload(cardId: number, input: CardReloadInput): Promise<CardReload> {
    const res = await api.post<CardReload>(`/api/finances/cards/${cardId}/reloads`, input)
    return res.data
  }

  async function listGoals(): Promise<SavingsGoal[]> {
    const res = await api.get<{ goals: SavingsGoal[] }>('/api/finances/savings-goals')
    return res.data.goals
  }

  async function createGoal(input: SavingsGoalInput): Promise<SavingsGoal> {
    const res = await api.post<SavingsGoal>('/api/finances/savings-goals', input)
    return res.data
  }

  async function updateGoal(id: number, input: SavingsGoalInput): Promise<SavingsGoal> {
    const res = await api.put<SavingsGoal>(`/api/finances/savings-goals/${id}`, input)
    return res.data
  }

  async function deleteGoal(id: number): Promise<void> {
    await api.delete(`/api/finances/savings-goals/${id}`)
  }

  async function listContributions(goalId: number): Promise<SavingsContribution[]> {
    const res = await api.get<{ contributions: SavingsContribution[] }>(
      `/api/finances/savings-goals/${goalId}/contributions`
    )
    return res.data.contributions
  }

  async function createContribution(goalId: number, input: SavingsContributionInput): Promise<SavingsContribution> {
    const res = await api.post<SavingsContribution>(
      `/api/finances/savings-goals/${goalId}/contributions`,
      input
    )
    return res.data
  }

  return {
    listTransactions,
    createTransaction,
    updateTransaction,
    deleteTransaction,
    listSubscriptions,
    createSubscription,
    updateSubscription,
    deleteSubscription,
    listBudgets,
    upsertBudget,
    deleteBudget,
    getSummary,
    listCards,
    createCard,
    updateCard,
    deleteCard,
    listReloads,
    createReload,
    listGoals,
    createGoal,
    updateGoal,
    deleteGoal,
    listContributions,
    createContribution,
  }
}
