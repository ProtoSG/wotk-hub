import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { QueryResult } from '@/types/db.types'

interface Props {
  results: QueryResult
}

export default function ResultsTable({ results }: Props) {
  const { columns, rows } = results

  return (
    <div className="max-h-64 overflow-auto rounded-md border">
      <table className="w-max min-w-full caption-bottom text-sm">
        <TableHeader>
          <TableRow>
            {columns.map((col) => (
              <TableHead key={col} className="whitespace-nowrap sticky top-0 bg-card z-10">
                {col}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, i) => (
            <TableRow key={i}>
              {columns.map((col) => (
                <TableCell key={col} className="font-mono text-xs whitespace-nowrap">
                  {row[col] === null || row[col] === undefined ? (
                    <span className="text-muted-foreground italic">NULL</span>
                  ) : (
                    String(row[col])
                  )}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </table>
    </div>
  )
}
