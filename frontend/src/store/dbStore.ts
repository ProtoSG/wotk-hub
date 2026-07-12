import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { SavedConnection, ColumnInfo } from '@/types/db.types'

interface DbStore {
  connections: SavedConnection[]
  activeConnectionId: string | null
  schema: Record<string, ColumnInfo[]>
  isLoadingSchema: boolean

  addConnection: (c: SavedConnection) => void
  updateConnection: (id: string, partial: Partial<SavedConnection>) => void
  removeConnection: (id: string) => void
  setActiveConnection: (id: string | null) => void
  setSchema: (table: string, cols: ColumnInfo[]) => void
  clearSchema: () => void
  setLoadingSchema: (v: boolean) => void
}

export const useDbStore = create<DbStore>()(
  persist(
    (set) => ({
      connections: [],
      activeConnectionId: null,
      schema: {},
      isLoadingSchema: false,

      addConnection: (c) => set((s) => ({ connections: [...s.connections, c] })),
      updateConnection: (id, partial) =>
        set((s) => ({ connections: s.connections.map((c) => (c.id === id ? { ...c, ...partial } : c)) })),
      removeConnection: (id) =>
        set((s) => ({
          connections: s.connections.filter((c) => c.id !== id),
          activeConnectionId: s.activeConnectionId === id ? null : s.activeConnectionId,
        })),
      setActiveConnection: (id) => set({ activeConnectionId: id, schema: {} }),
      setSchema: (table, cols) => set((s) => ({ schema: { ...s.schema, [table]: cols } })),
      clearSchema: () => set({ schema: {} }),
      setLoadingSchema: (v) => set({ isLoadingSchema: v }),
    }),
    {
      name: 'work-hub-db',
      partialize: (s) => ({
        connections: s.connections.map(({ password, ...rest }) => rest),
      }),
    }
  )
)
