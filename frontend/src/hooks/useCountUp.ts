import { useEffect, useRef, useState } from 'react'

/** Animates from the previous value to `target` over `durationMs` using an
 * ease-out cubic curve. First render shows `target` immediately (no
 * animation from a fake starting point). */
export function useCountUp(target: number, durationMs = 700) {
  const [value, setValue] = useState(target)
  const valueRef = useRef(target)

  useEffect(() => {
    const from = valueRef.current
    if (from === target) return

    const start = performance.now()
    let raf: number

    const tick = (now: number) => {
      const t = Math.min((now - start) / durationMs, 1)
      const eased = 1 - (1 - t) ** 3
      const next = Math.round(from + (target - from) * eased)
      valueRef.current = next
      setValue(next)
      if (t < 1) raf = requestAnimationFrame(tick)
    }

    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [target, durationMs])

  return value
}
