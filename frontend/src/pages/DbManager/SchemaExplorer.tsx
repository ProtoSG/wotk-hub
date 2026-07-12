import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import TableNode from './TableNode'
import { useDbStore } from '@/store/dbStore'
import { useDbApi } from '@/hooks/useDbApi'

export default function SchemaExplorer() {
  const [tables, setTables] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const { connections, activeConnectionId } = useDbStore()
  const { listTables } = useDbApi()

  const activeConn = connections.find((c) => c.id === activeConnectionId)

  useEffect(() => {
    if (!activeConn) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- reset on connection change, same pattern as other DbManager pages
      setTables([])
      return
    }
    let cancelled = false
    setLoading(true)
    listTables(activeConn)
      .then((result) => {
        if (!cancelled) setTables(result)
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setTables([])
          toast.error(err instanceof Error ? err.message : 'Failed to load tables')
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeConnectionId])

  if (!activeConn) {
    return (
      <div className="flex h-full items-center justify-center p-4">
        <p className="text-xs text-muted-foreground text-center">Select a connection to explore its schema</p>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      <div className="border-b px-3 py-2">
        <p className="text-xs font-semibold truncate">{activeConn.name}</p>
        <p className="text-[10px] text-muted-foreground">{activeConn.host}:{activeConn.port}</p>
      </div>
      <ScrollArea className="flex-1">
        <div className="p-2">
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-6 w-full" />
              ))}
            </div>
          ) : tables.length === 0 ? (
            <p className="text-xs text-muted-foreground px-2">No tables found</p>
          ) : (
            tables.map((t) => <TableNode key={t} table={t} connection={activeConn} />)
          )}
        </div>
      </ScrollArea>
      {loading && (
        <div className="flex items-center gap-1 border-t px-3 py-1.5 text-xs text-muted-foreground">
          <Loader2 size={10} className="animate-spin" />
          Loading schema…
        </div>
      )}
    </div>
  )
}
