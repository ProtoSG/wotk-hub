import { useRef } from 'react'
import { toast } from 'sonner'

const UNDO_WINDOW_MS = 4500

interface UndoableDeleteConfig<TItem, TId extends string | number> {
  getId: (item: TItem) => TId
  deleteFn: (id: TId) => Promise<unknown>
  removeFromCache: (item: TItem) => number
  restoreToCache: (item: TItem, removedIndex: number) => void
  successMessage: string
  errorMessage: string
  onDeleteError?: () => void
}

// Shared "optimistic delete with an undo toast" flow: yank the row from the
// query cache immediately, schedule the real delete after UNDO_WINDOW_MS, and
// restore the row if the user hits "Deshacer" before the timer fires. Cache
// shape varies per tab (plain array vs. a wrapped {items, total} object), so
// callers own the actual setQueryData calls via removeFromCache/restoreToCache.
export function useUndoableDelete<TItem, TId extends string | number>(
  config: UndoableDeleteConfig<TItem, TId>
) {
  const pendingDeletes = useRef(new Map<TId, number>())

  async function commitDelete(id: TId) {
    pendingDeletes.current.delete(id)
    try {
      await config.deleteFn(id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : config.errorMessage)
      config.onDeleteError?.()
    }
  }

  function handleDelete(item: TItem) {
    const id = config.getId(item)
    const removedIndex = config.removeFromCache(item)

    const timer = window.setTimeout(() => commitDelete(id), UNDO_WINDOW_MS)
    pendingDeletes.current.set(id, timer)

    toast.success(config.successMessage, {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(id)
          if (timerId !== undefined) {
            window.clearTimeout(timerId)
            pendingDeletes.current.delete(id)
          }
          config.restoreToCache(item, removedIndex)
        },
      },
    })
  }

  return { handleDelete }
}
