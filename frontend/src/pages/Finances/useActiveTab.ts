import { useSearchParams } from 'react-router-dom'

export function useActiveTab<T extends { value: string }>(tabs: readonly T[], fallback: string) {
  const [searchParams, setSearchParams] = useSearchParams()
  const param = searchParams.get('tab') ?? ''
  const tab = tabs.some((t) => t.value === param) ? param : fallback
  return { tab, setSearchParams } as const
}
