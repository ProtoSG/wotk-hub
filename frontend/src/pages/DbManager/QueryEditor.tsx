import { Loader2, Play, Trash2, Clock, CheckCircle2, XCircle } from 'lucide-react'
import { toast } from 'sonner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import ResultsTable from './ResultsTable'
import ERDiagram from './ERDiagram'
import { useQueryStore, type QueryResultSet } from '@/store/queryStore'
import { useDbStore } from '@/store/dbStore'
import { useDbApi } from '@/hooks/useDbApi'
import type { Dialect } from '@/types/db.types'

const DOLLAR_TAG_RE = /^\$[A-Za-z_][A-Za-z0-9_]*\$|^\$\$/

type ScanState =
  | 'normal'
  | 'line-comment'
  | 'block-comment'
  | 'single-quote'
  | 'double-quote'
  | 'dollar-quote'
  | 'backtick-quote'

/**
 * Splits a SQL script into individual statements on `;`, tracking `--` line
 * comments, `/* *\/` block comments, single-quoted strings, double-quoted
 * identifiers, Postgres dollar-quoted strings (`$tag$ ... $tag$`), and MySQL
 * backtick-quoted identifiers, so that semicolons inside any of those are not
 * treated as statement separators.
 * Backslash-escaping inside quoted strings only applies for MySQL — Postgres
 * (with default standard_conforming_strings) does not treat `\` specially.
 * Segments that contain only comments/whitespace are dropped rather than
 * emitted as empty "statements". An unterminated quoted string, backtick
 * identifier, or dollar-quote at end-of-input is a hard error — it would
 * otherwise silently swallow any real statements that follow it (comments
 * are exempt since they naturally end at EOF/newline).
 */
function splitStatements(sql: string, dialect: Dialect): string[] {
  const statements: string[] = []
  let current = ''
  let state: ScanState = 'normal'
  let dollarTag = ''
  let hasContent = false
  let i = 0
  const n = sql.length

  function flush() {
    if (hasContent) {
      const trimmed = current.trim()
      if (trimmed) statements.push(trimmed)
    }
    current = ''
    hasContent = false
  }

  while (i < n) {
    const ch = sql[i]
    const next = sql[i + 1]

    if (state === 'line-comment') {
      current += ch
      if (ch === '\n') state = 'normal'
      i++
      continue
    }

    if (state === 'block-comment') {
      if (ch === '*' && next === '/') {
        current += '*/'
        i += 2
        state = 'normal'
        continue
      }
      current += ch
      i++
      continue
    }

    if (state === 'single-quote' || state === 'double-quote') {
      const quoteChar = state === 'single-quote' ? "'" : '"'
      if (dialect === 'mysql' && ch === '\\' && next !== undefined) {
        current += ch + next
        hasContent = true
        i += 2
        continue
      }
      if (ch === quoteChar) {
        if (next === quoteChar) {
          // Doubled quote is an escaped literal quote, not a closing delimiter.
          current += quoteChar + quoteChar
          hasContent = true
          i += 2
          continue
        }
        current += ch
        hasContent = true
        state = 'normal'
        i++
        continue
      }
      current += ch
      hasContent = true
      i++
      continue
    }

    if (state === 'backtick-quote') {
      if (ch === '`') {
        if (next === '`') {
          // Doubled backtick is an escaped literal backtick, not a closing delimiter.
          current += '``'
          hasContent = true
          i += 2
          continue
        }
        current += ch
        hasContent = true
        state = 'normal'
        i++
        continue
      }
      current += ch
      hasContent = true
      i++
      continue
    }

    if (state === 'dollar-quote') {
      if (ch === '$' && sql.slice(i, i + dollarTag.length) === dollarTag) {
        current += dollarTag
        hasContent = true
        i += dollarTag.length
        state = 'normal'
        dollarTag = ''
        continue
      }
      current += ch
      hasContent = true
      i++
      continue
    }

    // state === 'normal'
    if (ch === '-' && next === '-') {
      current += '--'
      i += 2
      state = 'line-comment'
      continue
    }
    if (ch === '/' && next === '*') {
      current += '/*'
      i += 2
      state = 'block-comment'
      continue
    }
    if (ch === "'") {
      current += ch
      state = 'single-quote'
      hasContent = true
      i++
      continue
    }
    if (ch === '"') {
      current += ch
      state = 'double-quote'
      hasContent = true
      i++
      continue
    }
    if (dialect === 'mysql' && ch === '`') {
      current += ch
      state = 'backtick-quote'
      hasContent = true
      i++
      continue
    }
    if (dialect === 'postgres' && ch === '$') {
      const match = DOLLAR_TAG_RE.exec(sql.slice(i))
      if (match) {
        dollarTag = match[0]
        current += dollarTag
        hasContent = true
        i += dollarTag.length
        state = 'dollar-quote'
        continue
      }
    }
    if (ch === ';') {
      flush()
      i++
      continue
    }
    if (!/\s/.test(ch)) hasContent = true
    current += ch
    i++
  }

  if (state === 'single-quote' || state === 'double-quote' || state === 'dollar-quote' || state === 'backtick-quote') {
    throw new Error('Unterminated quoted string/dollar-quote in SQL — check for a missing closing delimiter')
  }

  flush()
  return statements
}

export default function QueryEditor() {
  const { currentQuery, resultSets, isExecuting, historyByConnection, setCurrentQuery, setResultSets, setExecuting, addToHistory, clearHistory, restoreFromHistory } =
    useQueryStore()
  const { connections, activeConnectionId } = useDbStore()
  const { runQuery } = useDbApi()

  const activeConn = connections.find((c) => c.id === activeConnectionId)
  const history = (activeConnectionId && historyByConnection[activeConnectionId]) || []

  async function handleRun() {
    if (!activeConn) { toast.error('No active connection'); return }
    let statements: string[]
    try {
      statements = splitStatements(currentQuery, activeConn.dialect)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to parse SQL')
      return
    }
    if (statements.length === 0) return

    setExecuting(true)
    setResultSets([])

    const sets: QueryResultSet[] = []
    let totalRows = 0
    let hasError = false

    for (const sql of statements) {
      try {
        const result = await runQuery(activeConn, sql)
        sets.push({ sql, result, error: null })
        totalRows += result.rowCount
      } catch (err) {
        const error = err instanceof Error ? err.message : 'Query failed'
        sets.push({ sql, result: null, error })
        hasError = true
        // continue running remaining statements
      }
      // update incrementally so user sees results as they come
      setResultSets([...sets])
    }

    setExecuting(false)

    addToHistory(activeConn.id, {
      id: crypto.randomUUID(),
      sql: currentQuery,
      executedAt: Date.now(),
      rowCount: totalRows,
    })

    if (hasError) {
      toast.error(`${sets.filter((s) => s.error).length} statement(s) failed`)
    } else {
      const label = statements.length > 1
        ? `${statements.length} queries · ${totalRows} total rows`
        : `${totalRows} rows`
      toast.success(label)
    }
  }

  return (
    <Tabs defaultValue="editor" className="flex flex-col h-full">
      <div className="flex items-center justify-between border-b px-3 py-2">
        <TabsList>
          <TabsTrigger value="editor">Editor</TabsTrigger>
          <TabsTrigger value="history">History ({history.length})</TabsTrigger>
          <TabsTrigger value="diagram">ER Diagram</TabsTrigger>
        </TabsList>
      </div>

      <TabsContent value="editor" className="flex flex-col gap-3 p-3 flex-1 mt-0 overflow-auto">
        <Textarea
          value={currentQuery}
          onChange={(e) => setCurrentQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.ctrlKey && e.key === 'Enter') handleRun()
          }}
          placeholder={`SELECT * FROM users LIMIT 10;\nSELECT count(*) FROM orders;\n\n(Ctrl+Enter to run all)`}
          className="font-mono text-sm resize-none min-h-[140px]"
          disabled={!activeConn}
        />

        <div className="flex gap-2">
          <Button size="sm" onClick={handleRun} disabled={isExecuting || !activeConn}>
            {isExecuting ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}
            {isExecuting ? 'Running…' : 'Run'}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => { setCurrentQuery(''); setResultSets([]) }}>
            <Trash2 size={14} />
            Clear
          </Button>
          {resultSets.length > 0 && !isExecuting && (
            <span className="text-xs text-muted-foreground self-center">
              {resultSets.length} statement{resultSets.length !== 1 ? 's' : ''}
              {' · '}
              {resultSets.filter((s) => s.error).length === 0
                ? `${resultSets.reduce((a, s) => a + (s.result?.rowCount ?? 0), 0)} total rows`
                : `${resultSets.filter((s) => s.error).length} error(s)`}
            </span>
          )}
        </div>

        {!activeConn && (
          <p className="text-xs text-muted-foreground">Connect to a database to run queries</p>
        )}

        {resultSets.length > 0 && (
          <div className="space-y-4">
            {resultSets.map((set, i) => (
              <ResultSetBlock key={i} index={i} set={set} total={resultSets.length} />
            ))}
          </div>
        )}
      </TabsContent>

      <TabsContent value="history" className="flex flex-col h-full mt-0">
        <div className="flex justify-end px-3 pt-2">
          <Button variant="ghost" size="sm" onClick={() => activeConnectionId && clearHistory(activeConnectionId)} disabled={history.length === 0}>
            Clear
          </Button>
        </div>
        <ScrollArea className="flex-1">
          <div className="space-y-1 p-2">
            {history.length === 0 ? (
              <p className="text-xs text-muted-foreground px-2 py-4 text-center">No history yet</p>
            ) : (
              history.map((entry) => (
                <button
                  key={entry.id}
                  onClick={() => restoreFromHistory(entry)}
                  className="w-full text-left rounded-md p-2 hover:bg-accent space-y-1"
                >
                  <p className="font-mono text-xs truncate">{entry.sql}</p>
                  <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
                    <Clock size={10} />
                    <span>{new Date(entry.executedAt).toLocaleTimeString()}</span>
                    <span>·</span>
                    <span>{entry.rowCount} rows</span>
                  </div>
                </button>
              ))
            )}
          </div>
        </ScrollArea>
      </TabsContent>

      <TabsContent value="diagram" className="flex-1 mt-0 overflow-hidden">
        <ERDiagram />
      </TabsContent>
    </Tabs>
  )
}

function ResultSetBlock({ index, set, total }: { index: number; set: QueryResultSet; total: number }) {
  return (
    <div className="rounded-md border overflow-hidden">
      <div className="flex items-center gap-2 bg-muted/50 px-3 py-1.5 border-b">
        {set.error
          ? <XCircle size={13} className="text-destructive shrink-0" />
          : <CheckCircle2 size={13} className="text-success shrink-0" />}
        {total > 1 && (
          <span className="text-[11px] font-medium text-muted-foreground shrink-0">#{index + 1}</span>
        )}
        <span className="font-mono text-[11px] truncate text-muted-foreground">{set.sql}</span>
        {set.result && (
          <span className="ml-auto text-[10px] text-muted-foreground shrink-0">
            {set.result.rowCount}r · {set.result.executionTimeMs}ms
          </span>
        )}
      </div>
      <div className="p-3">
        {set.error ? (
          <p className="font-mono text-xs text-destructive">{set.error}</p>
        ) : set.result ? (
          <ResultsTable results={set.result} />
        ) : null}
      </div>
    </div>
  )
}
