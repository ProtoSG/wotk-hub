import { Moon, Sun, Trash2, ShieldOff } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { CozyCard } from '@/components/ui/cozy-card'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useThemeStore } from '@/store/themeStore'
import { useDbStore } from '@/store/dbStore'
import { useAuthStore } from '@/store/authStore'
import { useAuthApi } from '@/hooks/useAuthApi'
import type { SavedConnection } from '@/types/db.types'

export default function ConfigurationPage() {
  const { theme, toggleTheme } = useThemeStore()
  const { connections, activeConnectionId, addConnection, removeConnection, setActiveConnection } = useDbStore()
  const setUser = useAuthStore((s) => s.setUser)
  const { logoutAll } = useAuthApi()
  const navigate = useNavigate()

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

  return (
    <div className="max-w-2xl space-y-6">
      <h1 className="text-2xl font-bold">Configuración</h1>

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

      <CozyCard className="animate-card-in [animation-delay:120ms]">
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

      <CozyCard className="animate-card-in [animation-delay:180ms]">
        <CardHeader>
          <CardTitle>Acerca de</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          <Separator />
          <p className="pt-2">Work Hub — nuestro espacio compartido</p>
          <p>Versión 0.1.0</p>
        </CardContent>
      </CozyCard>
    </div>
  )
}
