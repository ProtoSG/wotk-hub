import api from '@/lib/axios'
import type { SavedConnection, ColumnInfo, QueryResult, ForeignKey } from '@/types/db.types'

function connParams(c: SavedConnection) {
  return {
    dialect: c.dialect,
    host: c.host,
    port: c.port,
    user: c.user,
    password: c.password,
    database: c.database,
  }
}

export function useDbApi() {
  async function testConnection(conn: SavedConnection): Promise<void> {
    await api.post('/api/db/connect', connParams(conn))
  }

  async function listTables(conn: SavedConnection): Promise<string[]> {
    const res = await api.post<{ tables: string[] }>('/api/db/tables', connParams(conn))
    return res.data.tables
  }

  async function getSchema(conn: SavedConnection, table: string): Promise<ColumnInfo[]> {
    const res = await api.post<{ columns: ColumnInfo[] }>(`/api/db/table/${table}/schema`, connParams(conn))
    return res.data.columns
  }

  async function runQuery(conn: SavedConnection, sql: string): Promise<QueryResult> {
    const res = await api.post<QueryResult>('/api/db/query', { ...connParams(conn), sql })
    return res.data
  }

  async function getForeignKeys(conn: SavedConnection): Promise<ForeignKey[]> {
    const res = await api.post<{ relationships: ForeignKey[] }>('/api/db/relationships', connParams(conn))
    return res.data.relationships
  }

  return { testConnection, listTables, getSchema, runQuery, getForeignKeys }
}
