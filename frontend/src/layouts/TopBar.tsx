import { Moon, Sun, Menu, LogOut } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useThemeStore } from '@/store/themeStore'
import { useAuthStore } from '@/store/authStore'
import { useAuthApi } from '@/hooks/useAuthApi'
import { Button } from '@/components/ui/button'

interface TopBarProps {
  onMenuClick: () => void
}

export default function TopBar({ onMenuClick }: TopBarProps) {
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
      <Button
        variant="ghost"
        size="icon"
        className="lg:hidden"
        aria-label="Abrir menú de navegación"
        onClick={onMenuClick}
      >
        <Menu size={18} />
      </Button>

      <div className="flex flex-1 items-center justify-between">
        <div />
        <div className="flex items-center gap-3">
          {user && <span className="text-sm text-muted-foreground">{user.name}</span>}
          <Button variant="ghost" size="icon" onClick={toggleTheme} aria-label="Cambiar tema">
            {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
          </Button>
          <Button variant="ghost" size="icon" onClick={handleLogout} aria-label="Cerrar sesión">
            <LogOut size={16} />
          </Button>
        </div>
      </div>
    </header>
  )
}
