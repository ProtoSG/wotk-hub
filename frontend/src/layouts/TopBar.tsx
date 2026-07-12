import { Moon, Sun, Menu, LogOut } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useThemeStore } from '@/store/themeStore'
import { useAuthStore } from '@/store/authStore'
import { useAuthApi } from '@/hooks/useAuthApi'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetTrigger } from '@/components/ui/sheet'
import Sidebar from './Sidebar'

export default function TopBar() {
  const { theme, toggleTheme } = useThemeStore()
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const { logout } = useAuthApi()
  const navigate = useNavigate()

  async function handleLogout() {
    try {
      await logout()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'No se pudo cerrar sesión')
    } finally {
      setUser(null)
      navigate('/login', { replace: true })
    }
  }

  return (
    <header className="flex h-14 items-center border-b bg-card px-4 gap-3">
      <Sheet>
        <SheetTrigger asChild>
          <Button variant="ghost" size="icon" className="lg:hidden" aria-label="Abrir menú de navegación">
            <Menu size={18} />
          </Button>
        </SheetTrigger>
        <SheetContent side="left" className="w-56 p-0">
          <Sidebar />
        </SheetContent>
      </Sheet>

      <div className="flex flex-1 items-center justify-end gap-3">
        {user && <span className="text-sm text-muted-foreground">{user.name}</span>}
        <Button variant="ghost" size="icon" onClick={handleLogout} aria-label="Cerrar sesión">
          <LogOut size={16} />
        </Button>
      </div>

      <Button variant="ghost" size="icon" onClick={toggleTheme} aria-label="Cambiar tema">
        {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
      </Button>
    </header>
  )
}
