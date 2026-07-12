import { ChevronRight } from 'lucide-react'
import { toast } from 'sonner'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Skeleton } from '@/components/ui/skeleton'
import ColumnRow from './ColumnRow'
import { useDbStore } from '@/store/dbStore'
import { useDbApi } from '@/hooks/useDbApi'
import type { SavedConnection } from '@/types/db.types'
import { useEffect, useRef, useState } from 'react'

interface Props {
  table: string
  connection: SavedConnection
}

export default function TableNode({ table, connection }: Props) {
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const { schema, setSchema } = useDbStore()
  const { getSchema } = useDbApi()
  const columns = schema[table]
  const requestIdRef = useRef(0)

  // Invalidate any in-flight request when the connection changes or the component unmounts.
  useEffect(() => {
    return () => {
      requestIdRef.current++
    }
  }, [connection.id])

  async function handleOpen(isOpen: boolean) {
    setOpen(isOpen)
    if (isOpen && !columns) {
      const myRequestId = ++requestIdRef.current
      setLoading(true)
      try {
        const cols = await getSchema(connection, table)
        if (requestIdRef.current === myRequestId) setSchema(table, cols)
      } catch (err) {
        if (requestIdRef.current === myRequestId) {
          toast.error(err instanceof Error ? err.message : `Failed to load schema for ${table}`)
        }
      } finally {
        if (requestIdRef.current === myRequestId) setLoading(false)
      }
    }
  }

  return (
    <Collapsible open={open} onOpenChange={handleOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-1.5 rounded px-2 py-1.5 text-xs hover:bg-accent group">
        <ChevronRight
          size={12}
          className="transition-transform group-data-[state=open]:rotate-90 text-muted-foreground"
        />
        <span className="font-medium">{table}</span>
      </CollapsibleTrigger>
      <CollapsibleContent>
        {loading ? (
          <div className="space-y-1 pl-4 py-1">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-5/6" />
          </div>
        ) : (
          columns?.map((col) => <ColumnRow key={col.name} {...col} />)
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}
