import api from '@/lib/axios'

/**
 * Web Push plumbing — register the service worker, subscribe/unsubscribe a
 * device, and report whether this browser already has a live subscription.
 * Kept out of the React tree (no hook here) since none of this needs
 * re-render-driven reactivity; callers just await these directly.
 */

// Push notifications need a service worker as the receiving end — no
// browser support means don't even try (iOS Safari outside an installed
// PWA, very old browsers).
export function isPushSupported(): boolean {
  return 'serviceWorker' in navigator && 'PushManager' in window
}

export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  if (!isPushSupported()) return null
  return navigator.serviceWorker.register('/sw.js')
}

// PushManager.subscribe wants the VAPID public key as a Uint8Array, not the
// base64url string the backend hands out — standard conversion, same as
// every Web Push tutorial's applicationServerKey helper.
function urlBase64ToUint8Array(base64: string): Uint8Array {
  const padding = '='.repeat((4 - (base64.length % 4)) % 4)
  const b64 = (base64 + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(b64)
  const bytes = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
  return bytes
}

export async function getExistingSubscription(): Promise<PushSubscription | null> {
  if (!isPushSupported()) return null
  const reg = await navigator.serviceWorker.getRegistration('/sw.js')
  if (!reg) return null
  return reg.pushManager.getSubscription()
}

// Requests notification permission, subscribes this device, and reports the
// subscription to the backend. Throws if the user denies permission — the
// caller decides how to surface that (toast, silently give up, etc).
export async function subscribeToPush(): Promise<void> {
  if (!isPushSupported()) throw new Error('Este navegador no soporta notificaciones push')

  const permission = await Notification.requestPermission()
  if (permission !== 'granted') throw new Error('Permiso de notificaciones denegado')

  const reg = await registerServiceWorker()
  if (!reg) throw new Error('No se pudo registrar el service worker')
  await navigator.serviceWorker.ready

  const { data } = await api.get<{ publicKey: string }>('/api/push/vapid-key')
  const subscription = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(data.publicKey),
  })

  const json = subscription.toJSON()
  await api.post('/api/push/subscribe', {
    endpoint: json.endpoint,
    keys: { p256dh: json.keys?.p256dh, auth: json.keys?.auth },
  })
}

export async function unsubscribeFromPush(): Promise<void> {
  const sub = await getExistingSubscription()
  if (!sub) return
  await api.post('/api/push/unsubscribe', { endpoint: sub.endpoint })
  await sub.unsubscribe()
}
