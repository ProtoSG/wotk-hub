import { LogOut, Menu, Moon, Sun } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useThemeStore } from '@/store/themeStore'
import { useAuthStore } from '@/store/authStore'
import { useAuthApi } from '@/hooks/useAuthApi'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'

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
        <Menu className="h-[18px] w-[18px]" />
      </Button>

      <div className="flex flex-1 items-center justify-between">
        <div />
        <div className="flex items-center gap-3">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                className="relative h-8 w-8 rounded-full bg-primary text-primary-foreground hover:bg-primary/90"
                aria-label="Menú de usuario"
              >
                {user?.name?.[0]?.toUpperCase() ?? 'U'}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel className="font-normal">
                <div className="flex flex-col space-y-1">
                  <p className="text-sm font-medium leading-none">{user?.name}</p>
                  <p className="text-xs leading-none text-muted-foreground">{user?.email}</p>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={toggleTheme} className="cursor-pointer">
                {theme === 'dark' ? (
                  <Sun className="mr-2 h-3.5 w-3.5" />
                ) : (
                  <Moon className="mr-2 h-3.5 w-3.5" />
                )}
                {theme === 'dark' ? 'Modo claro' : 'Modo oscuro'}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={handleLogout} className="cursor-pointer text-destructive focus:text-destructive">
                <LogOut className="mr-2 h-3.5 w-3.5" />
                Cerrar sesión
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  )
}
