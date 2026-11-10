/**
 * Sanitasi HTML artikel untuk pratinjau di browser.
 *
 * Port aturan `src/lib/dom/sanitize.ts` app lama (DOMPurify + izin iframe
 * terbatas) tanpa menambah dependensi: parser bawaan browser (`DOMParser`)
 * dipakai untuk membuang elemen/atribut berbahaya.
 *
 * Aturan yang dipertahankan:
 * 1. iframe hanya boleh dari host embed tepercaya (YouTube/Vimeo/Google Maps)
 *    lewat HTTPS — daftar host sama dengan `lib/media/embed` app lama;
 * 2. `script`, `style`, `iframe` (non-tepercaya), `object`, `embed`, `form`,
 *    `input`, `textarea`, `select`, `button`, `link`, `meta`, `base` dibuang;
 * 3. semua atribut event (`on*`) dan atribut `href/src` ber-skema
 *    `javascript:`/`data:` (kecuali gambar data:) dibuang.
 */

const ALLOWED_EMBED_HOSTS = new Set([
	'www.youtube.com', 'youtube.com', 'www.youtube-nocookie.com', 'youtube-nocookie.com',
	'player.vimeo.com',
	'www.google.com', 'google.com', 'maps.google.com'
]);

const FORBID_TAGS = new Set([
	'script', 'style', 'object', 'embed', 'form', 'input', 'textarea', 'select',
	'button', 'link', 'meta', 'base', 'noscript', 'template'
]);

const URL_ATTRS = new Set(['href', 'src', 'xlink:href', 'action', 'formaction']);

// Atribut iframe yang dipertahankan. Selain ini (terutama `srcdoc`, yang
// menimpa `src` dan berjalan di origin halaman kalau sandbox memuat
// allow-same-origin) selalu dibuang.
const IFRAME_ALLOWED_ATTRS = new Set(['src', 'width', 'height', 'title', 'allow']);

// Skema URL dibaca setelah spasi dan karakter kontrol dibuang, karena browser
// mengabaikan tab/newline di dalam skema ("java\tscript:" tetap javascript:).
function normalizeUrlValue(value: string): string {
	// eslint-disable-next-line no-control-regex
	return value.replace(/[\u0000-\u0020\u007f-\u009f\u200b-\u200d\ufeff]+/g, '').toLowerCase();
}

function isAllowedEmbedSrc(src: string | null): boolean {
	if (!src) return false;
	let url: URL;
	try {
		url = new URL(src, 'https://localhost');
	} catch {
		return false;
	}
	return url.protocol === 'https:' && ALLOWED_EMBED_HOSTS.has(url.hostname.toLowerCase());
}

export function sanitizeHtml(html: string): string {
	if (typeof window === 'undefined' || !html) return '';
	const doc = new DOMParser().parseFromString(html, 'text/html');

	// Buang node berbahaya (dari belakang agar tidak mengganggu iterasi anak).
	const all = Array.from(doc.body.querySelectorAll('*'));
	for (const el of all) {
		const tag = el.tagName.toLowerCase();
		if (FORBID_TAGS.has(tag)) {
			el.remove();
			continue;
		}
		if (tag === 'iframe') {
			if (!isAllowedEmbedSrc(el.getAttribute('src'))) {
				el.remove();
				continue;
			}
			// Buang semua atribut di luar allowlist (termasuk srcdoc dan on*),
			// lalu paksa atribut pengaman — markup iframe tidak dipercaya.
			for (const attr of Array.from(el.attributes)) {
				if (!IFRAME_ALLOWED_ATTRS.has(attr.name.toLowerCase())) el.removeAttribute(attr.name);
			}
			el.setAttribute('loading', 'lazy');
			el.setAttribute('referrerpolicy', 'strict-origin-when-cross-origin');
			el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-presentation');
			el.setAttribute('allowfullscreen', '');
			continue;
		}
		for (const attr of Array.from(el.attributes)) {
			const name = attr.name.toLowerCase();
			if (name.startsWith('on')) {
				el.removeAttribute(attr.name);
				continue;
			}
			if (URL_ATTRS.has(name)) {
				const value = normalizeUrlValue(attr.value ?? '');
				if (value.startsWith('javascript:') || value.startsWith('vbscript:')) el.removeAttribute(attr.name);
				else if (value.startsWith('data:') && !value.startsWith('data:image/')) el.removeAttribute(attr.name);
			}
		}
	}
	return doc.body.innerHTML;
}
