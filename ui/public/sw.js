// Service worker of the app: shows the monitor's push notifications (server down or back, report, suspect player,
// B3 ban) and opens the right screen when one is tapped. No fetch handler: the app always loads from the network.
// Served at /sw.js: its scope is the whole site.

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', e => e.waitUntil(self.clients.claim()));

self.addEventListener('push', e => {
  let d = {};
  try {
    d = e.data ? e.data.json() : {};
  } catch {
    d = { body: e.data ? e.data.text() : '' };
  }
  e.waitUntil(self.registration.showNotification(d.title || 'CoD4 Monitor', {
    body: d.body || '',
    tag: d.tag || undefined,
    renotify: !!d.tag,
    icon: '/icon-192.png',
    data: { url: d.url || '/' }
  }));
});

self.addEventListener('notificationclick', e => {
  e.notification.close();
  const url = new URL(e.notification.data?.url || '/', self.location.origin);
  e.waitUntil((async () => {
    const wins = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    const app = wins.find(w => new URL(w.url).pathname === url.pathname);
    if (app) {
      // the app is open: it selects the server and the screen itself (no reload)
      app.postMessage({ type: 'open', server: Number(url.searchParams.get('s')) || 0, hash: url.hash });
      return app.focus();
    }
    return self.clients.openWindow(url.href);
  })());
});
