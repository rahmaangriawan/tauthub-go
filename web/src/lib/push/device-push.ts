/**
 * Langganan Web Push perangkat ini (browser desktop & mobile).
 *
 * Dipakai bersama halaman profil klien dan mitra. Kunci publik VAPID diambil
 * dari API (bukan konstanta di klien) supaya rotasi kunci tidak perlu rilis
 * ulang frontend; API mengembalikan enabled=false bila push belum dikonfigurasi.
 */
import { apiRequest, readApiError } from '$lib/auth';

export type PushConfig = { enabled: boolean; public_key: string; subscriptions: number };

export type DevicePushState =
	| 'unsupported'
	| 'unconfigured'
	| 'default' // didukung, izin belum diminta
	| 'denied'
	| 'subscribed'
	| 'granted' // izin ada tapi perangkat ini belum terdaftar
	| 'ios-needs-install';

/** base64url → Uint8Array untuk applicationServerKey. */
function urlBase64ToUint8Array(base64: string): Uint8Array<ArrayBuffer> {
	const padding = '='.repeat((4 - (base64.length % 4)) % 4);
	const normalized = (base64 + padding).replace(/-/g, '+').replace(/_/g, '/');
	const raw = window.atob(normalized);
	// ArrayBuffer eksplisit: applicationServerKey butuh BufferSource biasa.
	const out = new Uint8Array(new ArrayBuffer(raw.length));
	for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
	return out;
}

/**
 * iOS/iPadOS Safari hanya mengizinkan push setelah app dipasang ke Home Screen.
 * Dikembalikan sebagai keadaan tersendiri supaya halaman bisa menjelaskannya
 * alih-alih menampilkan tombol yang pasti gagal.
 */
function isIosWithoutStandalone(): boolean {
	if (typeof window === 'undefined') return false;
	const ua = window.navigator.userAgent;
	const isIos = /iPad|iPhone|iPod/.test(ua) || (ua.includes('Macintosh') && 'ontouchend' in document);
	const standalone =
		window.matchMedia('(display-mode: standalone)').matches ||
		(window.navigator as unknown as { standalone?: boolean }).standalone === true;
	return isIos && !standalone;
}

export function pushSupported(): boolean {
	return (
		typeof window !== 'undefined' &&
		'serviceWorker' in navigator &&
		'PushManager' in window &&
		'Notification' in window
	);
}

export async function fetchPushConfig(): Promise<PushConfig> {
	const res = await apiRequest('/v1/push/config', { cache: 'no-store' });
	if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat konfigurasi notifikasi.'));
	return (await res.json()) as PushConfig;
}

async function registerWorker(): Promise<ServiceWorkerRegistration> {
	const existing = await navigator.serviceWorker.getRegistration('/sw.js');
	if (existing) return existing;
	return navigator.serviceWorker.register('/sw.js', { scope: '/' });
}

export async function currentState(config: PushConfig | null): Promise<DevicePushState> {
	if (!pushSupported()) return isIosWithoutStandalone() ? 'ios-needs-install' : 'unsupported';
	if (!config || !config.enabled) return 'unconfigured';
	if (Notification.permission === 'denied') return 'denied';
	const reg = await navigator.serviceWorker.getRegistration('/sw.js');
	const sub = await reg?.pushManager.getSubscription();
	if (sub) return 'subscribed';
	return Notification.permission === 'granted' ? 'granted' : 'default';
}

/** Minta izin, daftarkan service worker, lalu simpan langganan ke API. */
export async function enableDevicePush(config: PushConfig): Promise<DevicePushState> {
	if (!pushSupported() || !config.enabled) return 'unsupported';
	const permission = await Notification.requestPermission();
	if (permission !== 'granted') return permission === 'denied' ? 'denied' : 'default';
	const reg = await registerWorker();
	await navigator.serviceWorker.ready;

	const key = urlBase64ToUint8Array(config.public_key);
	let sub = await reg.pushManager.getSubscription();
	if (!sub) {
		sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
	}
	const res = await apiRequest('/v1/push/subscriptions', {
		method: 'POST',
		body: JSON.stringify(sub.toJSON())
	});
	if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan langganan perangkat.'));
	return 'subscribed';
}

/** Batalkan langganan perangkat ini (API lalu browser). Idempoten. */
export async function disableDevicePush(): Promise<DevicePushState> {
	if (!pushSupported()) return 'unsupported';
	const reg = await navigator.serviceWorker.getRegistration('/sw.js');
	const sub = await reg?.pushManager.getSubscription();
	if (!sub) return Notification.permission === 'denied' ? 'denied' : 'default';
	const endpoint = sub.endpoint;
	try {
		await apiRequest('/v1/push/subscriptions', {
			method: 'DELETE',
			body: JSON.stringify({ endpoint })
		});
	} catch {
		// Tetap lepaskan di sisi browser; baris mati akan dibersihkan worker saat 410.
	}
	await sub.unsubscribe();
	return Notification.permission === 'denied' ? 'denied' : 'default';
}

export async function sendTestPush(): Promise<{ sent: number; devices: number }> {
	const res = await apiRequest('/v1/push/test', { method: 'POST' });
	if (!res.ok) throw new Error(await readApiError(res, 'Gagal mengirim notifikasi uji.'));
	return (await res.json()) as { sent: number; devices: number };
}
