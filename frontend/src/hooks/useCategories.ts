import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import api from '@/lib/axios'
import type { Category } from '@/types/finance.types'

export interface CategoryInput {
  name: string
  kind: 'expense' | 'income'
  label: string
}

export interface CategoriesByKind {
  expense: Category[]
  income: Category[]
}

async function listCategoriesApi(): Promise<Category[]> {
  const res = await api.get<{ categories: Category[] }>('/api/finances/categories')
  return res.data.categories
}

/**
 * Fetches every category and groups it by kind. Categories rarely change, so
 * there's no polling/cache-invalidation machinery here — consumers call
 * `refetch()` after a create/update/delete, same as `load()` in the other
 * Finances tabs.
 */
export function useCategories() {
  const [categories, setCategories] = useState<Category[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const refetch = useCallback(async () => {
    setIsLoading(true)
    try {
      setCategories(await listCategoriesApi())
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las categorías')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    refetch()
  }, [refetch])

  const data: CategoriesByKind = {
    expense: categories.filter((c) => c.kind === 'expense'),
    income: categories.filter((c) => c.kind === 'income'),
  }

  return { data, isLoading, refetch }
}

export function useCreateCategory() {
  return useCallback(async (input: CategoryInput): Promise<Category> => {
    const res = await api.post<Category>('/api/finances/categories', input)
    return res.data
  }, [])
}

export function useUpdateCategory() {
  return useCallback(async (id: number, input: CategoryInput): Promise<Category> => {
    const res = await api.put<Category>(`/api/finances/categories/${id}`, input)
    return res.data
  }, [])
}

export function useDeleteCategory() {
  return useCallback(async (id: number): Promise<void> => {
    await api.delete(`/api/finances/categories/${id}`)
  }, [])
}

/** Builds a `name -> label` lookup for display, e.g. `labels[t.category] ?? t.category`. */
export function toLabelMap(categories: Category[]): Record<string, string> {
  return Object.fromEntries(categories.map((c) => [c.name, c.label]))
}
