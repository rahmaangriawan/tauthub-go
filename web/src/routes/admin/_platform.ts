/**
 * Helper error halaman admin platform — parity `lib/platform.ts` app lama:
 * RBAC mengembalikan 404 dengan sentinel "Resource not found" saat akses
 * ditolak, dan pesan itu dipetakan ke pesan akses platform.
 */
import { readApiError } from '$lib/auth';

export const ACCESS_DENIED = 'Akses platform administrator tidak tersedia.';

export async function platformError(
	res: Response,
	fallback = 'Permintaan platform gagal.'
): Promise<string> {
	const detail = await readApiError(res, fallback);
	return detail === 'Resource not found' ? ACCESS_DENIED : detail;
}

/** true bila pesan error berarti akses modul tidak tersedia. */
export function isAccessDenied(message: string): boolean {
	return message === ACCESS_DENIED;
}
