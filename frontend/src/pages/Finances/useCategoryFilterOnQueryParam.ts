import { useEffect } from 'react'
import { flushSync } from 'react-dom'
import { useSearchParams } from 'react-router-dom'

// Resumen's CategoryChart drill-down appends ?category=<name> (see
// ResumenTab/FinancesPage) to pre-filter Movimientos when jumping here from
// a chart click. Same flushSync-then-strip pattern as
// useOpenFormOnQueryParam, for the same reason — avoid a one-frame flash of
// the unfiltered list before the param strip commits.
export function useCategoryFilterOnQueryParam(onCategory: (category: string) => void) {
  const [searchParams, setSearchParams] = useSearchParams()

  useEffect(() => {
    const category = searchParams.get('category')
    if (category) {
      flushSync(() => {
        onCategory(category)
        setSearchParams(
          (prev) => {
            const next = new URLSearchParams(prev)
            next.delete('category')
            return next
          },
          { replace: true }
        )
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- setSearchParams identity is stable, only react to searchParams changing
  }, [searchParams])
}
