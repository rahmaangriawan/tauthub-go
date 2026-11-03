/**
 * Helper error halaman admin platform — parity `lib/platform.ts` app lama:
 * RBAC mengembalikan 404 dengan sentinel "Resource not found" saat akses
 * ditolak, dan pesan itu dipetakan ke pesan akses platform.
 */
import { readApiError } from '$lib/auth';

export const ACCESS_DENIED = 'Akses platform administrator tidak tersedia.';

/**
 * Pesan 403 dari middleware SuperAdminOnly. Rute yang hanya untuk Super Admin
 * membalas 403 (bukan sentinel 404), jadi halaman perlu mengenalinya juga
 * supaya menampilkan kartu "tidak punya akses", bukan kotak error merah.
 */
export const SUPER_ADMIN_ONLY = 'Hanya Super Admin yang boleh membuka ini.';

export async function platformError(
	res: Response,
	fallback = 'Permintaan platform gagal.'
): Promise<string> {
	const detail = await readApiError(res, fallback);
	return detail === 'Resource not found' ? ACCESS_DENIED : detail;
}

/** true bila pesan error berarti akses modul tidak tersedia. */
export function isAccessDenied(message: string): boolean {
	return message === ACCESS_DENIED || message === SUPER_ADMIN_ONLY;
}
