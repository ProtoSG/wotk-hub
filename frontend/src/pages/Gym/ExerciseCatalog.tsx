import { useState } from 'react'
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { Dumbbell, Search, SlidersHorizontal, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useGymApi } from '@/hooks/useGymApi'
import type { Exercise } from '@/types/gym.types'
import { exerciseFiltersKey, exercisesKey } from './gymKeys'
import { useDebouncedValue } from './useDebouncedValue'
import ExerciseRow from './ExerciseRow'

const PAGE_SIZE = 50

/** Sentinel for "no filter" — Radix Select can't hold an empty-string value. */
const ALL = 'all'

interface ExerciseCatalogProps {
  /** Rendered when a row is picked. Omit to make the list read-only. */
  onSelect?: (exercise: Exercise) => void
}

export default function ExerciseCatalog({ onSelect }: ExerciseCatalogProps) {
  const [search, setSearch] = useState('')
  const [muscle, setMuscle] = useState(ALL)
  const [equipment, setEquipment] = useState(ALL)
  const [page, setPage] = useState(0)
  const debouncedSearch = useDebouncedValue(search)
  const { listExercises, listExerciseFilters } = useGymApi()

  // Every filter change resets the page: staying on page 4 of a result set
  // that now has one page would show an empty list. Done in the handlers
  // rather than an effect so there is no cascading render.
  const changeSearch = (value: string) => {
    setSearch(value)
    setPage(0)
  }

  const changeMuscle = (value: string) => {
    setMuscle(value)
    setPage(0)
  }

  const changeEquipment = (value: string) => {
    setEquipment(value)
    setPage(0)
  }

  const { data: filterValues } = useQuery({
    queryKey: exerciseFiltersKey(),
    queryFn: () => listExerciseFilters(),
    // The catalog only changes when a custom exercise is added, so these
    // dropdown values are worth holding on to for the session.
    staleTime: 5 * 60_000,
  })

  const { data, isPending } = useQuery({
    queryKey: exercisesKey(debouncedSearch, muscle, equipment, page),
    queryFn: () =>
      listExercises({
        q: debouncedSearch || undefined,
        muscle: muscle === ALL ? undefined : muscle,
        equipment: equipment === ALL ? undefined : equipment,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
    // Keeps the previous page on screen while the next one loads, instead of
    // collapsing the list to skeletons on every page step.
    placeholderData: keepPreviousData,
  })

  const exercises = data?.exercises ?? []
  const total = data?.total ?? 0
  const hasFilters = Boolean(search) || muscle !== ALL || equipment !== ALL
  const lastPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1)

  const clearFilters = () => {
    setSearch('')
    setMuscle(ALL)
    setEquipment(ALL)
    setPage(0)
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => changeSearch(e.target.value)}
            placeholder="Buscar ejercicio"
            aria-label="Buscar ejercicio"
            className="pl-9"
          />
        </div>

        <div className="flex gap-3">
          <Select value={muscle} onValueChange={changeMuscle}>
            <SelectTrigger className="flex-1 sm:w-44" aria-label="Filtrar por músculo">
              <SelectValue placeholder="Músculo" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Todos los músculos</SelectItem>
              {filterValues?.muscles.map((m) => (
                <SelectItem key={m} value={m}>
                  {m}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select value={equipment} onValueChange={changeEquipment}>
            <SelectTrigger className="flex-1 sm:w-44" aria-label="Filtrar por equipo">
              <SelectValue placeholder="Equipo" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Todo el equipo</SelectItem>
              {filterValues?.equipment.map((eq) => (
                <SelectItem key={eq} value={eq}>
                  {eq}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="flex items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {isPending ? 'Cargando…' : `${total} ${total === 1 ? 'ejercicio' : 'ejercicios'}`}
        </p>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <X className="mr-1 h-4 w-4" />
            Limpiar filtros
          </Button>
        )}
      </div>

      {isPending ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      ) : exercises.length === 0 ? (
        <EmptyState
          icon={hasFilters ? <SlidersHorizontal /> : <Dumbbell />}
          title={hasFilters ? 'Ningún ejercicio coincide' : 'No hay ejercicios'}
          description={
            hasFilters
              ? 'Probá con otro término o quitá los filtros.'
              : 'El catálogo se carga al iniciar el servidor.'
          }
          action={hasFilters ? { label: 'Limpiar filtros', onClick: clearFilters } : undefined}
        />
      ) : (
        <ul className="space-y-2">
          {exercises.map((exercise) => (
            <ExerciseRow key={exercise.id} exercise={exercise} onSelect={onSelect} />
          ))}
        </ul>
      )}

      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between gap-3">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 0}
            onClick={() => setPage((p) => Math.max(0, p - 1))}
          >
            Anterior
          </Button>
          <span className="text-sm text-muted-foreground">
            Página {page + 1} de {lastPage + 1}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= lastPage}
            onClick={() => setPage((p) => Math.min(lastPage, p + 1))}
          >
            Siguiente
          </Button>
        </div>
      )}
    </div>
  )
}
