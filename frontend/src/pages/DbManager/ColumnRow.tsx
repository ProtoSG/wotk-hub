import { Badge } from '@/components/ui/badge'
import type { ColumnInfo } from '@/types/db.types'

export default function ColumnRow({ name, type, nullable }: ColumnInfo) {
  return (
    <div className="flex items-center gap-2 py-1 pl-6 text-xs">
      <span className="flex-1 text-muted-foreground">{name}</span>
      <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
        {type}
      </Badge>
      {nullable && (
        <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-muted-foreground">
          null
        </Badge>
      )}
    </div>
  )
}
