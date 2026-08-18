import { useCallback, useEffect, useRef } from 'react'
import api from '@/lib/axios'
import type { GameSocketEvent } from '@/types/games.types'

const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 15000
// Jitter as a fraction of the computed backoff, so many idle tabs
// reconnecting after e.g. a shared backend restart don't all hit the
// server in the same instant.
const RECONNECT_JITTER_RATIO = 0.3

// Builds the games WS URL from the same host axios talks to (VITE_API_URL,
// see lib/axios.ts) rather than window.location — this app's dev setup
// serves the frontend and backend on different origins/ports (no Vite dev
// proxy, see vite.config.ts), so a window.location-derived URL would point
// the socket at the frontend's own origin and never reach the API.
function buildWsUrl(): string {
  const base = api.defaults.baseURL ?? window.location.origin
  const url = new URL('/api/games/ws', base)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

interface UseGameSocketOptions {
  /** Fired for every parsed message the server pushes. */
  onMessage: (event: GameSocketEvent) => void
  /**
   * Fired every time the socket finishes connecting (including the very
   * first connect). Callers use this as the cue to do one REST refetch and
   * reconcile anything that happened while disconnected — the hook itself
   * never refetches on its own.
   */
  onOpen?: () => void
}

// Long-lived WebSocket connection to the games live-update channel
// (GET /api/games/ws), shared by UltimaPreguntaTab and EmojiMoviesTab in
// place of the old 6s poll. Reconnects with exponential backoff (1s → 15s,
// jittered) on close/error, pauses entirely while the tab is hidden and
// resumes on visibility return, and cleans up on unmount.
export function useGameSocket({ onMessage, onOpen }: UseGameSocketOptions) {
  const socketRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const attemptRef = useRef(0)
  const pausedRef = useRef(false)
  // connectRef lets scheduleReconnect call back into connect without the
  // two needing to be declared in a particular order or pulled into each
  // other's useCallback deps.
  const connectRef = useRef<() => void>(() => {})

  // Latest callbacks in refs so reconnects aren't torn down/rebuilt just
  // because the caller re-rendered with a fresh function identity. Updated
  // in an effect, not inline during render — refs must not be written
  // while rendering.
  const onMessageRef = useRef(onMessage)
  const onOpenRef = useRef(onOpen)
  useEffect(() => {
    onMessageRef.current = onMessage
    onOpenRef.current = onOpen
  }, [onMessage, onOpen])

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  // Closes the current socket, if any, without letting its onclose handler
  // schedule a reconnect — used for deliberate pauses (tab hidden, unmount).
  const closeSocket = useCallback(() => {
    clearReconnectTimer()
    const socket = socketRef.current
    if (socket) {
      socketRef.current = null
      socket.onopen = null
      socket.onmessage = null
      socket.onclose = null
      socket.onerror = null
      socket.close()
    }
  }, [clearReconnectTimer])

  useEffect(() => {
    function scheduleReconnect() {
      clearReconnectTimer()
      const attempt = attemptRef.current++
      const backoff = Math.min(RECONNECT_BASE_MS * 2 ** attempt, RECONNECT_MAX_MS)
      const jitter = backoff * Math.random() * RECONNECT_JITTER_RATIO
      reconnectTimerRef.current = setTimeout(() => connectRef.current(), backoff + jitter)
    }

    function connect() {
      if (pausedRef.current) return
      clearReconnectTimer()

      const socket = new WebSocket(buildWsUrl())
      socketRef.current = socket

      socket.onopen = () => {
        attemptRef.current = 0
        onOpenRef.current?.()
      }
      socket.onmessage = (e) => {
        try {
          onMessageRef.current(JSON.parse(e.data) as GameSocketEvent)
        } catch {
          // malformed/unexpected frame — ignore, the next one may be fine
        }
      }
      socket.onerror = () => {
        socket.close()
      }
      socket.onclose = () => {
        if (socketRef.current === socket) socketRef.current = null
        if (!pausedRef.current) scheduleReconnect()
      }
    }
    connectRef.current = connect

    // Pause while the tab isn't visible — no point holding a live socket
    // open for a screen nobody's looking at (same UX as the old polling's
    // visibility handling). Resume on return with a fresh backoff.
    function handleVisibility() {
      if (document.hidden) {
        pausedRef.current = true
        closeSocket()
      } else {
        pausedRef.current = false
        attemptRef.current = 0
        connect()
      }
    }

    if (!document.hidden) connect()
    document.addEventListener('visibilitychange', handleVisibility)

    return () => {
      document.removeEventListener('visibilitychange', handleVisibility)
      pausedRef.current = true
      closeSocket()
    }
  }, [closeSocket, clearReconnectTimer])
}
