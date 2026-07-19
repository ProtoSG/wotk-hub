import { useCallback, useEffect, useState } from 'react'
import { SlidersHorizontal } from 'lucide-react'
import {
  Carousel,
  CarouselContent,
  CarouselItem,
  type CarouselApi,
} from '@/components/ui/carousel'
import { cn } from '@/lib/utils'
import { formatPEN } from '@/lib/currency'
import type { Card } from '@/types/finance.types'
import { getCardUtilization } from './cardUtilization'
import { CardTextureOverlay } from './cardVisuals'

interface Props {
  cards: Card[]
  transactionsCount: number
  onCardChange: (cardId: number | null) => void
}

// Mobile-only full-width card carousel that replaces the desktop chip row.
// Slide 0 is "Todos" (parity with the chip row's "all cards" filter), then
// one slide per card. Swiping drives the same setCardFilter the chip row
// used, so filteredTransactions in MovimientosTab needs no changes.
export default function CardCarousel({ cards, transactionsCount, onCardChange }: Props) {
  const [api, setApi] = useState<CarouselApi>()
  const [selectedIndex, setSelectedIndex] = useState(0)

  const slideCount = 1 + cards.length

  const handleSelect = useCallback(
    (emblaApi: CarouselApi) => {
      if (!emblaApi) return
      const index = emblaApi.selectedScrollSnap()
      setSelectedIndex(index)
      onCardChange(index === 0 ? null : cards[index - 1].id)
    },
    [cards, onCardChange]
  )

  // Only 'select' (a real swipe/drag) reports back to the parent via
  // onCardChange. This component stays mounted (just CSS-hidden) on desktop,
  // and `cards` gets a new array reference on every transaction mutation
  // (cardsKey invalidation) — calling onCardChange from a mount/reInit sync
  // would silently reset the desktop chip-row's cardFilter to "Todos" on
  // every save. reInit only resyncs the local dot indicator, since embla can
  // clamp the snap index if the card list shrinks.
  useEffect(() => {
    if (!api) return
    api.on('select', handleSelect)
    const resyncIndex = (emblaApi: CarouselApi) => {
      if (!emblaApi) return
      setSelectedIndex(emblaApi.selectedScrollSnap())
    }
    api.on('reInit', resyncIndex)
    return () => {
      api.off('select', handleSelect)
      api.off('reInit', resyncIndex)
    }
  }, [api, handleSelect])

  return (
    <div className="sm:hidden">
      <Carousel setApi={setApi} opts={{ align: 'start', loop: false }}>
        <CarouselContent className="-ml-3">
          <CarouselItem className="basis-full pl-3">
            <div className="flex h-32 flex-col justify-between rounded-xl border-2 border-dashed border-muted-foreground/30 bg-muted/30 p-4 shadow-sm">
              <div className="flex items-start justify-between">
                <span className="text-sm font-medium text-foreground/90">Todos</span>
                <SlidersHorizontal className="text-foreground/50" size={16} />
              </div>
              <span className="text-xs text-foreground/40">{transactionsCount} movimientos</span>
            </div>
          </CarouselItem>

          {cards.map((card) => {
            const { hasCreditLimit, utilization, utilizationColor } = getCardUtilization(
              card,
              cards.length
            )

            return (
              <CarouselItem key={card.id} className="basis-full pl-3">
                <div
                  className="relative flex h-32 flex-col justify-between overflow-hidden rounded-xl p-4 shadow-md"
                  style={{ backgroundColor: card.color }}
                >
                  <CardTextureOverlay />
                  <div className="flex items-start justify-between">
                    <span className="truncate text-sm font-medium text-white/90">{card.name}</span>
                    <span className="text-xs text-white/70">•••• {card.last4}</span>
                  </div>

                  {hasCreditLimit ? (
                    <div className="space-y-1">
                      <div className="flex items-baseline justify-between">
                        <span className="text-xs text-white/70">
                          Usado: {formatPEN(card.usedCreditCents)}
                        </span>
                        <span className="text-sm font-semibold text-white">
                          {formatPEN(card.creditLimitCents - card.usedCreditCents)}{' '}
                          <span className="text-xs font-normal text-white/70">disponible</span>
                        </span>
                      </div>
                      {card.creditLimitCents > 0 && (
                        <div className="h-1.5 w-full overflow-hidden rounded-full bg-white/20">
                          <div
                            className={cn('h-full rounded-full transition-all', utilizationColor)}
                            style={{ width: `${Math.min(utilization * 100, 100)}%` }}
                          />
                        </div>
                      )}
                    </div>
                  ) : (
                    <span className="text-sm font-semibold text-white/90">
                      {formatPEN(card.balanceCents)}
                    </span>
                  )}
                </div>
              </CarouselItem>
            )
          })}
        </CarouselContent>
      </Carousel>

      {slideCount > 1 && (
        <div className="mt-2 flex justify-center gap-1.5">
          {Array.from({ length: slideCount }).map((_, i) => (
            <span
              key={i}
              className={cn(
                'h-1.5 rounded-full transition-all',
                i === selectedIndex ? 'w-4 bg-primary' : 'w-1.5 bg-muted-foreground/30'
              )}
            />
          ))}
        </div>
      )}
    </div>
  )
}
