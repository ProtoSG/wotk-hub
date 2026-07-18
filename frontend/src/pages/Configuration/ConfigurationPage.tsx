import { useState } from 'react'
import { Moon, Sun, Trash2, ShieldOff, Plus, Copy, Loader2, Palette, Database, ShieldCheck } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { useThemeStore } from '@/store/themeStore'
import { useDbStore } from '@/store/dbStore'
import { useAuthStore } from '@/store/authStore'
import { useAuthApi } from '@/hooks/useAuthApi'
import { useApiKeys, useCreateApiKey, useRevokeApiKey, type CreatedApiKey } from '@/hooks/useApiKeys'
import type { SavedConnection } from '@/types/db.types'
import type { ApiKey } from '@/types/auth.types'

const TABS = [
  { value: 'general', label: 'General', icon: Palette },
  { value: 'conexiones', label: 'Conexiones', icon: Database },
  { value: 'seguridad', label: 'Seguridad y accesos', icon: ShieldCheck },
]

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString('es-PE')
}

// Falls back to the first tab when the URL is missing ?tab= or carries a
// value that no longer matches a tab (e.g. an old bookmark).
function resolveTab(searchParams: URLSearchParams): string {
  const param = searchParams.get('tab') ?? ''
  return TABS.some((t) => t.value === param) ? param : TABS[0].value
}

export default function ConfigurationPage() {
  const { theme, toggleTheme } = useThemeStore()
  const { connections, activeConnectionId, addConnection, removeConnection, setActiveConnection } = useDbStore()
  const setUser = useAuthStore((s) => s.setUser)
  const { logoutAll } = useAuthApi()
  const navigate = useNavigate()

  const [searchParams, setSearchParams] = useSearchParams()
  const tab = resolveTab(searchParams)

  const { keys, isLoading: isLoadingKeys, refetch: refetchKeys } = useApiKeys()
  const createApiKey = useCreateApiKey()
  const revokeApiKey = useRevokeApiKey()

  const [createOpen, setCreateOpen] = useState(false)
  const [creatingKey, setCreatingKey] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [revealedKey, setRevealedKey] = useState<CreatedApiKey | null>(null)
  const [keyToRevoke, setKeyToRevoke] = useState<ApiKey | null>(null)
  const [revokingKey, setRevokingKey] = useState(false)

  function handleDeleteConnection(conn: SavedConnection) {
    const wasActive = activeConnectionId === conn.id
    removeConnection(conn.id)
    toast.success(`Conexión "${conn.name}" eliminada`, {
      duration: 4500,
      action: {
        label: 'Deshacer',
        onClick: () => {
          addConnection(conn)
          if (wasActive) setActiveConnection(conn.id)
        },
      },
    })
  }

  async function handleLogoutAll() {
    try {
      await logoutAll()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo cerrar la sesión en todos los dispositivos')
      return
    }
    setUser(null)
    navigate('/login', { replace: true })
  }

  async function handleCreateKey() {
    setCreatingKey(true)
    try {
      const created = await createApiKey(newKeyName.trim())
      setCreateOpen(false)
      setNewKeyName('')
      setRevealedKey(created)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo generar la API key')
    } finally {
      setCreatingKey(false)
    }
  }

  function handleCloseRevealedKey() {
    setRevealedKey(null)
    refetchKeys()
  }

  async function handleCopyKey() {
    if (!revealedKey) return
    await navigator.clipboard.writeText(revealedKey.key)
    toast.success('Key copiada al portapapeles')
  }

  async function handleConfirmRevokeKey() {
    if (!keyToRevoke) return
    setRevokingKey(true)
    try {
      await revokeApiKey(keyToRevoke.id)
      toast.success('API key revocada')
      setKeyToRevoke(null)
      refetchKeys()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo revocar la API key')
    } finally {
      setRevokingKey(false)
    }
  }

  return (
    <>
      <div className="max-w-2xl space-y-6 pb-24 sm:pb-0">
        <h1 className="text-2xl font-bold">Configuración</h1>

        <Tabs value={tab} onValueChange={(v) => setSearchParams({ tab: v }, { replace: true })}>
          <TabsList className="hidden sm:inline-flex">
            {TABS.map((t) => (
              <TabsTrigger key={t.value} value={t.value}>
                {t.label}
              </TabsTrigger>
            ))}
          </TabsList>

          <TabsContent
            value="general"
            className="space-y-6 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95"
          >
            <CozyCard className="animate-card-in">
              <CardHeader>
                <CardTitle>Apariencia</CardTitle>
                <CardDescription>Personalizá el aspecto de Work Hub</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {theme === 'dark' ? <Moon size={16} /> : <Sun size={16} />}
                    <Label>Modo oscuro</Label>
                  </div>
                  <Switch checked={theme === 'dark'} onCheckedChange={toggleTheme} />
                </div>
              </CardContent>
            </CozyCard>

            <CozyCard className="animate-card-in [animation-delay:60ms]">
              <CardHeader>
                <CardTitle>Acerca de</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-sm text-muted-foreground">
                <Separator />
                <p className="pt-2">Work Hub — nuestro espacio compartido</p>
                <p>Versión 0.1.0</p>
              </CardContent>
            </CozyCard>
          </TabsContent>

          <TabsContent
            value="conexiones"
            className="data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95"
          >
            <CozyCard className="animate-card-in">
              <CardHeader>
                <CardTitle>Conexiones guardadas</CardTitle>
                <CardDescription>Administrá tus conexiones a bases de datos</CardDescription>
              </CardHeader>
              <CardContent>
                {connections.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    Todavía no hay conexiones guardadas. Agregá una desde el DB Manager.
                  </p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Host</TableHead>
                        <TableHead>Dialecto</TableHead>
                        <TableHead className="w-12" />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {connections.map((conn) => (
                        <TableRow key={conn.id}>
                          <TableCell className="font-medium">{conn.name}</TableCell>
                          <TableCell className="text-muted-foreground text-sm">
                            {conn.host}:{conn.port}/{conn.database}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline" className="text-xs">
                              {conn.dialect}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="text-destructive hover:text-destructive"
                              onClick={() => handleDeleteConnection(conn)}
                              aria-label={`Eliminar conexión ${conn.name}`}
                            >
                              <Trash2 size={14} />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </CozyCard>
          </TabsContent>

          <TabsContent
            value="seguridad"
            className="space-y-6 data-[state=active]:animate-in data-[state=active]:fade-in data-[state=active]:zoom-in-95"
          >
            <CozyCard className="animate-card-in">
              <CardHeader>
                <CardTitle>Seguridad</CardTitle>
                <CardDescription>Cerrá la sesión en todos los dispositivos donde iniciaste sesión</CardDescription>
              </CardHeader>
              <CardContent>
                <Button variant="outline" className="text-destructive hover:text-destructive" onClick={handleLogoutAll}>
                  <ShieldOff size={16} />
                  Cerrar sesión en todos los dispositivos
                </Button>
              </CardContent>
            </CozyCard>

            <CozyCard className="animate-card-in [animation-delay:60ms]">
              <CardHeader>
                <CardTitle>API Keys</CardTitle>
                <CardDescription>
                  Generá una key para automatizaciones externas (como un Shortcut de iOS/macOS) que autentican con{' '}
                  <code className="text-xs">Authorization: Bearer &lt;key&gt;</code> y no pueden iniciar sesión normalmente
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <Button variant="outline" onClick={() => setCreateOpen(true)}>
                  <Plus size={16} />
                  Generar nueva key
                </Button>

                {isLoadingKeys ? (
                  <p className="text-sm text-muted-foreground">Cargando…</p>
                ) : keys.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Todavía no generaste ninguna API key.</p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Nombre</TableHead>
                        <TableHead>Creada</TableHead>
                        <TableHead>Último uso</TableHead>
                        <TableHead>Estado</TableHead>
                        <TableHead className="w-12" />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {keys.map((k) => (
                        <TableRow key={k.id}>
                          <TableCell className="font-medium">{k.name || '(sin nombre)'}</TableCell>
                          <TableCell className="text-muted-foreground text-sm">{formatDate(k.created_at)}</TableCell>
                          <TableCell className="text-muted-foreground text-sm">
                            {k.last_used_at ? formatDate(k.last_used_at) : 'Nunca'}
                          </TableCell>
                          <TableCell>
                            <Badge variant={k.revoked_at ? 'destructive' : 'outline'} className="text-xs">
                              {k.revoked_at ? 'Revocada' : 'Activa'}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {!k.revoked_at && (
                              <Button
                                variant="ghost"
                                size="icon"
                                className="text-destructive hover:text-destructive"
                                onClick={() => setKeyToRevoke(k)}
                                aria-label={`Revocar API key ${k.name || '(sin nombre)'}`}
                              >
                                <Trash2 size={14} />
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </CozyCard>
          </TabsContent>
        </Tabs>
      </div>

      <nav
        className="fixed left-4 right-4 z-40 flex h-14 items-center justify-around gap-0.5 rounded-full border bg-background px-2 shadow-lg sm:hidden"
        style={{ bottom: 'max(env(safe-area-inset-bottom), 1rem)' }}
      >
        {TABS.map((t) => {
          const Icon = t.icon
          const active = tab === t.value
          return (
            <button
              key={t.value}
              type="button"
              onClick={() => setSearchParams({ tab: t.value }, { replace: true })}
              aria-label={t.label}
              className={cn(
                'flex h-11 flex-1 items-center justify-center rounded-full',
                active ? 'bg-muted text-foreground' : 'text-muted-foreground'
              )}
            >
              <Icon className="h-5 w-5" />
            </button>
          )
        })}
      </nav>

      <Dialog
        open={createOpen}
        onOpenChange={(v) => {
          if (v) return
          setCreateOpen(false)
          setNewKeyName('')
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Nueva API key</DialogTitle>
          </DialogHeader>
          <div className="space-y-1">
            <Label>Nombre (opcional)</Label>
            <Input value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder="Ej: Shortcut iOS" />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setCreateOpen(false)
                setNewKeyName('')
              }}
            >
              Cancelar
            </Button>
            <Button type="button" disabled={creatingKey} onClick={handleCreateKey}>
              {creatingKey && <Loader2 size={14} className="animate-spin" />}
              {creatingKey ? 'Generando…' : 'Generar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!revealedKey} onOpenChange={(v) => !v && handleCloseRevealedKey()}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>API key generada</DialogTitle>
            <DialogDescription>Copiala ahora: por seguridad, no vamos a volver a mostrarla.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Input readOnly value={revealedKey?.key ?? ''} onFocus={(e) => e.target.select()} className="font-mono text-xs" />
            <Button type="button" variant="outline" onClick={handleCopyKey}>
              <Copy size={14} />
              Copiar
            </Button>
            <p className="text-xs text-destructive">
              Esta es la única vez que vas a ver esta key. Guardala en un lugar seguro: si la perdés, vas a tener que generar
              una nueva.
            </p>
          </div>
          <DialogFooter>
            <Button type="button" onClick={handleCloseRevealedKey}>
              Listo
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!keyToRevoke} onOpenChange={(v) => !v && setKeyToRevoke(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Revocar API key</DialogTitle>
            <DialogDescription>
              ¿Seguro que querés revocar &quot;{keyToRevoke?.name || '(sin nombre)'}&quot;? Cualquier automatización que la
              use va a dejar de funcionar de inmediato. Esta acción no se puede deshacer.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setKeyToRevoke(null)}>
              Cancelar
            </Button>
            <Button type="button" variant="destructive" disabled={revokingKey} onClick={handleConfirmRevokeKey}>
              {revokingKey && <Loader2 size={14} className="animate-spin" />}
              {revokingKey ? 'Revocando…' : 'Revocar'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
