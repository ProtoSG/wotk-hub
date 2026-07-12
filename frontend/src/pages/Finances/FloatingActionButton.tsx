import { Plus } from 'lucide-react'

interface Props {
  onClick: () => void
  label: string
}

export default function FloatingActionButton({ onClick, label }: Props) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="fixed right-4 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-foreground text-background shadow-lg transition-transform active:scale-95 sm:hidden"
      style={{ bottom: 'env(safe-area-inset-bottom)' }}
    >
      <Plus className="h-6 w-6" />
    </button>
  )
}
