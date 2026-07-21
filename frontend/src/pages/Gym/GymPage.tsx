import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus } from 'lucide-react'
import { useGymApi } from '@/hooks/useGymApi'
import type { Exercise } from '@/types/gym.types'
import { ClipboardList, Dumbbell, History, ListChecks, TrendingUp } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { FloatingActionButton } from '@/components/ui/floating-action-button'
import MobileTabNav from '@/components/MobileTabNav'
import { useActiveTab } from '@/hooks/useActiveTab'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import EntrenarTab from './EntrenarTab'
import RutinasTab from './RutinasTab'
import ProgresoTab from './ProgresoTab'
import HistorialTab from './HistorialTab'
import ExerciseCatalog from './ExerciseCatalog'
import ExerciseDetailDialog from './ExerciseDetailDialog'
import ExerciseForm from './ExerciseForm'
import SessionDetailDialog from './SessionDetailDialog'
import { useStartSession } from './useStartSession'

const TABS = [
  { value: 'entrenar',   label: 'Entrenar',   icon: Dumbbell },
  { value: 'rutinas',    label: 'Rutinas',    icon: ClipboardList },
  { value: 'progreso',   label: 'Progreso',   icon: TrendingUp },
  { value: 'historial',  label: 'Historial',  icon: History },
  { value: 'ejercicios', label: 'Ejercicios', icon: ListChecks },
]

/** Tabs whose FAB creates something; the label doubles as its aria-label. */
const FAB_LABELS: Record<string, string> = {
  entrenar: 'Agregar ejercicio',
  rutinas: 'Nueva rutina',
  ejercicios: 'Nuevo ejercicio',
}

const TAB_CONTENT_CLASS =
  'mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95'

export default function GymPage() {
  const { tab, setSearchParams } = useActiveTab(TABS, 'entrenar')
  const [pickerOpen, setPickerOpen] = useState(false)
  const [routineFormOpen, setRoutineFormOpen] = useState(false)
  const [openSessionId, setOpenSessionId] = useState<number | null>(null)
  // The catalog tab is not inside a dialog, so the detail can be one here.
  const [detailExercise, setDetailExercise] = useState<Exercise | null>(null)
  const [exerciseFormOpen, setExerciseFormOpen] = useState(false)
  const [editingExercise, setEditingExercise] = useState<Exercise | null>(null)
  const queryClient = useQueryClient()
  const { deleteExercise } = useGymApi()

  const removeExercise = useMutation({
    mutationFn: (exercise: Exercise) => deleteExercise(exercise.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gym', 'exercises'] })
      setDetailExercise(null)
      toast.success('Ejercicio eliminado')
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : 'No se pudo eliminar el ejercicio')
    },
  })

  const openExerciseForm = (exercise: Exercise | null) => {
    setEditingExercise(exercise)
    setDetailExercise(null)
    setExerciseFormOpen(true)
  }

  const goToTab = (value: string) => setSearchParams({ tab: value }, { replace: true })

  // Starting from the routine list hands off to the Entrenar tab, which is
  // where the session it just created is logged.
  const start = useStartSession(() => goToTab('entrenar'))

  const fabLabel = FAB_LABELS[tab]

  return (
    <>
      <div className="space-y-6 pb-24 sm:pb-0">
        <h1 className="text-xl font-bold sm:text-2xl">Gimnasio</h1>

        <Tabs value={tab} onValueChange={goToTab}>
          <TabsList className="hidden sm:inline-flex">
            {TABS.map((t) => (
              <TabsTrigger key={t.value} value={t.value}>
                {t.label}
              </TabsTrigger>
            ))}
          </TabsList>

          <TabsContent value="entrenar" className={TAB_CONTENT_CLASS}>
            <EntrenarTab pickerOpen={pickerOpen} onPickerOpenChange={setPickerOpen} />
          </TabsContent>

          <TabsContent value="rutinas" className={TAB_CONTENT_CLASS}>
            <RutinasTab
              formOpen={routineFormOpen}
              onFormOpenChange={setRoutineFormOpen}
              onStart={(routine) => start.mutate(routine.id)}
              starting={start.isPending}
            />
          </TabsContent>

          <TabsContent value="progreso" className={TAB_CONTENT_CLASS}>
            <ProgresoTab />
          </TabsContent>

          <TabsContent value="historial" className={TAB_CONTENT_CLASS}>
            <HistorialTab onOpen={(session) => setOpenSessionId(session.id)} />
          </TabsContent>

          <TabsContent value="ejercicios" className={TAB_CONTENT_CLASS}>
            <CozyCard>
              <CardHeader className="flex-row items-center justify-between gap-3 space-y-0">
                <CardTitle>Catálogo</CardTitle>
                <Button
                  variant="outline"
                  size="sm"
                  className="hidden sm:flex"
                  onClick={() => openExerciseForm(null)}
                >
                  <Plus className="mr-1 h-4 w-4" />
                  Nuevo ejercicio
                </Button>
              </CardHeader>
              <CardContent>
                <ExerciseCatalog onOpen={setDetailExercise} />
              </CardContent>
            </CozyCard>
          </TabsContent>
        </Tabs>
      </div>

      {/* Only the tabs that create something get a FAB: adding an exercise to
          the session in progress, and creating a routine. Browsing tabs have
          no primary action worth a thumb-reach target. */}
      {fabLabel && (
        <FloatingActionButton
          label={fabLabel}
          onClick={() => {
            if (tab === 'rutinas') return setRoutineFormOpen(true)
            if (tab === 'ejercicios') return openExerciseForm(null)
            return setPickerOpen(true)
          }}
          className="bottom-[max(env(safe-area-inset-bottom),1rem)]"
        />
      )}

      <MobileTabNav
        tabs={TABS}
        activeTab={tab}
        onChange={goToTab}
        fabVisible={Boolean(fabLabel)}
      />

      <SessionDetailDialog sessionId={openSessionId} onClose={() => setOpenSessionId(null)} />

      <ExerciseDetailDialog
        exercise={detailExercise}
        onClose={() => setDetailExercise(null)}
        onEdit={openExerciseForm}
        onDelete={(exercise) => removeExercise.mutate(exercise)}
      />

      <ExerciseForm
        open={exerciseFormOpen}
        editing={editingExercise}
        onClose={() => setExerciseFormOpen(false)}
        onSaved={setDetailExercise}
      />
    </>
  )
}
