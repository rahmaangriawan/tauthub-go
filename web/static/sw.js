// Service worker Tauthub: hanya untuk Web Push. Tidak melakukan caching —
// aplikasi tetap online-first seperti biasa.

self.addEventListener('install', () => {
	self.skipWaiting();
});

self.addEventListener('activate', (event) => {
	event.waitUntil(self.clients.claim());
});

self.addEventListener('push', (event) => {
	let data = { title: 'Tauthub', body: '', url: '/', tag: undefined };
	try {
		if (event.data) data = { ...data, ...event.data.json() };
	} catch {
		if (event.data) data.body = event.data.text();
	}
	const options = {
		body: data.body || '',
		icon: '/tauthub-logo.webp',
		badge: '/tauthub-logo.webp',
		tag: data.tag,
		data: { url: data.url || '/' },
		// Pengumuman & hal mendesak menyusul di atas notifikasi sebelumnya
		// bertag sama tanpa bergetar lagi.
		renotify: false
	};
	event.waitUntil(self.registration.showNotification(data.title || 'Tauthub', options));
});

self.addEventListener('notificationclick', (event) => {
	event.notification.close();
	const target = new URL(event.notification.data?.url || '/', self.location.origin).href;
	event.waitUntil(
		self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((list) => {
			for (const client of list) {
				if (client.url === target && 'focus' in client) return client.focus();
			}
			for (const client of list) {
				if ('navigate' in client && 'focus' in client) {
					return client.navigate(target).then((c) => c && c.focus());
				}
			}
			return self.clients.openWindow(target);
		})
	);
});

// Browser bisa mengganti langganan (rotasi kunci push server). Kirim ulang ke
// API supaya endpoint di database tetap yang terbaru.
self.addEventListener('pushsubscriptionchange', (event) => {
	event.waitUntil(
		(async () => {
			const sub = await self.registration.pushManager.subscribe(event.oldSubscription?.options ?? {});
			await fetch('/v1/push/subscriptions', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(sub.toJSON())
			});
		})()
	);
});
