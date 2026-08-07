import { Gamepad2 } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import EmojiMoviesTab from './EmojiMoviesTab'
import UltimaPreguntaTab from './UltimaPreguntaTab'

export default function GamesPage() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Gamepad2 className="h-6 w-6 text-muted-foreground" />
        <h1 className="text-2xl font-bold text-foreground">Juegos</h1>
      </div>

      <Tabs defaultValue="ultima-pregunta">
        <TabsList>
          <TabsTrigger value="ultima-pregunta">La Última Pregunta</TabsTrigger>
          <TabsTrigger value="emoji-movies">Emoji Movies</TabsTrigger>
        </TabsList>
        <TabsContent value="ultima-pregunta" className="mt-4">
          <UltimaPreguntaTab />
        </TabsContent>
        <TabsContent value="emoji-movies" className="mt-4">
          <EmojiMoviesTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
