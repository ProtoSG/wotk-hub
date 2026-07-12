import { useRef, useState } from 'react'
import { Plus, MoreVertical, Plug, Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import ConnectionForm from './ConnectionForm'
import { useDbStore } from '@/store/dbStore'
import { useDbApi } from '@/hooks/useDbApi'
import type { SavedConnection } from '@/types/db.types'
import { cn } from '@/lib/utils'

export default function ConnectionsSidebar() {
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SavedConnection | null>(null)
  const { connections, activeConnectionId, setActiveConnection, removeConnection } = useDbStore()
  const { testConnection } = useDbApi()
  const requestIdRef = useRef(0)

  async function handleConnect(conn: SavedConnection) {
    if (!conn.password) {
      // Password isn't persisted (see dbStore partialize) — ask the user to re-enter it.
      setEditing(conn)
      setFormOpen(true)
      return
    }

    const myRequestId = ++requestIdRef.current
    try {
      await testConnection(conn)
      if (requestIdRef.current === myRequestId) {
        setActiveConnection(conn.id)
        toast.success(`Connected to ${conn.name}`)
      }
    } catch (err) {
      if (requestIdRef.current === myRequestId) {
        const msg = err instanceof Error ? err.message : 'Connection failed'
        toast.error(msg)
      }
    }
  }

  return (
    <div className="flex h-full flex-col border-r">
      <div className="flex items-center justify-between border-b px-3 py-2">
        <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Connections</span>
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={() => { setEditing(null); setFormOpen(true) }}
          aria-label="Add connection"
        >
          <Plus size={14} />
        </Button>
      </div>
      <ScrollArea className="flex-1">
        <div className="space-y-1 p-2">
          {connections.length === 0 ? (
            <p className="text-center text-xs text-muted-foreground py-4">No connections yet</p>
          ) : (
            connections.map((conn) => (
              <div
                key={conn.id}
                className={cn(
                  'group flex items-center gap-2 rounded-md px-2 py-2 cursor-pointer hover:bg-accent',
                  activeConnectionId === conn.id && 'bg-accent'
                )}
                onClick={() => handleConnect(conn)}
              >
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-medium truncate">{conn.name}</p>
                  <p className="text-[10px] text-muted-foreground truncate">{conn.host}/{conn.database}</p>
                </div>
                <Badge variant="outline" className="text-[10px] px-1 py-0 shrink-0">
                  {conn.dialect === 'postgres' ? 'PG' : 'MY'}
                </Badge>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-5 w-5 opacity-0 group-hover:opacity-100"
                      aria-label={`Connection options for ${conn.name}`}
                    >
                      <MoreVertical size={12} />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); handleConnect(conn) }}>
                      <Plug size={12} /> Connect
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); setEditing(conn); setFormOpen(true) }}>
                      <Pencil size={12} /> Edit
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      className="text-destructive"
                      onClick={(e) => { e.stopPropagation(); removeConnection(conn.id) }}
                    >
                      <Trash2 size={12} /> Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            ))
          )}
        </div>
      </ScrollArea>
      <ConnectionForm open={formOpen} onClose={() => { setFormOpen(false); setEditing(null) }} editing={editing} />
    </div>
  )
}
