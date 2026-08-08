// Minimal service worker — its only job is receiving Web Push events and
// showing a notification. No caching/offline strategy here; the manifest
// already makes the app installable, this file just adds the push
// receiver that installability alone doesn't include.

self.addEventListener('push', (event) => {
  let data = { title: 'Work Hub', body: '' }
  if (event.data) {
    try {
      data = event.data.json()
    } catch {
      data.body = event.data.text()
    }
  }
  event.waitUntil(
    self.registration.showNotification(data.title ?? 'Work Hub', {
      body: data.body ?? '',
      icon: '/icon-192.webp',
      badge: '/icon-192.webp',
      tag: 'riddle-reminder', // replaces any earlier unread reminder instead of stacking
    })
  )
})

// Focus an already-open tab if there is one, otherwise open a new one at
// the games page — clicking a "you haven't solved it" reminder should land
// you on the riddle, not the dashboard.
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      for (const client of clientList) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.navigate('/juegos')
          return client.focus()
        }
      }
      return self.clients.openWindow('/juegos')
    })
  )
})
