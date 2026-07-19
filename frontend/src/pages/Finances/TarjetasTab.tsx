import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, CreditCard, ArrowLeftRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { useFinanceApi } from '@/hooks/useFinanceApi'
import { formatPEN } from '@/lib/currency'
import type { Card } from '@/types/finance.types'
import { cardsKey } from './financeKeys'
import { useUndoableDelete } from './useUndoableDelete'
import { useOpenFormOnQueryParam } from './useOpenFormOnQueryParam'
import { getCardUtilization } from './cardUtilization'
import CardForm from './CardForm'
import TransferForm from './TransferForm'

export default function TarjetasTab() {
  const [formOpen, setFormOpen] = useState(false)
  const [editCard, setEditCard] = useState<Card | undefined>()
  const [transferOpen, setTransferOpen] = useState(false)
  const [selectedCard, setSelectedCard] = useState<Card | null>(null)
  const { listCards, deleteCard } = useFinanceApi()
  const queryClient = useQueryClient()

  const { data: cards = [] } = useQuery({
    queryKey: cardsKey(),
    queryFn: () => listCards(),
  })

  function invalidateCards() {
    queryClient.invalidateQueries({ queryKey: cardsKey() })
  }

  // A card transfer moves balance between two cards AND creates a
  // transfer-kind Transaction row — excluded from the Movimientos list, but
  // still lives in the same table, so invalidate both cache prefixes.
  function invalidateAfterTransfer() {
    queryClient.invalidateQueries({ queryKey: cardsKey() })
    queryClient.invalidateQueries({ queryKey: ['finances', 'transactions'] })
  }

  useOpenFormOnQueryParam(() => {
    setEditCard(undefined)
    setFormOpen(true)
  })

  const { handleDelete } = useUndoableDelete<Card, number>({
    getId: (c) => c.id,
    deleteFn: deleteCard,
    removeFromCache: (c) => {
      let removedIndex = -1
      queryClient.setQueryData(cardsKey(), (prev: Card[] = []) => {
        removedIndex = prev.findIndex((x) => x.id === c.id)
        return prev.filter((x) => x.id !== c.id)
      })
      return removedIndex
    },
    restoreToCache: (c, removedIndex) => {
      queryClient.setQueryData(cardsKey(), (prev: Card[] = []) => {
        const next = [...prev]
        next.splice(Math.min(removedIndex, next.length), 0, c)
        return next
      })
    },
    successMessage: 'Tarjeta eliminada',
    errorMessage: 'No se pudo eliminar la tarjeta',
    onDeleteError: invalidateCards,
  })

  return (
    <div className="space-y-4">
      <div className="hidden justify-end sm:flex">
        <Button
          onClick={() => {
            setEditCard(undefined)
            setFormOpen(true)
          }}
        >
          <Plus size={14} />
          Nueva tarjeta
        </Button>
      </div>

      {cards.length === 0 ? (
        <CozyCard className="animate-card-in">
          <CardContent className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground">
            <CreditCard className="h-10 w-10 opacity-30" />
            <p>No tienes tarjetas registradas</p>
            <button
              onClick={() => {
                setEditCard(undefined)
                setFormOpen(true)
              }}
              className="mt-1 text-sm text-primary hover:underline"
            >
              Agregar primera tarjeta
            </button>
          </CardContent>
        </CozyCard>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {cards.map((card, i) => {
            const { hasCreditLimit, utilization, utilizationColor, isLastCard } = getCardUtilization(
              card,
              cards.length
            )

            return (
              <CozyCard
                key={card.id}
                className="animate-card-in"
                style={{ animationDelay: `${Math.min(i * 40, 320)}ms`, borderTop: `4px solid ${card.color}` }}
              >
                <CardHeader className="flex flex-row items-start justify-between pb-2">
                  <div>
                    <CardTitle className="text-sm font-medium">{card.name}</CardTitle>
                    <p className="text-xs text-muted-foreground">
                      {card.bank ? `${card.bank}` : ''}
                      {card.bank && card.last4 ? ` · ` : ''}
                      {card.last4 ? `${card.last4}` : ''}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Editar tarjeta ${card.name}`}
                      onClick={() => {
                        setEditCard(card)
                        setFormOpen(true)
                      }}
                    >
                      <Pencil size={14} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      disabled={isLastCard}
                      aria-label={
                        isLastCard
                          ? `No podés archivar tu última tarjeta activa`
                          : `Eliminar tarjeta ${card.name}`
                      }
                      title={
                        isLastCard ? 'No podés archivar tu última tarjeta activa' : undefined
                      }
                      onClick={() => !isLastCard && handleDelete(card)}
                    >
                      <Trash2 size={14} />
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="space-y-2">
                  <div className="flex items-end justify-between">
                    {hasCreditLimit ? (
                      <div className="flex flex-col gap-1">
                        <p className="text-sm text-muted-foreground">
                          Usado:{' '}
                          <span className="font-medium text-foreground">
                            {formatPEN(card.usedCreditCents)}
                          </span>
                        </p>
                        <p className="text-2xl font-bold">
                          {formatPEN(card.creditLimitCents - card.usedCreditCents)}
                          <span className="ml-1 text-xs font-normal text-muted-foreground">
                            disponible
                          </span>
                        </p>
                      </div>
                    ) : (
                      <p className="text-2xl font-bold">{formatPEN(card.balanceCents)}</p>
                    )}
                    <div className="flex gap-1">
                      {!hasCreditLimit && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-xs"
                          onClick={() => {
                            setSelectedCard(card)
                            setTransferOpen(true)
                          }}
                        >
                          <ArrowLeftRight className="mr-1 h-3 w-3" />
                          Transferir
                        </Button>
                      )}
                    </div>
                  </div>
                  {hasCreditLimit && card.creditLimitCents > 0 && (
                    <div className="space-y-1">
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span>Límite: {formatPEN(card.creditLimitCents)}</span>
                        <span>{Math.round(utilization * 100)}% usado</span>
                      </div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                        <div
                          className={`h-full rounded-full transition-all ${utilizationColor}`}
                          style={{ width: `${Math.min(utilization * 100, 100)}%` }}
                        />
                      </div>
                    </div>
                  )}
                </CardContent>
              </CozyCard>
            )
          })}
        </div>
      )}

      <CardForm open={formOpen} onClose={() => setFormOpen(false)} onSaved={invalidateCards} editCard={editCard} />
      {selectedCard && (
        <TransferForm
          open={transferOpen}
          onClose={() => setTransferOpen(false)}
          onSaved={invalidateAfterTransfer}
          fromCard={selectedCard}
          cards={cards}
        />
      )}
    </div>
  )
}
