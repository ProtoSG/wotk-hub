import api from '@/lib/axios'
import type {
  Transaction,
  TransactionInput,
  TransactionFilters,
  Subscription,
  SubscriptionInput,
  Budget,
  FinanceSummary,
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
  }
}
