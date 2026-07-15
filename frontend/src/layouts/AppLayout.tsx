import { Outlet } from 'react-router-dom'
import { useEffect, useRef, useState } from 'react'
import { useThemeStore } from '@/store/themeStore'
import Sidebar from './Sidebar'
import TopBar from './TopBar'
import { Sheet, SheetContent } from '@/components/ui/sheet'

export default function AppLayout() {
  const theme = useThemeStore((s) => s.theme)
  const [navOpen, setNavOpen] = useState(false)
  const touchStartX = useRef<number | null>(null)

  useEffect(() => {
    const root = document.documentElement
    root.classList.toggle('dark', theme === 'dark')
  }, [theme])

  function handleTouchStart(e: React.TouchEvent) {
    touchStartX.current = e.touches[0].clientX
  }

  function handleTouchMove(e: React.TouchEvent) {
    if (touchStartX.current === null) return
    const delta = e.touches[0].clientX - touchStartX.current
    // Swipe right from left edge (first 50px of screen), minimum 80px travel
    if (delta > 80 && touchStartX.current < 50) {
      setNavOpen(true)
      touchStartX.current = null
    }
  }

  function handleTouchEnd() {
    touchStartX.current = null
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      {/* Desktop sidebar — always visible on lg+ */}
      <div className="hidden lg:flex">
        <Sidebar />
      </div>

      {/* Mobile: swipe detection zone */}
      <div
        className="flex flex-1 flex-col overflow-hidden"
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        <TopBar onMenuClick={() => setNavOpen(true)} />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>

      {/* Mobile nav sheet */}
      <Sheet open={navOpen} onOpenChange={setNavOpen}>
        <SheetContent side="left" className="w-56 p-0">
          <Sidebar />
        </SheetContent>
      </Sheet>
    </div>
  )
}
