/**
 * Logika format inline bersama untuk editor artikel.
 *
 * Dipakai dua editor yang tampil di portal klien & mitra:
 * - `$lib/components/rich-text-editor.svelte` (Editor Post klien, editor draf mitra)
 * - `_editor-modal.svelte` (dialog "Edit Artikel" di Ringkasan kampanye)
 *
 * Tujuannya satu perilaku: B/I/U dengan kursor saja (tanpa seleksi) mengenakan
 * format ke kata di bawah kursor, dan melepasnya bila kata itu sudah ber-tag.
 * Pembungkusan dilakukan manual dengan <b>/<i>/<u>, bukan execCommand, karena di
 * dalam heading (tebal berasal dari CSS) execCommand justru menghasilkan
 * <span style="font-weight: normal"> — sampah inline style yang bisa ikut terbit.
 */

export type ToolState = {
	bold: boolean;
	italic: boolean;
	underline: boolean;
	h2: boolean;
	h3: boolean;
	blockquote: boolean;
	ul: boolean;
	ol: boolean;
	inTable: boolean;
};

export const NO_TOOL_STATE: ToolState = {
	bold: false,
	italic: false,
	underline: false,
	h2: false,
	h3: false,
	blockquote: false,
	ul: false,
	ol: false,
	inTable: false
};

const WORD = /[\p{L}\p{N}]/u;

/** Tag inline untuk perintah B/I/U; null bila perintah bukan format inline. */
export function inlineTagsFor(cmd: string): string[] | null {
	if (cmd === 'bold') return ['b', 'strong'];
	if (cmd === 'italic') return ['i', 'em'];
	if (cmd === 'underline') return ['u'];
	return null;
}

/** Tag yang dibuat saat membungkus kata untuk perintah tersebut. */
function tagForCommand(cmd: string): 'b' | 'i' | 'u' {
	return cmd === 'bold' ? 'b' : cmd === 'italic' ? 'i' : 'u';
}

/** Rentang kata di bawah kursor (hanya bila kursor titik, tanpa seleksi). */
export function wordRangeAtCaret(body: HTMLElement): { range: Range; textNode: Text; word: string } | null {
	const sel = window.getSelection();
	if (!sel || sel.rangeCount === 0 || !sel.isCollapsed) return null;
	const range = sel.getRangeAt(0);
	const node = range.startContainer;
	if (node.nodeType !== Node.TEXT_NODE) return null;
	const textNode = node as Text;
	if (!body.contains(textNode)) return null;
	const text = textNode.data ?? '';
	const offset = range.startOffset;
	let start = offset;
	let end = offset;
	while (start > 0 && WORD.test(text[start - 1])) start--;
	while (end < text.length && WORD.test(text[end])) end++;
	if (start === end) return null; // kursor di spasi/tanda baca
	const wordRange = document.createRange();
	wordRange.setStart(textNode, start);
	wordRange.setEnd(textNode, end);
	return { range: wordRange, textNode, word: text.slice(start, end) };
}

/** Elemen inline terdekat (dalam body) yang membungkus node. */
export function inlineAncestorFor(node: Node, tags: string[], body: HTMLElement): Element | null {
	for (let el = node.parentElement; el && el !== body; el = el.parentElement) {
		if (tags.includes(el.tagName.toLowerCase())) return el;
	}
	return null;
}

/** Pindahkan kursor ke posisi teks tertentu. */
function placeCaret(node: Text, offset: number) {
	const after = document.createRange();
	after.setStart(node, offset);
	after.collapse(true);
	const sel = window.getSelection();
	sel?.removeAllRanges();
	sel?.addRange(after);
}

/**
 * Terapkan B/I/U pada pemilihan saat ini.
 *
 * @returns true  → sudah ditangani (kata dibungkus/dilepas manual); pemanggil
 *                  tinggal sync + refresh status.
 *           false → pemanggil harus menjalankan document.execCommand(cmd):
 *                  tidak ada kata di bawah kursor, atau pembungkusnya memuat
 *                  teks lain sehingga perlu dipecah oleh browser.
 */
export function applyInlineFormat(body: HTMLElement, cmd: string): boolean {
	const tags = inlineTagsFor(cmd);
	if (!tags) return false;
	const word = wordRangeAtCaret(body);
	if (!word) return false;
	const anc = inlineAncestorFor(word.textNode, tags, body);

	if (!anc) {
		const wrapper = document.createElement(tagForCommand(cmd));
		word.range.surroundContents(wrapper);
		const isi = wrapper.firstChild;
		if (isi && isi.nodeType === Node.TEXT_NODE) placeCaret(isi as Text, (isi as Text).data.length);
		else {
			const r = document.createRange();
			r.selectNodeContents(wrapper);
			r.collapse(true);
			const sel = window.getSelection();
			sel?.removeAllRanges();
			sel?.addRange(r);
		}
		return true;
	}

	// Pembungkus hanya berisi kata ini (hasil tombol editor sendiri) → lepas
	// manual; execCommand tidak selalu melepas tag saat seleksinya persis
	// seluruh isi elemen.
	if (anc.childNodes.length === 1 && anc.firstChild === word.textNode) {
		const kata = document.createTextNode(anc.textContent ?? '');
		anc.replaceWith(kata);
		placeCaret(kata, kata.data.length);
		kata.parentElement?.normalize();
		return true;
	}

	// Pembungkus memuat teks lain: pilih kata, biarkan browser memecah tag-nya.
	const sel = window.getSelection();
	sel?.removeAllRanges();
	sel?.addRange(word.range);
	return false;
}

/** Format yang berlaku di posisi kursor — untuk menyalakan tombol toolbar. */
export function readToolState(body: HTMLElement): ToolState {
	const sel = window.getSelection();
	if (!sel || sel.rangeCount === 0) return { ...NO_TOOL_STATE };
	const anchor = sel.anchorNode;
	const el = anchor
		? anchor.nodeType === Node.ELEMENT_NODE
			? (anchor as Element)
			: anchor.parentElement
		: null;
	if (!el || !body.contains(el)) return { ...NO_TOOL_STATE };

	let format = '';
	try {
		format = String(document.queryCommandValue('formatBlock') ?? '').toLowerCase();
	} catch {
		format = '';
	}
	const commandOn = (cmd: string) => {
		try {
			return document.queryCommandState(cmd);
		} catch {
			return false;
		}
	};

	return {
		// queryCommandState('bold') membaca gaya terhitung, jadi heading yang
		// tebal lewat CSS selalu melapor bold — deteksi berbasis tag lebih jujur.
		bold: Boolean(inlineAncestorFor(el, ['b', 'strong'], body)),
		italic: Boolean(inlineAncestorFor(el, ['i', 'em'], body)),
		underline: Boolean(inlineAncestorFor(el, ['u'], body)),
		h2: format === 'h2',
		h3: format === 'h3',
		blockquote: format === 'blockquote',
		ul: commandOn('insertUnorderedList'),
		ol: commandOn('insertOrderedList'),
		inTable: Boolean(el.closest('table'))
	};
}

/** Perintah yang punya status aktif (bukan undo/redo). */
export function isToggleCommand(cmd: string): boolean {
	return cmd !== 'undo' && cmd !== 'redo';
}

/** Status aktif satu tombol toolbar berdasarkan ToolState. */
export function toolCommandActive(state: ToolState, cmd: string, arg?: string): boolean {
	switch (cmd) {
		case 'bold':
			return state.bold;
		case 'italic':
			return state.italic;
		case 'underline':
			return state.underline;
		case 'formatBlock':
			return arg === 'h2' ? state.h2 : arg === 'h3' ? state.h3 : state.blockquote;
		case 'insertUnorderedList':
			return state.ul;
		case 'insertOrderedList':
			return state.ol;
		default:
			return false;
	}
}
