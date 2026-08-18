import { ChevronDown, ChevronUp, Heart, Send, Trash2 } from 'lucide-react'
import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CardContent } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Textarea } from '@/components/ui/textarea'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useCoupleApi } from '@/hooks/useCoupleApi'
import type { Poem } from '@/types/couple.types'
import { poemsKey } from './coupleKeys'

const MAX_CHARS = 1000
const UNDO_WINDOW_MS = 4500

interface PoemasTabProps {
  canManage: boolean
}

export default function PoemasTab({ canManage }: PoemasTabProps) {
  const { listPoems, createPoem, deletePoem } = useCoupleApi()
  const queryClient = useQueryClient()

  const [composing, setComposing] = useState('')
  const [myPoemsOpen, setMyPoemsOpen] = useState(false)
  const pendingDeletes = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const { data: poems = [], isPending } = useQuery({
    queryKey: poemsKey(),
    queryFn: () => listPoems(),
  })

  const createMutation = useMutation({
    mutationFn: (content: string) => createPoem({ content }),
    onSuccess: (newPoem) => {
      queryClient.setQueryData<Poem[]>(poemsKey(), (prev = []) => [...prev, newPoem])
      setComposing('')
      toast.success('¡Poema enviado!')
      setMyPoemsOpen(true)
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo enviar el poema')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deletePoem(id),
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el poema')
      queryClient.invalidateQueries({ queryKey: poemsKey() })
    },
  })

  const receivedPoems = poems.filter((p) => !p.isMine)
  const myPoems = poems.filter((p) => p.isMine)
  const unseenCount = receivedPoems.filter((p) => !p.isSeen).length

  function handleSend() {
    const content = composing.trim()
    if (!content || createMutation.isPending) return
    createMutation.mutate(content)
  }

  function handleDelete(poem: Poem) {
    let removedIndex = -1
    queryClient.setQueryData<Poem[]>(poemsKey(), (prev = []) => {
      removedIndex = prev.findIndex((x) => x.id === poem.id)
      return prev.filter((x) => x.id !== poem.id)
    })

    const timer = setTimeout(() => {
      pendingDeletes.current.delete(poem.id)
      deleteMutation.mutate(poem.id)
    }, UNDO_WINDOW_MS)
    pendingDeletes.current.set(poem.id, timer)

    toast.success('Poema eliminado', {
      duration: UNDO_WINDOW_MS,
      action: {
        label: 'Deshacer',
        onClick: () => {
          const timerId = pendingDeletes.current.get(poem.id)
          if (timerId !== undefined) {
            clearTimeout(timerId)
            pendingDeletes.current.delete(poem.id)
          }
          queryClient.setQueryData<Poem[]>(poemsKey(), (prev = []) => {
            const next = [...prev]
            next.splice(Math.min(removedIndex, next.length), 0, poem)
            return next
          })
        },
      },
    })
  }

  if (isPending) {
    return (
      <div className="flex justify-center py-12">
        <Heart className="h-6 w-6 animate-pulse text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6">
      {/* Reader: poems received from partner */}
      {receivedPoems.length === 0 ? (
        <CozyCard className="animate-card-in">
          <CardContent className="flex flex-col items-center gap-2 py-10 text-center text-muted-foreground">
            <Heart className="h-7 w-7 animate-pulse" />
            <p className="text-sm">Aún no tienes poemas.</p>
            <p className="text-xs">¡Tu pareja te sorprenderá!</p>
          </CardContent>
        </CozyCard>
      ) : (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-muted-foreground">Poemas para ti</h2>
            {unseenCount > 0 && (
              <Badge variant="secondary" className="gap-1">
                <Heart className="h-3 w-3 fill-primary text-primary animate-pulse" />
                {unseenCount} sin leer
              </Badge>
            )}
          </div>
          <div className="space-y-3" role="region" aria-label="Poemas recibidos">
            {receivedPoems.map((poem) => (
              <PoemCard key={poem.id} poem={poem} onDelete={undefined} canManage={false} />
            ))}
          </div>
        </div>
      )}

      {/* Writer: compose area — gated to canManage */}
      {canManage && (
        <CozyCard className="animate-card-in">
          <CardContent className="space-y-4 p-5">
            <p className="text-xs text-muted-foreground">Escribe algo bonito para tu pareja</p>

            <div className="relative">
              <Textarea
                ref={textareaRef}
                placeholder="Tu poema aquí..."
                value={composing}
                onChange={(e) => setComposing(e.target.value.slice(0, MAX_CHARS))}
                rows={4}
                className="resize-none pr-14 font-serif italic"
                aria-label="Escribe tu poema"
              />
              <span className="absolute bottom-3 right-3 text-xs text-muted-foreground">
                {composing.length}/{MAX_CHARS}
              </span>
            </div>

            <div className="flex justify-end">
              <Button
                onClick={handleSend}
                disabled={!composing.trim() || createMutation.isPending}
                className="gap-2"
              >
                <Send className="h-4 w-4" />
                {createMutation.isPending ? 'Enviando...' : 'Enviar'}
              </Button>
            </div>
          </CardContent>
        </CozyCard>
      )}

      {/* My poems: collapsible list of poems written by the current user */}
      {myPoems.length > 0 && (
        <div className="space-y-2">
          <button
            type="button"
            onClick={() => setMyPoemsOpen((v) => !v)}
            className="flex w-full items-center justify-between rounded-md px-1 py-1.5 text-sm font-semibold text-muted-foreground hover:text-foreground"
          >
            <span>Mis poemas ({myPoems.length})</span>
            {myPoemsOpen ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </button>

          {myPoemsOpen && (
            <div className="space-y-3" role="region" aria-label="Mis poemas">
              {myPoems.map((poem) => (
                <PoemCard
                  key={poem.id}
                  poem={poem}
                  onDelete={canManage ? () => handleDelete(poem) : undefined}
                  canManage={!!canManage}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

interface PoemCardProps {
  poem: Poem
  onDelete?: () => void
  canManage: boolean
}

function PoemCard({ poem, onDelete, canManage }: PoemCardProps) {
  const isUnseen = !poem.isMine && !poem.isSeen

  return (
    <CozyCard className="relative animate-card-in">
      {isUnseen && (
        <div className="absolute -top-2 -right-2">
          <span className="relative flex h-3 w-3">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary opacity-75" />
            <span className="relative inline-flex h-3 w-3 rounded-full bg-primary" />
          </span>
        </div>
      )}
      <CardContent className="p-5">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="mb-2 flex items-center gap-1.5">
              {isUnseen ? (
                <Heart className="h-3.5 w-3.5 fill-primary text-primary" />
              ) : (
                <Heart className="h-3.5 w-3.5 text-muted-foreground" />
              )}
              <span className="text-xs" style={{ color: 'var(--cover-sakura-dark, var(--primary))' }}>
                {poem.isMine ? 'De ti' : 'De tu pareja'}
              </span>
              {!poem.isSeen && poem.isMine === false && (
                <Badge variant="secondary" className="ml-1 text-[10px]">
                  Nuevo
                </Badge>
              )}
            </div>
            <p className="whitespace-pre-wrap font-serif italic leading-relaxed">{poem.content}</p>
          </div>

          {canManage && onDelete && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="Más acciones" className="shrink-0">
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onClick={onDelete}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 className="h-4 w-4" />
                  Eliminar
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </CardContent>
    </CozyCard>
  )
}
