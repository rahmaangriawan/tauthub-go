import { apiRequest } from '$lib/auth';

// Badge "Publikasi" di sidebar portal mitra: jumlah publikasi yang dibuat
// SETELAH mitra terakhir kali membuka halaman Publikasinya. Halaman
// Publikasi memanggil markPubSeen() setelah daftar terrender → angka hilang;
// publikasi baru masuk → poll berikutnya menampilkan angka lagi.
export const pubsUnread = $state({ count: 0 });

let timer: ReturnType<typeof setInterval> | null = null;

async function refreshPubsUnread(apiBase = '/v1/partner') {
	try {
		const res = await apiRequest(`${apiBase}/publications/unread-count`);
		if (!res.ok) return;
		const d = await res.json();
		pubsUnread.count = (d.unread as number) ?? 0;
	} catch {
		/* polling diam-diam */
	}
}

/** Mulai poll (interval 20 detik) + segarkan sekali; dipanggil layout portal mitra. */
export function startPubsUnreadPolling() {
	void refreshPubsUnread();
	if (timer) clearInterval(timer);
	timer = setInterval(() => void refreshPubsUnread(), 20_000);
}

export function stopPubsUnreadPolling() {
	if (timer) {
		clearInterval(timer);
		timer = null;
	}
}

/** Segarkan angka saat ini (dipakai halaman Publikasi). */
export async function refreshPubsUnreadNow() {
	await refreshPubsUnread();
}

/** Halaman Publikasi terrender → tandai semua publikasi sudah dibaca. */
export async function markPubSeen() {
	try {
		const res = await apiRequest('/v1/partner/publications/mark-seen', { method: 'POST' });
		if (res.ok) pubsUnread.count = 0;
	} catch {
		/* best-effort */
	}
}
