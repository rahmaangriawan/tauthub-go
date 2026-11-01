import type { RequestHandler } from './$types';
import { env } from '$env/dynamic/private';

/**
 * Shim callback OAuth integrasi Google (`/api/blogger/callback` dan
 * `/api/gsc/callback`) — parity route.ts app lama (Next.js).
 *
 * Nginx hanya mem-proxikan /v1, /uploads, /t, /feed, /healthz ke API Go;
 * path /api/* jatuh ke SvelteKit yang tidak punya halamannya, sehingga
 * redirect balik dari Google selalu 404 dan kode OAuth tidak pernah
 * ditukar. Route ini meneruskan request persis ke handler Go, termasuk
 * status 302 + Location hasil callback.
 *
 * URI callback tidak berubah, jadi Authorized redirect URI di Google Cloud
 * Console tidak perlu disentuh. Target mengikuti konvensi register/+server.ts.
 */
const API_URL = env.API_URL ?? 'http://localhost:8080';

const ALLOWED = new Set(['blogger', 'gsc']);

export const GET: RequestHandler = async ({ params, url }) => {
	const integration = params.integration;
	if (!ALLOWED.has(integration)) {
		return new Response(null, { status: 404 });
	}

	try {
		const upstream = await fetch(`${API_URL}/api/${integration}/callback${url.search}`, {
			method: 'GET',
			// Jangan ikuti redirect: Go membalas 302 + Location yang harus
			// diteruskan apa adanya ke browser mitra.
			redirect: 'manual',
			signal: AbortSignal.timeout(15_000)
		});

		const headers = new Headers();
		const location = upstream.headers.get('location');
		if (location) headers.set('location', location);
		const getSetCookie = (upstream.headers as Headers & { getSetCookie?: () => string[] }).getSetCookie;
		for (const cookie of getSetCookie?.call(upstream.headers) ?? []) {
			headers.append('set-cookie', cookie);
		}
		headers.set('cache-control', 'no-store');
		return new Response(null, { status: upstream.status, headers });
	} catch {
		// API tidak terjangkau: kembali ke portal mitra dengan pesan error
		// mengikuti pola redirect gagal dari handler Go.
		return new Response(null, {
			status: 302,
			headers: { location: `/mitra?${integration}_error=api_unreachable` }
		});
	}
};
