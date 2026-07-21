import { useQuery } from '@tanstack/react-query'
import { CalendarDays, ChevronRight, History } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { useGymApi } from '@/hooks/useGymApi'
import { formatVolume } from '@/lib/weight'
import type { SessionSummary } from '@/types/gym.types'
import { sessionsKey } from './gymKeys'
import { formatDuration } from './useElapsed'

interface HistorialTabProps {
  onOpen: (session: SessionSummary) => void
}

export default function HistorialTab({ onOpen }: HistorialTabProps) {
  const { listSessions } = useGymApi()

  const { data: sessions = [], isPending } = useQuery({
    queryKey: sessionsKey(),
    queryFn: () => listSessions(),
  })

  if (isPending) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-lg" />
        ))}
      </div>
    )
  }

  if (sessions.length === 0) {
    return (
      <EmptyState
        icon={<History />}
        title="Todavía no hay entrenamientos"
        description="Cuando termines el primero va a aparecer acá, con sus series y su volumen."
      />
    )
  }

  // Grouped by month so a long history stays scannable without a date filter.
  const groups = groupByMonth(sessions)

  return (
    <div className="space-y-6">
      {groups.map(([month, monthSessions]) => (
        <section key={month} className="space-y-2">
          <h2 className="px-1 text-sm font-medium text-muted-foreground first-letter:uppercase">
            {month}
          </h2>
          <ul className="space-y-2">
            {monthSessions.map((session) => (
              <li key={session.id}>
                <button
                  type="button"
                  onClick={() => onOpen(session)}
                  className="flex min-h-[44px] w-full items-center gap-3 rounded-lg border bg-card px-4 py-3 text-left transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-medium">
                      {session.name || 'Entrenamiento libre'}
                      {session.finishedAt === null && (
                        <span className="ml-2 text-sm font-normal text-primary">En curso</span>
                      )}
                    </p>
                    <p className="mt-0.5 flex flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground tabular-nums">
                      <CalendarDays className="h-3.5 w-3.5" />
                      {formatDayLabel(session.occurredOn)}
                      <span aria-hidden>·</span>
                      {session.exerciseCount} {session.exerciseCount === 1 ? 'ejercicio' : 'ejercicios'}
                      {session.totalVolumeGrams > 0 && (
                        <>
                          <span aria-hidden>·</span>
                          {formatVolume(session.totalVolumeGrams)}
                        </>
                      )}
                      {session.finishedAt && (
                        <>
                          <span aria-hidden>·</span>
                          {formatDuration(durationMinutes(session))}
                        </>
                      )}
                    </p>
                  </div>
                  <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

function groupByMonth(sessions: SessionSummary[]): [string, SessionSummary[]][] {
  const groups = new Map<string, SessionSummary[]>()
  for (const session of sessions) {
    const label = new Date(`${session.occurredOn}T00:00:00`).toLocaleDateString('es', {
      month: 'long',
      year: 'numeric',
    })
    const bucket = groups.get(label)
    if (bucket) bucket.push(session)
    else groups.set(label, [session])
  }
  return [...groups.entries()]
}

function formatDayLabel(occurredOn: string): string {
  return new Date(`${occurredOn}T00:00:00`).toLocaleDateString('es', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
  })
}

function durationMinutes(session: SessionSummary): number {
  if (!session.finishedAt) return 0
  const started = new Date(session.startedAt).getTime()
  const finished = new Date(session.finishedAt).getTime()
  if (Number.isNaN(started) || Number.isNaN(finished)) return 0
  return Math.max(0, Math.round((finished - started) / 60_000))
}
