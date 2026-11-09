/**
 * Deteksi host situs publik (data.tauthub.com) vs aplikasi (app.tauthub.com).
 *
 * Satu aplikasi SvelteKit melayani dua host: katalog publik dan dashboard.
 * Semua aturan host ada di sini supaya hooks, robots, dan halaman memakai
 * definisi yang sama.
 */
import { env } from '$env/dynamic/private';

const DATA_HOST = (env.PUBLIC_CATALOG_HOST ?? 'data.tauthub.com').toLowerCase();
const APP_HOST = (env.APP_HOST ?? 'app.tauthub.com').toLowerCase();
const DATA_ORIGIN = (env.PUBLIC_CATALOG_ORIGIN ?? `https://${DATA_HOST}`).replace(/\/+$/, '');
const APP_ORIGIN = (env.APP_ORIGIN ?? `https://${APP_HOST}`).replace(/\/+$/, '');

/** Host katalog publik: tepat host yang dikonfigurasi, atau subdomain `data.`. */
export function isDataHost(hostname: string): boolean {
	const host = hostname.toLowerCase();
	return host === DATA_HOST || host.startsWith('data.');
}

/** Host dashboard. Selain ini (localhost, IP, staging) tidak ada redirect lintas host. */
export function isAppHost(hostname: string): boolean {
	return hostname.toLowerCase() === APP_HOST;
}

/**
 * Origin kanonik untuk tag canonical/OG/sitemap.
 *
 * Selalu memakai satu host (PUBLIC_CATALOG_ORIGIN), termasuk saat halaman
 * dibuka lewat host lain: preview Cloudflare, staging, atau `www.data...`.
 * Kalau origin mengikuti host permintaan, halaman yang sama akan mengklaim
 * dirinya kanonik di beberapa host sekaligus — persis duplikasi yang ingin
 * dicegah canonical.
 */
export function canonicalOrigin(_hostname?: string): string {
	return DATA_ORIGIN;
}

export function dataOrigin(): string {
	return DATA_ORIGIN;
}

export function appOrigin(): string {
	return APP_ORIGIN;
}

/** Kill switch yang sama dengan API (PUBLIC_CATALOG_ENABLED=false). */
export function catalogEnabled(): boolean {
	return (env.PUBLIC_CATALOG_ENABLED ?? 'true').toLowerCase() !== 'false';
}

// Area yang hanya ada di dashboard. Di host publik path ini dialihkan ke app.
const APP_ONLY_PREFIXES = [
	'/login',
	'/register',
	'/admin',
	'/klien',
	'/mitra',
	'/forgot-password',
	'/reset-password',
	'/payments',
	'/berita-acara',
	'/docs',
	'/dokumentasi',
	'/blogger-pick-blog'
];

export function isAppOnlyPath(pathname: string): boolean {
	return APP_ONLY_PREFIXES.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

// Path katalog yang juga punya URL di host aplikasi. Di host aplikasi path ini
// dialihkan ke host publik supaya tidak ada dua URL untuk konten yang sama.
// robots.txt/sitemap.xml sengaja TIDAK termasuk: keduanya harus dilayani di
// host masing-masing dengan aturan berbeda.
export function isCatalogPath(pathname: string): boolean {
	return pathname.startsWith('/media') || pathname.startsWith('/kategori');
}

export type SeoMeta = {
	title: string;
	description?: string;
	canonical?: string;
	robots?: string;
	ogType?: string;
	jsonLd?: unknown;
};

/** Escape JSON-LD supaya tidak bisa menutup tag <script> lebih awal. */
export function jsonLdScript(data: unknown): string {
	return JSON.stringify(data).replace(/</g, '\\u003c');
}
