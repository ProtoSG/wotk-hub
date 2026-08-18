import { Database, Dumbbell, Gamepad2, Heart, LayoutDashboard, Music, Settings, Wallet } from 'lucide-react'
import type { ComponentType } from 'react'
import { NavLink } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { useAuthStore, type Role } from '@/store/authStore'

const navItems: {
  to: string
  label: string
  icon: ComponentType<{ className?: string }>
  roles?: readonly Role[]
  // Admin-grantable module key — see RequireRole's `module` prop for the
  // matching route-level check. Admin bypasses this (never gated by grants).
  module?: string
}[] = [
  { to: '/dashboard', label: 'Inicio', icon: LayoutDashboard, roles: ['admin', 'guest'], module: 'dashboard' },
  { to: '/db-manager', label: 'DB Manager', icon: Database, roles: ['admin'] },
  { to: '/finances', label: 'Finanzas', icon: Wallet, roles: ['admin', 'guest'], module: 'finances' },
  { to: '/citas', label: 'Citas', icon: Heart },
  { to: '/juegos', label: 'Juegos', icon: Gamepad2 },
  { to: '/gym', label: 'Gimnasio', icon: Dumbbell, roles: ['admin', 'guest'], module: 'gym' },
  { to: '/ytdlp', label: 'YouTube a MP3', icon: Music, roles: ['admin', 'guest'], module: 'ytdlp' },
  { to: '/configuration', label: 'Configuración', icon: Settings, roles: ['admin'] },
]

export default function Sidebar() {
  const role = useAuthStore((s) => s.user?.role)
  const modules = useAuthStore((s) => s.modules)
  const items = navItems.filter((item) => {
    if (item.roles && (role == null || !item.roles.includes(role))) return false
    if (item.module && role !== 'admin' && !modules.includes(item.module)) return false
    return true
  })

  return (
    <aside className="flex h-full w-56 flex-col border-r bg-card">
      <div className="flex h-14 items-center border-b px-4">
        <span className="font-bold text-foreground">Work Hub</span>
      </div>
      <nav className="flex-1 space-y-1 p-2">
        {items.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
