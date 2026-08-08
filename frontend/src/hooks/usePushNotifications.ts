import { useEffect, useState } from 'react'
import { isPushSupported, getExistingSubscription, subscribeToPush, unsubscribeFromPush } from '@/lib/push'

export function usePushNotifications() {
  const supported = isPushSupported()
  const [subscribed, setSubscribed] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!supported) return
    let cancelled = false
    getExistingSubscription().then((sub) => {
      if (!cancelled) setSubscribed(!!sub)
    })
    return () => {
      cancelled = true
    }
  }, [supported])

  async function toggle() {
    setBusy(true)
    try {
      if (subscribed) {
        await unsubscribeFromPush()
        setSubscribed(false)
      } else {
        await subscribeToPush()
        setSubscribed(true)
      }
    } finally {
      setBusy(false)
    }
  }

  return { supported, subscribed, busy, toggle }
}
