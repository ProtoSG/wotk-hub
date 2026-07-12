import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface CoupleCoverProps {
  onNewDate: () => void
}

/**
 * Decorative hero band for the Citas page — a deliberate, explicit exception
 * to the app's restrained Ghibli direction (see .impeccable.md). The user
 * asked specifically for this ONE page to be more "llamativa": a wabi-sabi
 * ink-wash (sumi-e) silhouette scene — asymmetric mountain layers, a
 * crescent moon tucked off-center, a sakura branch drifting in from the
 * corner. Hand-crafted inline SVG, no external assets, no animation.
 *
 * Colors come from the `--cover-*` custom properties in index.css, a
 * dedicated "night ink" palette scoped to this component only — it does not
 * touch --background/--primary/etc.
 */
function Blossom({ cx, cy, scale = 1 }: { cx: number; cy: number; scale?: number }) {
  const r = 5 * scale
  const petalOffset = 6 * scale
  const angles = [0, 72, 144, 216, 288]
  return (
    <g>
      {angles.map((deg) => {
        const rad = (deg * Math.PI) / 180
        const px = cx + Math.cos(rad) * petalOffset
        const py = cy + Math.sin(rad) * petalOffset
        return <ellipse key={deg} cx={px} cy={py} rx={r} ry={r * 0.72} fill="var(--cover-sakura)" opacity={0.9} />
      })}
      <circle cx={cx} cy={cy} r={r * 0.5} fill="var(--cover-sakura-dark)" opacity={0.85} />
    </g>
  )
}

export default function CoupleCover({ onNewDate }: CoupleCoverProps) {
  return (
    <div className="relative h-[170px] w-full overflow-hidden rounded-[var(--radius)] sm:h-[210px] lg:h-[250px]">
      <svg
        viewBox="0 0 1200 400"
        preserveAspectRatio="xMidYMax slice"
        className="absolute inset-0 h-full w-full"
        role="img"
        aria-label="Ilustración de tinta sumi-e con montañas, luna creciente y una rama de sakura"
      >
        <defs>
          <linearGradient id="cover-sky" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--cover-sky-top)" />
            <stop offset="100%" stopColor="var(--cover-sky-bottom)" />
          </linearGradient>
        </defs>

        {/* dusk sky wash */}
        <rect x="0" y="0" width="1200" height="400" fill="url(#cover-sky)" />

        {/* crescent moon, off-center upper right — asymmetric wabi-sabi placement */}
        <circle cx="880" cy="108" r="42" fill="var(--cover-moon)" />
        <circle cx="862" cy="94" r="37" fill="var(--cover-sky-top)" />

        {/* mountain silhouettes, three receding layers, asymmetric peaks */}
        <path
          d="M0,260 C150,232 300,248 420,224 C560,202 680,236 800,214 C920,194 1050,222 1200,206 L1200,400 L0,400 Z"
          fill="var(--cover-ink-far)"
        />
        <path
          d="M0,302 C120,292 220,262 340,272 C480,284 560,232 660,216 C760,200 860,252 960,256 C1060,260 1140,240 1200,246 L1200,400 L0,400 Z"
          fill="var(--cover-ink-mid)"
        />
        <path
          d="M0,344 C100,338 180,322 260,327 C360,332 420,300 500,290 C600,278 680,198 760,190 C840,182 900,240 980,262 C1060,280 1140,270 1200,276 L1200,400 L0,400 Z"
          fill="var(--cover-ink-near)"
        />

        {/* sakura branch drifting in from the top-left corner, classic
            sumi-e corner composition */}
        <path
          d="M-10,18 C55,46 95,58 150,90 C205,122 235,132 290,150"
          fill="none"
          stroke="var(--cover-ink-near)"
          strokeWidth="3"
          strokeLinecap="round"
          opacity={0.85}
        />
        <path
          d="M108,72 C118,54 128,44 145,30"
          fill="none"
          stroke="var(--cover-ink-near)"
          strokeWidth="2"
          strokeLinecap="round"
          opacity={0.85}
        />
        <path
          d="M212,120 C226,104 242,96 262,84"
          fill="none"
          stroke="var(--cover-ink-near)"
          strokeWidth="2"
          strokeLinecap="round"
          opacity={0.85}
        />

        <Blossom cx={62} cy={34} scale={0.8} />
        <Blossom cx={145} cy={28} scale={1} />
        <Blossom cx={168} cy={98} scale={0.7} />
        <Blossom cx={260} cy={80} scale={0.9} />
        <Blossom cx={295} cy={148} scale={0.75} />
        <circle cx="120" cy="60" r="3" fill="var(--cover-sakura)" opacity={0.7} />
        <circle cx="235" cy="110" r="2.5" fill="var(--cover-sakura)" opacity={0.7} />
      </svg>

      {/* scrim so the overlaid title/button stay legible against the
          illustration regardless of what's behind them */}
      <div
        className="absolute inset-0"
        style={{ background: 'linear-gradient(to top, var(--cover-scrim), transparent 65%)' }}
      />

      <div className="relative flex h-full flex-col justify-end gap-4 p-4 sm:p-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h1
            className="text-2xl font-bold drop-shadow-sm"
            style={{ color: 'var(--cover-text)' }}
          >
            Citas
          </h1>
          <div className="hidden sm:block">
            <Button onClick={onNewDate}>
              <Plus size={14} />
              Nueva cita
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
