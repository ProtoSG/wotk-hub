import { useEffect, useState, useCallback, useRef } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  Position,
  Handle,
  MarkerType,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import dagre from '@dagrejs/dagre'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useDbStore } from '@/store/dbStore'
import { useThemeStore } from '@/store/themeStore'
import { useDbApi } from '@/hooks/useDbApi'
import type { ForeignKey, ColumnInfo } from '@/types/db.types'

interface TableNodeData {
  label: string
  columns: ColumnInfo[]
  fkColumns: Set<string>
  pkColumns: Set<string>
  [key: string]: unknown
}

function TableNodeComponent({ data }: { data: TableNodeData }) {
  return (
    <div className="rounded-lg border border-border bg-card shadow-md min-w-[180px] overflow-hidden">
      <Handle type="target" position={Position.Left} className="!bg-primary" />
      <Handle type="source" position={Position.Right} className="!bg-primary" />
      <div className="bg-primary/10 border-b border-border px-3 py-1.5">
        <span className="font-semibold text-xs text-foreground">{data.label}</span>
      </div>
      <div className="divide-y divide-border">
        {data.columns.map((col) => (
          <div key={col.name} className="flex items-center gap-2 px-3 py-1 text-[11px]">
            <span className={`flex-1 ${data.fkColumns.has(col.name) ? 'text-blue-400' : data.pkColumns.has(col.name) ? 'text-yellow-400' : 'text-muted-foreground'}`}>
              {data.pkColumns.has(col.name) && <span className="mr-1">🔑</span>}
              {data.fkColumns.has(col.name) && !data.pkColumns.has(col.name) && <span className="mr-1">🔗</span>}
              {col.name}
            </span>
            <Badge variant="secondary" className="text-[9px] px-1 py-0 h-4">{col.type}</Badge>
          </div>
        ))}
      </div>
    </div>
  )
}

const nodeTypes = { tableNode: TableNodeComponent }

// Matches TableNodeComponent's rendered size closely enough for dagre to
// space nodes without overlap (header row + one row per column).
const NODE_WIDTH = 240
const HEADER_H = 30
const ROW_H = 27

function nodeHeight(cols: ColumnInfo[]) {
  return HEADER_H + cols.length * ROW_H
}

// Lays tables out by FK relationships (dagre layered graph) instead of a
// fixed grid — grid slots didn't account for each table's actual height, so
// taller tables overlapped their neighbors below.
function layoutWithDagre(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'LR', nodesep: 60, ranksep: 120 })

  nodes.forEach((node) => {
    const { columns } = node.data as TableNodeData
    g.setNode(node.id, { width: NODE_WIDTH, height: nodeHeight(columns) })
  })
  edges.forEach((edge) => g.setEdge(edge.source, edge.target))

  dagre.layout(g)

  return nodes.map((node) => {
    const { columns } = node.data as TableNodeData
    const { x, y } = g.node(node.id)
    return { ...node, position: { x: x - NODE_WIDTH / 2, y: y - nodeHeight(columns) / 2 } }
  })
}

function buildGraph(
  tables: string[],
  schema: Record<string, ColumnInfo[]>,
  fks: ForeignKey[]
): { nodes: Node[]; edges: Edge[] } {
  const fkSet = new Map<string, Set<string>>()
  fks.forEach(({ fromTable, fromColumn }) => {
    if (!fkSet.has(fromTable)) fkSet.set(fromTable, new Set())
    fkSet.get(fromTable)!.add(fromColumn)
  })

  const nodes: Node[] = tables.map((table) => {
    const cols = schema[table] ?? []
    // Prefer the real primary-key flag from the backend; fall back to the name
    // heuristic only if the field is entirely absent from this schema's columns.
    const hasPkField = cols.some((c) => c.isPrimaryKey !== undefined)
    const pkColumns = hasPkField
      ? new Set<string>(cols.filter((c) => c.isPrimaryKey).map((c) => c.name))
      : new Set<string>(
          cols.filter((c) => c.name === 'id' || (c.name.endsWith('_id') && cols.indexOf(c) === 0)).map((c) => c.name)
        )

    return {
      id: table,
      type: 'tableNode',
      position: { x: 0, y: 0 },
      data: {
        label: table,
        columns: cols,
        fkColumns: fkSet.get(table) ?? new Set(),
        pkColumns,
      } satisfies TableNodeData,
    }
  })

  const edges: Edge[] = fks.map((fk, i) => ({
    id: `fk-${i}`,
    source: fk.fromTable,
    target: fk.toTable,
    label: `${fk.fromColumn} → ${fk.toColumn}`,
    labelStyle: { fontSize: 9, fill: 'var(--muted-foreground)' },
    labelBgStyle: { fill: 'var(--card)', fillOpacity: 0.8 },
    style: { stroke: 'var(--primary)', strokeWidth: 1.5 },
    markerEnd: { type: MarkerType.ArrowClosed, color: 'var(--primary)' },
    animated: false,
  }))

  return { nodes: layoutWithDagre(nodes, edges), edges }
}

export default function ERDiagram() {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { connections, activeConnectionId, schema } = useDbStore()
  const theme = useThemeStore((s) => s.theme)
  const { getForeignKeys, listTables, getSchema } = useDbApi()
  const { setSchema } = useDbStore()

  const activeConn = connections.find((c) => c.id === activeConnectionId)
  const requestIdRef = useRef(0)

  const loadDiagram = useCallback(async () => {
    if (!activeConn) return
    const myRequestId = ++requestIdRef.current
    setLoading(true)
    setError(null)
    try {
      const [tables, fks] = await Promise.all([
        listTables(activeConn),
        getForeignKeys(activeConn),
      ])
      if (requestIdRef.current !== myRequestId) return

      // fetch missing schemas, build fullSchema map directly
      const fullSchema: Record<string, ColumnInfo[]> = { ...schema }
      const missing = tables.filter((t) => !fullSchema[t])
      await Promise.all(missing.map(async (t) => {
        const cols = await getSchema(activeConn, t)
        if (requestIdRef.current !== myRequestId) return
        fullSchema[t] = cols
        setSchema(t, cols)
      }))
      if (requestIdRef.current !== myRequestId) return

      const { nodes: n, edges: e } = buildGraph(tables, fullSchema, fks)
      setNodes(n)
      setEdges(e)
    } catch (err) {
      if (requestIdRef.current === myRequestId) {
        setError(err instanceof Error ? err.message : 'Failed to load diagram')
      }
    } finally {
      if (requestIdRef.current === myRequestId) setLoading(false)
    }
  }, [activeConn, schema])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch-then-set on connection change, same pattern as other DbManager pages
    if (activeConn) loadDiagram()
    return () => {
      requestIdRef.current++
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeConnectionId])

  if (!activeConn) {
    return (
      <div className="flex h-full items-center justify-center">
        <p className="text-sm text-muted-foreground">Connect to a database to view the ER diagram</p>
      </div>
    )
  }

  return (
    <div className="relative h-full w-full">
      <div className="absolute top-3 right-3 z-10">
        <Button variant="outline" size="sm" onClick={loadDiagram} disabled={loading}>
          {loading ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
          Refresh
        </Button>
      </div>

      {error && (
        <div className="absolute top-3 left-3 z-10 rounded border border-destructive/50 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      {loading && nodes.length === 0 ? (
        <div className="flex h-full items-center justify-center gap-2">
          <Loader2 size={16} className="animate-spin text-muted-foreground" />
          <span className="text-sm text-muted-foreground">Loading diagram…</span>
        </div>
      ) : (
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          minZoom={0.2}
          maxZoom={2}
          colorMode={theme}
        >
          <Background gap={16} size={1} />
          <Controls />
          <MiniMap
            nodeColor={() => 'var(--primary)'}
            maskColor="var(--background)"
          />
        </ReactFlow>
      )}
    </div>
  )
}
