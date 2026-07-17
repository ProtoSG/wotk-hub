import { cn } from '@/lib/utils'

interface ProgressProps {
  value: number // 0..100 (values above 100 are clamped)
  className?: string
  indicatorClassName?: string
  indicatorStyle?: React.CSSProperties
}

function Progress({ value, className, indicatorClassName, indicatorStyle }: ProgressProps) {
  const clamped = Math.min(Math.max(value, 0), 100)
  return (
    <div className={cn('relative h-2 w-full overflow-hidden rounded-full bg-secondary', className)}>
      <div
        className={cn('h-full bg-primary transition-all', indicatorClassName)}
        style={{ width: `${clamped}%`, ...indicatorStyle }}
      />
    </div>
  )
}

export { Progress }
