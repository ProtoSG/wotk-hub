import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface Tab {
  value: string
  label: string
  icon: LucideIcon
}

interface Props {
  tabs: readonly Tab[]
  activeTab: string
  onChange: (value: string) => void
  fabVisible: boolean
}

export default function MobileTabNav({ tabs, activeTab, onChange, fabVisible }: Props) {
  return (
    <nav
      className="fixed left-4 z-40 flex h-14 items-center justify-around gap-0.5 rounded-full border bg-background px-2 shadow-lg sm:hidden"
      style={{
        right: fabVisible ? '5.5rem' : '1rem',
        bottom: 'max(env(safe-area-inset-bottom), 1rem)',
      }}
    >
      {tabs.map((t) => {
        const Icon = t.icon
        const active = activeTab === t.value
        return (
          <button
            key={t.value}
            type="button"
            onClick={() => onChange(t.value)}
            aria-label={t.label}
            className={cn(
              'flex h-11 flex-1 items-center justify-center rounded-full',
              active ? 'bg-muted text-foreground' : 'text-muted-foreground'
            )}
          >
            <Icon className="h-5 w-5" />
          </button>
        )
      })}
    </nav>
  )
}
