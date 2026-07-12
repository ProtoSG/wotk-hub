import { Loader2 } from 'lucide-react'

export default function RouteFallback() {
  return (
    <div className="flex h-full min-h-[50vh] items-center justify-center">
      <Loader2 className="animate-spin text-muted-foreground" size={24} />
    </div>
  )
}
