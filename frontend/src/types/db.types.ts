export type Dialect = 'postgres' | 'mysql'

export interface SavedConnection {
  id: string
  name: string
  dialect: Dialect
  host: string
  port: number
  user: string
  password: string
  database: string
}

export interface ColumnInfo {
  name: string
  type: string
  nullable: boolean
  default: string | null
  /** Real primary-key flag from the backend, when available (falls back to a name heuristic if absent). */
  isPrimaryKey?: boolean
}

export interface QueryResult {
  columns: string[]
  rows: Record<string, unknown>[]
  rowCount: number
  executionTimeMs: number
}

export interface ForeignKey {
  fromTable: string
  fromColumn: string
  toTable: string
  toColumn: string
}
