// Port src/proxy.ts (repo lama) — gating edge untuk shell terautentikasi:
// verifikasi signature refresh_token cookie untuk /admin, /mitra, /klien;
// unauthenticated → /login dengan returnTo. Cek role detail tetap di layout.
import { jwtVerify } from 'jose';
import type { Handle } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { redirect } from '@sveltejs/kit';

const secretString = env.APP_SECRET ?? 'local-development-secret-change-before-production';
const secret = new TextEncoder().encode(secretString);

function safeReturnTo(value: string | null | undefined, origin: string): string | null {
	if (!value || value.length > 4096 || /[\u0000-\u001f\u007f]/.test(value)) return null;
	let target: URL;
	try {
		target = new URL(value, origin);
	} catch {
		return null;
	}
	if (target.origin !== origin) return null;
	if (!value.startsWith('/') && !/^https?:\/\//i.test(value)) return null;
	if (value.startsWith('//') || target.pathname.startsWith('//')) return null;
	const normalized = target.pathname.replace(/\/+$/, '') || '/';
	if (normalized === '/login' || normalized.startsWith('/login/')) return null;
	return `${target.pathname}${target.search}${target.hash}`;
}

// Redirect legacy (parity next.config.ts redirects, permanent):
// /brands → /klien, /platform → /admin, /partner → /mitra.
const LEGACY_REDIRECTS: Array<[string, string]> = [
	['/brands', '/klien'],
	['/platform', '/admin'],
	['/partner', '/mitra']
];

export const handle: Handle = async ({ event, resolve }) => {
	const { pathname } = event.url;

	for (const [from, to] of LEGACY_REDIRECTS) {
		if (pathname === from || pathname.startsWith(`${from}/`)) {
			throw redirect(308, `${to}${pathname.slice(from.length)}${event.url.search}`);
		}
	}

	const gated =
		pathname.startsWith('/admin') || pathname.startsWith('/mitra') || pathname.startsWith('/klien');
	if (gated) {
		const token = event.cookies.get('refresh_token');
		let valid = false;
		if (token) {
			try {
				const { payload } = await jwtVerify(token, secret, { algorithms: ['HS256'] });
				valid = payload.type === 'refresh' && typeof payload.sub === 'string';
			} catch {
				valid = false;
			}
		}
		if (!valid) {
			const returnTo = safeReturnTo(`${pathname}${event.url.search}`, event.url.origin);
			throw redirect(307, `/login${returnTo ? `?returnTo=${encodeURIComponent(returnTo)}` : ''}`);
		}
	}

	return resolve(event);
};
