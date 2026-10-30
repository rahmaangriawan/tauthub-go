import { apiRequest } from '$lib/auth';

// Badge merah "Status Publikasi" di sidebar klien: jumlah notifikasi
// publication.partner_approved / publication.published yang belum dibaca user.
// Modul-level $state (pola sama dengan chat-unread) supaya layout dan halaman
// publikasi berbagi satu sumber; dibuka halaman publikasi = ditandai dibaca.
export const publicationUnread = $state({ count: 0 });

let timer: ReturnType<typeof setInterval> | null = null;
let apiBase = '';

async function refresh() {
	if (!apiBase) return;
	try {
		const res = await apiRequest(`${apiBase}/notifications/publication-unread`);
		if (!res.ok) return;
		const d = await res.json();
		publicationUnread.count = Number(d?.count ?? 0);
	} catch {
		/* polling diam-diam */
	}
}

/** Mulai poll (60 detik) + segarkan sekali; dipanggil dari layout klien. */
export function startPublicationUnreadPolling(base: string) {
	apiBase = base;
	void refresh();
	if (timer) clearInterval(timer);
	timer = setInterval(() => void refresh(), 60_000);
}

export function stopPublicationUnreadPolling() {
	if (timer) {
		clearInterval(timer);
		timer = null;
	}
	apiBase = '';
}

/** Tandai semua notifikasi publikasi dibaca (halaman Status Publikasi dibuka). */
export async function markPublicationUnreadRead() {
	if (!apiBase) return;
	try {
		await apiRequest(`${apiBase}/notifications/publication-unread`, { method: 'POST' });
		publicationUnread.count = 0;
	} catch {
		/* best-effort */
	}
}

/** Segarkan langsung (mis. setelah kembali dari halaman publikasi). */
export async function refreshPublicationUnreadNow() {
	await refresh();
}
