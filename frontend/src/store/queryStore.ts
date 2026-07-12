import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { QueryResult } from '@/types/db.types'

interface HistoryEntry {
  id: string
  sql: string
  executedAt: number
  rowCount: number
}

export interface QueryResultSet {
  sql: string
  result: QueryResult | null
  error: string | null
}

interface QueryStore {
  currentQuery: string
  resultSets: QueryResultSet[]
  isExecuting: boolean
  historyByConnection: Record<string, HistoryEntry[]>

  setCurrentQuery: (q: string) => void
  setResultSets: (r: QueryResultSet[]) => void
  setExecuting: (v: boolean) => void
  addToHistory: (connectionId: string, entry: HistoryEntry) => void
  clearHistory: (connectionId: string) => void
  restoreFromHistory: (entry: HistoryEntry) => void
}

export const useQueryStore = create<QueryStore>()(
  persist(
    (set) => ({
      currentQuery: '',
      resultSets: [],
      isExecuting: false,
      historyByConnection: {},

      setCurrentQuery: (q) => set({ currentQuery: q }),
      setResultSets: (r) => set({ resultSets: r }),
      setExecuting: (v) => set({ isExecuting: v }),
      addToHistory: (connectionId, entry) =>
        set((s) => ({
          historyByConnection: {
            ...s.historyByConnection,
            [connectionId]: [entry, ...(s.historyByConnection[connectionId] ?? [])].slice(0, 50),
          },
        })),
      clearHistory: (connectionId) =>
        set((s) => ({ historyByConnection: { ...s.historyByConnection, [connectionId]: [] } })),
      restoreFromHistory: (entry) => set({ currentQuery: entry.sql }),
    }),
    {
      name: 'work-hub-query',
      partialize: (s) => ({ historyByConnection: s.historyByConnection }),
    }
  )
)
