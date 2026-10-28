/**
 * Pembentuk URL kanonik untuk halaman katalog publik.
 *
 * Aturan yang dipakai:
 * - Selalu satu host (origin kanonik dari konfigurasi), bukan host permintaan.
 * - Parameter pelacak (utm_*, gclid, _r, parameter internal SvelteKit) dibuang.
 * - Parameter yang mengubah isi halaman (q, kategori, tier, da, harga, urut,
 *   page) DIPERTAHANKAN. Halaman hasil filter di-canonical ke dirinya sendiri,
 *   bukan ke beranda: menggabungkan `noindex` dengan canonical ke URL lain
 *   mengirim sinyal bertentangan ke Google, dan bisa membuat URL filter
 *   dianggap duplikat dari beranda padahal isinya berbeda.
 * - Parameter diurutkan supaya `?a=1&b=2` dan `?b=2&a=1` menghasilkan satu URL.
 */

const TRACKING_PARAMS = new Set([
	'utm_source',
	'utm_medium',
	'utm_campaign',
	'utm_term',
	'utm_content',
	'utm_id',
	'gclid',
	'fbclid',
	'msclkid',
	'ref',
	'_r'
]);

export function canonicalUrl(origin: string, pathname: string, search: URLSearchParams): string {
	const pairs: Array<[string, string]> = [];
	for (const [key, value] of search.entries()) {
		const k = key.trim();
		const v = value.trim();
		if (!k || !v) continue;
		if (TRACKING_PARAMS.has(k.toLowerCase())) continue;
		// Parameter internal SvelteKit (`x-sveltekit-invalidated`, `_data`) bukan
		// bagian dari identitas halaman.
		if (k.toLowerCase().startsWith('x-sveltekit') || k === '_data') continue;
		pairs.push([k, v]);
	}
	pairs.sort((a, b) => (a[0] === b[0] ? a[1].localeCompare(b[1]) : a[0].localeCompare(b[0])));

	const path = pathname === '/' ? '/' : pathname.replace(/\/+$/, '');
	const qs = new URLSearchParams(pairs).toString();
	return qs ? `${origin}${path}?${qs}` : `${origin}${path}`;
}
