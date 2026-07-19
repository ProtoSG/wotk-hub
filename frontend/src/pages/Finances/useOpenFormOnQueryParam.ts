import { useEffect } from 'react'
import { flushSync } from 'react-dom'
import { useSearchParams } from 'react-router-dom'

// FAB navigation appends ?new=1 (see MobileTabNav / FloatingActionButton in
// FinancesPage) to signal "open the create form". flushSync forces the form
// state to commit before the param strip below re-renders, so the dialog is
// already open on the URL update — otherwise it can flash closed for a frame.
export function useOpenFormOnQueryParam(onOpen: () => void) {
  const [searchParams, setSearchParams] = useSearchParams()

  useEffect(() => {
    if (searchParams.get('new') === '1') {
      flushSync(() => {
        onOpen()
        setSearchParams(
          (prev) => {
            const next = new URLSearchParams(prev)
            next.delete('new')
            return next
          },
          { replace: true }
        )
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- setSearchParams identity is stable, only react to searchParams changing
  }, [searchParams])
}
