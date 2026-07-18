import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import api from '@/lib/axios'
import type { ApiKey } from '@/types/auth.types'

export interface CreatedApiKey extends ApiKey {
  key: string
}

async function listApiKeysApi(): Promise<ApiKey[]> {
  const res = await api.get<ApiKey[]>('/api/auth/keys')
  return res.data
}

export function useApiKeys() {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const refetch = useCallback(async () => {
    setIsLoading(true)
    try {
      setKeys(await listApiKeysApi())
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudieron cargar las API keys')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on mount, same pattern as DbManager pages
    refetch()
  }, [refetch])

  return { keys, isLoading, refetch }
}

export function useCreateApiKey() {
  return useCallback(async (name: string): Promise<CreatedApiKey> => {
    const res = await api.post<CreatedApiKey>('/api/auth/keys', { name })
    return res.data
  }, [])
}

export function useRevokeApiKey() {
  return useCallback(async (id: number): Promise<void> => {
    await api.delete(`/api/auth/keys/${id}`)
  }, [])
}
