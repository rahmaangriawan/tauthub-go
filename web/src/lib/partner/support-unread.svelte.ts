import { apiRequest } from '$lib/auth';

// Badge menu "Dukungan" di sidebar mitra: jumlah balasan admin yang belum
// dibaca. Membuka detail tiket menandai pesan dibaca di server, lalu halaman
// memanggil refreshSupportUnreadNow() supaya angka langsung turun.
export const supportUnread = $state({ count: 0 });

let timer: ReturnType<typeof setInterval> | null = null;

async function refresh() {
	try {
		const res = await apiRequest('/v1/partner/tickets/unread-count');
		if (!res.ok) return;
		const d = await res.json();
		supportUnread.count = (d.unread as number) ?? 0;
	} catch {
		/* polling diam-diam */
	}
}

// Segarkan juga saat tab kembali aktif: admin sering membalas dari tab lain,
// dan menunggu putaran 20 detik membuat badge terasa tidak berfungsi.
function onVisible() {
	if (document.visibilityState === 'visible') void refresh();
}

export function startSupportUnreadPolling() {
	void refresh();
	if (timer) clearInterval(timer);
	timer = setInterval(() => void refresh(), 20_000);
	document.addEventListener('visibilitychange', onVisible);
}

export function stopSupportUnreadPolling() {
	if (timer) clearInterval(timer);
	timer = null;
	document.removeEventListener('visibilitychange', onVisible);
}

export async function refreshSupportUnreadNow() {
	await refresh();
}
