import { redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { env } from '$env/dynamic/private';

/**
 * Landing link referral — parity app/register/route.ts app lama.
 *
 * Alur: `/register?ref=KODE[&role=mitra|klien]` → catat klik ke backend Go
 * (`POST /v1/auth/referral/click`, yang meng-upsert `referral_clicks` + audit
 * `referral.click`) → set cookie `tauthub_ref` 30 hari → redirect ke halaman
 * daftar sesuai peran, atau `/login?mode=register&ref=`.
 *
 * Cookie/link itu yang nanti dibaca AuthPanel sebagai atribusi saat daftar.
 * Kode tak dikenal (ok:false) → langsung `/login?mode=register` tanpa cookie,
 * persis perilaku app lama.
 *
 * API_URL opsional untuk produksi (container terpisah); default
 * http://localhost:8080 = target proxy Vite dev dan nginx (REWRITE/02 §9).
 */
const API_URL = env.API_URL ?? 'http://localhost:8080';

/** Catat klik referral; true bila kode dikenal partner. Kegagalan jaringan = false. */
async function recordClick(ref: string, request: Request, clientAddress: string): Promise<boolean> {
	// Fingerprint backend = ip|ua, jadi kirim IP & UA browser — bukan IP web server.
	const forwarded = request.headers.get('x-forwarded-for')?.split(',')[0]?.trim();
	const ip = forwarded || request.headers.get('x-real-ip') || clientAddress;
	const headers: Record<string, string> = { 'Content-Type': 'application/json' };
	if (ip) headers['x-forwarded-for'] = ip;
	const ua = request.headers.get('user-agent');
	if (ua) headers['user-agent'] = ua;

	try {
		const res = await fetch(`${API_URL}/v1/auth/referral/click`, {
			method: 'POST',
			headers,
			body: JSON.stringify({ ref }),
			signal: AbortSignal.timeout(6_000)
		});
		if (!res.ok) return false;
		const data = (await res.json()) as { ok?: boolean };
		return Boolean(data.ok);
	} catch {
		// API tidak terjangkau: perlakukan sebagai kode tak dikenal, jangan
		// blokir user — mereka tetap diarahkan ke halaman daftar.
		return false;
	}
}

export const GET: RequestHandler = async ({ url, request, cookies, getClientAddress }) => {
	const ref = (url.searchParams.get('ref') ?? '').trim().slice(0, 64);
	const role = (url.searchParams.get('role') ?? '').trim();

	if (ref && (await recordClick(ref, request, getClientAddress()))) {
		// httpOnly: false WAJIB — AuthPanel membaca cookie ini lewat
		// document.cookie (parity app lama; Next.js tidak men-set HttpOnly).
		cookies.set('tauthub_ref', ref, { path: '/', sameSite: 'lax', maxAge: 60 * 60 * 24 * 30, httpOnly: false });
		const rolePath = role === 'mitra' || role === 'partner' ? 'mitra' : role === 'klien' || role === 'client' ? 'klien' : null;
		const params = new URLSearchParams(rolePath ? { ref } : { mode: 'register', ref });
		throw redirect(307, rolePath ? `/register/${rolePath}?${params}` : `/login?${params}`);
	}

	throw redirect(307, '/login?mode=register');
};
