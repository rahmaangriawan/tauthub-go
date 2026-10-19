import { apiRequest } from '$lib/auth';

// Jumlah pesan belum dibaca obrolan untuk badge menu "Pesan" di sidebar kedua
// portal. Modul-level $state (pola seperti brand-url.svelte.ts) supaya
// layout dan halaman chat berbagi satu sumber.
export const chatUnread = $state({ count: 0 });

let timer: ReturnType<typeof setInterval> | null = null;
let apiBase = '';

async function refreshChatUnread() {
	if (!apiBase) return;
	try {
		const res = await apiRequest(`${apiBase}/chat/threads`);
		if (!res.ok) return;
		const d = await res.json();
		chatUnread.count = (d.threads ?? []).reduce((n: number, t: { unread_count?: number }) => n + (t.unread_count ?? 0), 0);
	} catch {
		/* polling diam-diam */
	}
}

/** Mulai poll (interval 20 detik) + segarkan sekali; dipanggil dari layout portal. */
export function startChatUnreadPolling(base: string) {
	apiBase = base;
	void refreshChatUnread();
	if (timer) clearInterval(timer);
	timer = setInterval(() => void refreshChatUnread(), 20_000);
}

export function stopChatUnreadPolling() {
	if (timer) {
		clearInterval(timer);
		timer = null;
	}
	apiBase = '';
}

/** Segarkan langsung (dipanggil halaman chat setelah buka/kirim agar badge tidak mengambang 20 detik). */
export async function refreshChatUnreadNow() {
	await refreshChatUnread();
}
