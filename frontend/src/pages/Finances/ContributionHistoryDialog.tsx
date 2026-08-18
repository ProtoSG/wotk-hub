import { useQuery } from '@tanstack/react-query'
import { History } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import type { SavingsGoal } from '@/types/finance.types'
import { contributionsKey } from './financeKeys'

interface Props {
  open: boolean
  onClose: () => void
  goal: SavingsGoal | null
}

// nextBillingOn-style date-only string ("2026-08-15") — same off-by-one fix
// as SubscriptionsMobileList's formatShortLocalDate: parsed alone, Date
// treats it as UTC midnight, which shifts a day back in Peru's UTC-5.
function formatLocalDate(dateOnly: string): string {
  const date = new Date(`${dateOnly}T00:00:00`)
  if (Number.isNaN(date.getTime())) return dateOnly
  return date.toLocaleDateString('es-PE', { day: 'numeric', month: 'short', year: 'numeric' })
}

export default function ContributionHistoryDialog({ open, onClose, goal }: Props) {
  const { listContributions } = useFinanceApi()

  const { data: contributions = [], isPending } = useQuery({
    queryKey: contributionsKey(goal?.id ?? 0),
    queryFn: () => listContributions(goal!.id),
    enabled: open && !!goal,
  })

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Aportes a {goal?.name}</DialogTitle>
          <DialogDescription>Historial de ahorros registrados para esta meta</DialogDescription>
        </DialogHeader>
        {isPending ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        ) : contributions.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground">
            <History className="h-8 w-8 opacity-30" />
            <p className="text-sm">Todavía no registraste ningún aporte</p>
          </div>
        ) : (
          <div className="max-h-80 space-y-1 overflow-y-auto">
            {contributions.map((c) => (
              <div
                key={c.id}
                className="flex items-center justify-between rounded-md px-2 py-2 text-sm hover:bg-muted/50"
              >
                <span className="text-muted-foreground">{formatLocalDate(c.date)}</span>
                <span className="font-medium" style={{ color: goal?.color }}>
                  +{formatPEN(c.amountCents)}
                </span>
              </div>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
