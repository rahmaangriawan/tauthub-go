/**
 * Clipboard kaya (Google Docs / Word web) → HTML artikel bersih.
 *
 * Dipakai dua tempat:
 * - kotak "Tempel teks" di tab Unggah Dokumen (`_naskah-modal.svelte`);
 * - tempelan langsung ke Editor Post (`$lib/components/rich-text-editor.svelte`).
 *
 * Yang dibuang: seluruh atribut selain href/src/alt/colspan/rowspan (style,
 * class, id, dir, role) — inilah penyebab artikel terlihat seperti tempelan
 * dokumen di editor. Yang dipertahankan: heading, tabel, daftar, tautan, dan
 * makna format inline (tebal/miring/garis bawah/coret) yang di Google Docs
 * datang sebagai `style="font-weight:700"` pada <span>, bukan <b>.
 */

import { sanitizeHtml } from '$lib/sanitize-html';

export type TempelKaya = { title: string; html: string; plain: string };

export type ClipboardArticleOptions = {
	/** Ambil heading sebagai judul artikel lalu keluarkan dari isi (default true). */
	takeTitle?: boolean;
	/** Cadangan judul dari baris pertama bila dokumen tidak punya heading (default true). */
	fallbackTitleFromFirstLine?: boolean;
};

/** Atribut yang boleh tersisa di HTML artikel. */
const KEEP_ATTR: Record<string, true> = { href: true, src: true, alt: true, colspan: true, rowspan: true };

/**
 * Clipboard Google Docs menandai akar kopiannya dengan
 * id="docs-internal-guid-…". Dipakai hanya untuk memberi tahu klien bahwa
 * struktur aslinya terbaca — bukan syarat pemrosesan.
 */
export function isDocsClipboardHtml(html: string): boolean {
	return /docs-internal-guid|id="docs-[^"]*"|class="[^"]*\bdocs-/.test(html);
}

/**
 * Apakah clipboard HTML berasal dari aplikasi dokumen yang membawa inline style
 * ekspor. Tiga sinyal, semuanya khas ekspor dokumen:
 * - penanda Google Docs (`docs-internal-guid` / `class="docs-…"`);
 * - penanda Word/Office (`mso-`, `urn:schemas-microsoft-com:office`);
 * - pola ekspor Docs: paragraf `dir="ltr"` dengan `line-height` dan `font-family`
 *   inline. Tempelan dari halaman web biasa hampir tidak pernah memakai pola ini,
 *   jadi paste web tidak ikut dikonversi.
 */
export function isDocumentClipboardHtml(html: string): boolean {
	if (!html) return false;
	if (isDocsClipboardHtml(html)) return true;
	if (/urn:schemas-microsoft-com:office|xmlns:w=|mso-/.test(html)) return true;
	return /dir="ltr"/.test(html) && /line-height\s*:/i.test(html) && /font-family\s*:/i.test(html);
}

/**
 * Inline style yang maknanya diubah jadi tag semantik. Google Docs menulis
 * tebal/miring/garis bawah sebagai style pada <span>, sehingga tanpa langkah ini
 * format hilang begitu seluruh atribut dibuang.
 */
function semanticTagsFor(el: Element): Array<'strong' | 'em' | 'u' | 's'> {
	const style = (el.getAttribute('style') ?? '').toLowerCase();
	if (!style) return [];
	const tags: Array<'strong' | 'em' | 'u' | 's'> = [];

	const weight = /font-weight\s*:\s*([^;]+)/.exec(style)?.[1]?.trim() ?? '';
	if (/(^|\s)(bold|bolder|[6-9]00)(\s|$)/.test(weight)) tags.push('strong');

	const fontStyle = /font-style\s*:\s*([^;]+)/.exec(style)?.[1] ?? '';
	if (/\b(italic|oblique)\b/.test(fontStyle)) tags.push('em');

	const decoration = /text-decoration(?:-line)?\s*:\s*([^;]+)/.exec(style)?.[1] ?? '';
	if (/\bunderline\b/.test(decoration)) tags.push('u');
	if (/\bline-through\b/.test(decoration)) tags.push('s');

	return tags;
}

/** Bungkus isi elemen dengan tag semantik, melewati tag yang sudah membungkusnya. */
function applySemanticStyles(body: HTMLElement) {
	for (const el of Array.from(body.querySelectorAll('[style]'))) {
		const tags = semanticTagsFor(el);
		if (tags.length === 0) continue;
		let host: Element = el;
		for (const tag of tags) {
			if (host.closest(tag)) continue;
			const wrap = body.ownerDocument.createElement(tag);
			while (host.firstChild) wrap.appendChild(host.firstChild);
			host.appendChild(wrap);
			host = wrap;
		}
	}
}

/**
 * HTML clipboard (Google Docs / Word web) → HTML artikel yang rapi.
 *
 * Textarea hanya menerima plain text, padahal clipboard membawa dua flavor
 * sekaligus (text/plain + text/html) dan yang HTML memuat struktur asli:
 * `<h1>/<h2>/<h3>`, `<table>`, `<ul>/<ol>`. Fungsi ini mempertahankan struktur
 * itu dan membuang sampah ekspor Docs:
 * - `<meta>`/`<style>`/`<colgroup>` dan pembungkus
 *   `<b style="font-weight:normal" id="docs-internal-guid-…">`;
 * - seluruh atribut kecuali href/src/alt/colspan/rowspan (style, class, id,
 *   dir, role) — penyebab tampilan "tempelan" yang berantakan di editor;
 * - NBSP, zero-width, dan blok kosong.
 *
 * Heading dipertahankan PERSIS seperti di Docs: "Judul 1" (`<h1>`) tetap `<h1>`,
 * "Judul 2" tetap `<h2>`, dan seterusnya. Judul artikel diambil dengan dua pola:
 * `<h1 class="title">` (gaya "Judul" dokumen di Docs), atau — bila tidak ada —
 * H1 pertama yang menjadi blok berisi pertama (pola umum "Judul 1" sebagai judul
 * artikel). Heading yang diangkat menjadi judul dikeluarkan dari isi.
 *
 * `takeTitle: false` membiarkan heading tetap di isi (dipakai Editor Post saat
 * kolom judul sudah terisi); `fallbackTitleFromFirstLine: false` mematikan
 * cadangan "baris pertama jadi judul" untuk pemakaian yang hanya ingin heading.
 */
export function clipboardHtmlToArticle(rawHtml: string, opts: ClipboardArticleOptions = {}): TempelKaya | null {
	const takeTitle = opts.takeTitle ?? true;
	const fallbackTitle = opts.fallbackTitleFromFirstLine ?? true;
	if (typeof DOMParser === 'undefined' || !rawHtml.trim()) return null;
	const doc = new DOMParser().parseFromString(rawHtml, 'text/html');
	const body = doc.body;
	if (!body) return null;

	body.querySelectorAll('meta, style, link, title, colgroup, col, script, noscript').forEach((el) => el.remove());

	// Pembungkus ekspor Docs: <b style="font-weight: normal" id="docs-internal-guid-…">.
	// Dibuka dulu supaya penilaian "H1 pertama di awal dokumen" melihat sibling
	// yang sebenarnya.
	body.querySelectorAll('b[style*="font-weight"], b[id*="docs-internal-guid"]').forEach((el) => {
		el.replaceWith(...Array.from(el.childNodes));
	});

	// Format inline (Docs = style pada <span>) jadi tag semantik sebelum atribut
	// dibuang.
	applySemanticStyles(body);

	// Judul artikel, dua pola Docs:
	// 1. Gaya "Judul" (<h1 class="title">) — judul dokumen, selalu dipakai.
	// 2. Gaya "Judul 1" (<h1>) yang ditulis sebagai judul artikel — dipakai bila
	//    ia blok berisi PERTAMA (tidak ada isi lain sebelumnya). H1 berikutnya
	//    dibiarkan di badan sebagai sub-bagian.
	let title = '';
	if (takeTitle) {
		const judulNode = body.querySelector('h1.title, h1[class*="title"]');
		if (judulNode) {
			title = (judulNode.textContent ?? '').replace(/\s+/g, ' ').trim();
			judulNode.remove();
		}
		if (!title) {
			const h1Pertama = body.querySelector('h1');
			if (h1Pertama) {
				let adaIsiSebelumnya = false;
				for (let s = h1Pertama.previousElementSibling; s; s = s.previousElementSibling) {
					if ((s.textContent ?? '').trim()) {
						adaIsiSebelumnya = true;
						break;
					}
				}
				const teks = (h1Pertama.textContent ?? '').replace(/\s+/g, ' ').trim();
				if (!adaIsiSebelumnya && teks && teks.length <= 200) {
					title = teks;
					h1Pertama.remove();
				}
			}
		}
	}

	for (const el of Array.from(body.querySelectorAll('*'))) {
		for (const attr of Array.from(el.attributes)) {
			if (!KEEP_ATTR[attr.name.toLowerCase()]) el.removeAttribute(attr.name);
		}
	}

	const walker = doc.createTreeWalker(body, NodeFilter.SHOW_TEXT);
	const teksNodes: Text[] = [];
	while (walker.nextNode()) teksNodes.push(walker.currentNode as Text);
	for (const node of teksNodes) {
		node.data = node.data
			.replace(/[\u200B-\u200D\uFEFF]/g, '')
			.replace(/\u00A0/g, ' ')
			.replace(/[ \t]{2,}/g, ' ');
	}

	// <span> tanpa atribut hanyalah sisa pembungkus ekspor Docs — dibuka.
	body.querySelectorAll('span').forEach((el) => {
		if (el.attributes.length === 0) el.replaceWith(...Array.from(el.childNodes));
	});

	// <li><p>isi</p></li> (pola daftar Docs) → <li>isi</li>. Paragraf yang
	// berbagi <li> dengan sub-daftar dibiarkan supaya sub-daftar tetap utuh.
	body.querySelectorAll('li > p').forEach((p) => {
		const li = p.parentElement;
		if (li && li.tagName === 'LI' && li.childElementCount === 1) p.replaceWith(...Array.from(p.childNodes));
	});

	body.querySelectorAll('p, h2, h3, h4, li, td, th, tr, ul, ol, table').forEach((el) => {
		const adaMedia = el.querySelector('img, iframe, br, td, th, li');
		if (!adaMedia && !(el.textContent ?? '').trim()) el.remove();
	});
	// <tr> kosong yang tersisa (semua selnya terhapus) ikut dibuang.
	body.querySelectorAll('tr').forEach((tr) => {
		if (!tr.querySelector('td, th')) tr.remove();
	});

	const html = sanitizeHtml(body.innerHTML).trim();

	// Teks biasa per blok, dipakai untuk mengisi kotak tempel (perilaku lama).
	// Hanya blok terdalam yang diambil agar isi <li><p>…</p></li> tidak dobel,
	// dan tiap blok jadi satu baris (bukan hasil textContent yang menempel).
	const BLOK = 'p, h2, h3, h4, li, th, td';
	const plain = Array.from(body.querySelectorAll(BLOK))
		.filter((el) => !el.querySelector(BLOK))
		.map((el) => (el.textContent ?? '').replace(/\u00A0/g, ' ').replace(/\s+/g, ' ').trim())
		.filter(Boolean)
		.join('\n');

	if (!html || !plain) return null;
	if (!title && fallbackTitle) {
		const barisPertama = plain.split('\n')[0] ?? '';
		if (barisPertama.length > 0 && barisPertama.length <= 120) title = barisPertama;
	}
	return { title, html, plain };
}
