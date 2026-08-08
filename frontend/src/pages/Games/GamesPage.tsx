import { Gamepad2, HelpCircle, Film } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import MobileTabNav from '@/components/MobileTabNav'
import { useActiveTab } from '@/hooks/useActiveTab'
import EmojiMoviesTab from './EmojiMoviesTab'
import UltimaPreguntaTab from './UltimaPreguntaTab'

const TABS = [
  { value: 'ultima-pregunta', label: 'La Última Pregunta', icon: HelpCircle },
  { value: 'emoji-movies', label: 'Emoji Movies', icon: Film },
]

export default function GamesPage() {
  const { tab, setSearchParams } = useActiveTab(TABS, 'ultima-pregunta')
  const goToTab = (value: string) => setSearchParams({ tab: value }, { replace: true })

  return (
    <div className="space-y-4 pb-24 sm:pb-0">
      <div className="flex items-center gap-2">
        <Gamepad2 className="h-6 w-6 text-muted-foreground" />
        <h1 className="text-2xl font-bold text-foreground">Juegos</h1>
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
