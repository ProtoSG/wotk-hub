import { NavLink } from 'react-router-dom'
import { LayoutDashboard, Database, Settings, Wallet, Heart } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore, type Role } from '@/store/authStore'

const navItems: { to: string; label: string; icon: typeof LayoutDashboard; roles?: readonly Role[] }[] = [
  { to: '/dashboard', label: 'Inicio', icon: LayoutDashboard },
  { to: '/db-manager', label: 'DB Manager', icon: Database, roles: ['admin'] },
  { to: '/finances', label: 'Finanzas', icon: Wallet },
  { to: '/citas', label: 'Citas', icon: Heart },
  { to: '/configuration', label: 'Configuración', icon: Settings },
]

export default function Sidebar() {
  const role = useAuthStore((s) => s.user?.role)
  const items = navItems.filter((item) => !item.roles || (role != null && item.roles.includes(role)))

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
            <Icon size={16} />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
