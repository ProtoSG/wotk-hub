import { Download, Loader2, Music } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { IconChip } from '@/components/ui/icon-chip'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useYtdlpApi } from '@/hooks/useYtdlpApi'
import { LOVE_MESSAGES } from './loveMessages'
import { useYtDlpDownload } from './useYtDlpDownload'

export default function YtDlpPage() {
  const { downloadMp3 } = useYtdlpApi()
  const { url, setUrl, loading, error, messageIndex, handleSubmit } = useYtDlpDownload({
    download: downloadMp3,
  })

  return (
    <div className="space-y-6 pb-24 sm:pb-0">
      <h1 className="text-2xl font-bold">YouTube a MP3</h1>

      <div className="mx-auto w-full max-w-lg">
        <CozyCard className="animate-card-in">
          <CardHeader className="flex flex-row items-center gap-3 pb-2">
            <IconChip icon={Music} accent="--primary" size="sm" />
            <CardTitle className="text-base font-semibold">Descargar audio</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="ytdlp-url">Link de YouTube</Label>
                <Input
                  id="ytdlp-url"
                  type="url"
                  placeholder="https://www.youtube.com/watch?v=..."
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  disabled={loading}
                  required
                />
              </div>

              {error && <p className="text-sm text-destructive">{error}</p>}

              <Button type="submit" disabled={loading || !url.trim()} className="w-full">
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Descargando...
                  </>
                ) : (
                  <>
                    <Download className="h-4 w-4" />
                    Descargar MP3
                  </>
                )}
              </Button>

              {loading && (
                <p className="text-center text-sm italic text-muted-foreground">
                  {LOVE_MESSAGES[messageIndex]}
                </p>
              )}
            </form>
          </CardContent>
        </CozyCard>
      </div>
    </div>
  )
}
