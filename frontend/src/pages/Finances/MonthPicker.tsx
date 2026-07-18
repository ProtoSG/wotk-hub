import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { monthLabel, monthLabelShort, shiftMonth } from '@/lib/currency'

interface Props {
  month: string
  onChange: (month: string) => void
}

export default function MonthPicker({ month, onChange }: Props) {
  return (
    <div className="flex items-center gap-1">
      <Button variant="ghost" size="icon" onClick={() => onChange(shiftMonth(month, -1))} aria-label="Mes anterior">
        <ChevronLeft size={16} />
      </Button>
      <span className="min-w-16 text-center text-sm font-medium capitalize sm:hidden">{monthLabelShort(month)}</span>
      <span className="hidden min-w-32 text-center text-sm font-medium capitalize sm:inline">{monthLabel(month)}</span>
      <Button variant="ghost" size="icon" onClick={() => onChange(shiftMonth(month, 1))} aria-label="Mes siguiente">
        <ChevronRight size={16} />
      </Button>
    </div>
  )
}
