import { useState } from 'react'
import { Dumbbell, History, ListChecks } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { FloatingActionButton } from '@/components/ui/floating-action-button'
import MobileTabNav from '@/components/MobileTabNav'
import { useActiveTab } from '@/hooks/useActiveTab'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import EntrenarTab from './EntrenarTab'
import HistorialTab from './HistorialTab'
import ExerciseCatalog from './ExerciseCatalog'
import SessionDetailDialog from './SessionDetailDialog'

const TABS = [
  { value: 'entrenar',  label: 'Entrenar',  icon: Dumbbell },
  { value: 'historial', label: 'Historial', icon: History },
  { value: 'ejercicios', label: 'Ejercicios', icon: ListChecks },
]

const TAB_CONTENT_CLASS =
  'mt-4 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95'

export default function GymPage() {
  const { tab, setSearchParams } = useActiveTab(TABS, 'entrenar')
  const [pickerOpen, setPickerOpen] = useState(false)
  const [openSessionId, setOpenSessionId] = useState<number | null>(null)

  return (
    <>
      <div className="space-y-6 pb-24 sm:pb-0">
        <h1 className="text-xl font-bold sm:text-2xl">Gimnasio</h1>

        <Tabs value={tab} onValueChange={(v) => setSearchParams({ tab: v }, { replace: true })}>
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

          <TabsContent value="historial" className={TAB_CONTENT_CLASS}>
            <HistorialTab onOpen={(session) => setOpenSessionId(session.id)} />
          </TabsContent>

          <TabsContent value="ejercicios" className={TAB_CONTENT_CLASS}>
            <CozyCard>
              <CardHeader>
                <CardTitle>Catálogo</CardTitle>
              </CardHeader>
              <CardContent>
                <ExerciseCatalog />
              </CardContent>
            </CozyCard>
          </TabsContent>
        </Tabs>
      </div>

      {/* The FAB only makes sense while logging — it adds an exercise to the
          session in progress, which is the one action worth a thumb-reach
          target on mobile. */}
      {tab === 'entrenar' && (
        <FloatingActionButton
          label="Agregar ejercicio"
          onClick={() => setPickerOpen(true)}
          className="bottom-[max(env(safe-area-inset-bottom),1rem)]"
        />
      )}

      <MobileTabNav
        tabs={TABS}
        activeTab={tab}
        onChange={(value) => setSearchParams({ tab: value }, { replace: true })}
        fabVisible={tab === 'entrenar'}
      />

      <SessionDetailDialog sessionId={openSessionId} onClose={() => setOpenSessionId(null)} />
    </>
  )
}
