import { useSearchParams } from 'react-router-dom'
import { Download, Loader2, Heart } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { usePublicYtdlpApi } from '@/hooks/usePublicYtdlpApi'
import { LOVE_MESSAGES } from './loveMessages'
import { useYtDlpDownload } from './useYtDlpDownload'

export default function PublicYtDlpPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const { downloadMp3 } = usePublicYtdlpApi()
  const { url, setUrl, loading, error, messageIndex, handleSubmit } = useYtDlpDownload({
    download: (url) => downloadMp3(url, token),
  })

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-lg">
        <CozyCard className="animate-card-in">
          <CardHeader className="flex flex-row items-center gap-3 pb-2">
            <span className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
              <Heart size={18} strokeWidth={2.25} />
            </span>
            <CardTitle className="text-base font-semibold">YouTube a MP3</CardTitle>
          </CardHeader>
          <CardContent>
            {!token ? (
              <p className="text-sm text-destructive">
                Este link está incompleto. Pedí el link completo de nuevo.
              </p>
            ) : (
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
            )}
          </CardContent>
        </CozyCard>
      </div>
    </div>
  )
}
