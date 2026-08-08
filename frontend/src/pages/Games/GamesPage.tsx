import { Gamepad2, HelpCircle, Film, Bell, BellOff } from 'lucide-react'
import { toast } from 'sonner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import MobileTabNav from '@/components/MobileTabNav'
import { useActiveTab } from '@/hooks/useActiveTab'
import { usePushNotifications } from '@/hooks/usePushNotifications'
import EmojiMoviesTab from './EmojiMoviesTab'
import UltimaPreguntaTab from './UltimaPreguntaTab'

const TABS = [
  { value: 'ultima-pregunta', label: 'La Última Pregunta', icon: HelpCircle },
  { value: 'emoji-movies', label: 'Emoji Movies', icon: Film },
]

export default function GamesPage() {
  const { tab, setSearchParams } = useActiveTab(TABS, 'ultima-pregunta')
  const goToTab = (value: string) => setSearchParams({ tab: value }, { replace: true })
  const { supported, subscribed, busy, toggle } = usePushNotifications()

  async function handleToggleReminders() {
    try {
      await toggle()
      toast.success(subscribed ? 'Recordatorios desactivados' : 'Te avisamos al mediodía si falta resolver')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo activar el recordatorio')
    }
  }

  return (
    <div className="space-y-4 pb-24 sm:pb-0">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Gamepad2 className="h-6 w-6 text-muted-foreground" />
          <h1 className="text-2xl font-bold text-foreground">Juegos</h1>
        </div>
        {supported && (
          <Button
            variant="ghost"
            size="icon"
            onClick={handleToggleReminders}
            disabled={busy}
            aria-label={subscribed ? 'Desactivar recordatorios' : 'Activar recordatorios'}
            title={subscribed ? 'Recordatorios activados — te avisamos al mediodía' : 'Avisarme si no resolví la pregunta del día'}
          >
            {subscribed ? <Bell className="h-5 w-5 text-primary" /> : <BellOff className="h-5 w-5 text-muted-foreground" />}
          </Button>
        )}
      </div>

      <Tabs value={tab} onValueChange={goToTab}>
        <TabsList className="hidden sm:inline-flex">
          {TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="ultima-pregunta" className="mt-4">
          <UltimaPreguntaTab />
        </TabsContent>
        <TabsContent value="emoji-movies" className="mt-4">
          <EmojiMoviesTab />
        </TabsContent>
      </Tabs>

      <MobileTabNav tabs={TABS} activeTab={tab} onChange={goToTab} fabVisible={false} />
    </div>
  )
}
