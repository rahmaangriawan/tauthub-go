<script lang="ts">
	// Editor teks kaya untuk halaman editor dokumen — dipisahkan dari toolbar
	// contentEditable _editor-modal.svelte (Taut Pintar Writer) supaya bisa
	// dipakai di luar modal: perintah format, pustaka gambar AI (bila scope
	// brand+campaign+media tersedia) atau unggah gambar ke /brands/:id/assets,
	// dialog tautan, dan sinkronisasi HTML dua arah.
	import { CircleAlert, Image, Link, List, ListOrdered, LoaderCircle, Quote, RotateCcw, RotateCw, Trash2, Upload, X } from '@lucide/svelte';
	import type { Component } from 'svelte';
	import { apiRequest } from '$lib/auth';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import { tr } from '$lib/i18n/phrases';
	import MediaLibraryPicker from '$lib/components/media-library-picker.svelte';

	let {
		value = $bindable(''),
		brandId = '',
		campaignId = '',
		mediaId = '',
		articleId = null,
		disabled = false,
		minHeight = 420,
		/** Mode kolom penuh: root jadi kolom flex, isinya memakai sisa tinggi
		 *  kolom (tanpa max-height 60vh) — scroll diatur oleh kolom pemanggil.
		 *  Dipakai modal editor draf mitra; pemanggil lain tidak berubah. */
		fill = false,
		ariaLabel,
		uploadImage,
		stats = $bindable({ words: 0, links: 0, images: 0 })
	}: {
		value?: string;
		brandId?: string;
		/** Scope pustaka gambar AI. Kosong = tombol gambar jatuh ke modal unggah. */
		campaignId?: string;
		mediaId?: string;
		articleId?: string | null;
		disabled?: boolean;
		minHeight?: number;
		fill?: boolean;
		ariaLabel?: string;
		/**
		 * Pengunggah gambar khusus. Bila diisi, editor memakai fungsi ini dan
		 * tidak menyentuh aset brand (dipakai halaman mitra lewat unggahan draf).
		 */
		uploadImage?: (file: File) => Promise<string>;
		/** Statistik isi (kata, tautan aktif, gambar) — dihitung saat konten berubah. */
		stats?: { words: number; links: number; images: number };
	} = $props();

	let bodyEl = $state<HTMLDivElement | null>(null);
	let imageInput = $state<HTMLInputElement | undefined>(undefined);
	let imageBusy = $state(false);
	let imageError = $state('');

	// Gambar yang sedang dipilih di dalam artikel. Tanpa ini, menghapus gambar
	// hanya bisa lewat caret + Backspace — tidak kasat mata dan sering gagal.
	let rootEl = $state<HTMLElement | null>(null);
	let selectedImg = $state<HTMLImageElement | null>(null);
	let imgBarPos = $state<{ top: number; left: number }>({ top: 0, left: 0 });

	function clearImageSelection() {
		selectedImg?.classList.remove('rte-img-selected');
		selectedImg = null;
	}

	function selectImage(img: HTMLImageElement) {
		clearImageSelection();
		selectedImg = img;
		img.classList.add('rte-img-selected');
		placeImageBar();
	}

	function placeImageBar() {
		if (!selectedImg || !rootEl) return;
		const r = selectedImg.getBoundingClientRect();
		const root = rootEl.getBoundingClientRect();
		imgBarPos = {
			top: Math.max(4, r.top - root.top - 36),
			left: Math.max(4, r.left - root.left)
		};
	}

	function deleteSelectedImage() {
		const img = selectedImg;
		if (!img || !bodyEl?.contains(img)) {
			clearImageSelection();
			return;
		}
		img.remove();
		clearImageSelection();
		sync();
		commitHistory();
	}
	let mode = $state<'rich' | 'html'>('rich');
		// Saat kembali dari mode HTML, elemen contenteditable dibuat ulang oleh
		// {#if} — isinya harus diisi ulang dari value.
		let pendingRender = $state(false);
		let linkDialog = $state<{ open: boolean; url: string; text: string; isEdit: boolean; rel: string; error: string }>({
			open: false,
			url: '',
			text: '',
			isEdit: false,
			rel: 'dofollow',
			error: ''
		});
		// Keterangan yang tampil mengikuti radio rel yang dipilih, ditulis dengan
		// bahasa awam: apa efeknya + kapan sebaiknya dipakai.
		const REL_HINTS: Record<string, string> = {
			dofollow: tr('Tautan biasa. Google ikut membaca dan menilai tautan ini, jadi bisa membantu SEO. Pakai untuk tautan yang wajar ada di dalam artikel.'),
			nofollow: tr('Google tidak menghitung tautan ini. Pakai kalau tautannya bukan untuk mendongkrak SEO, atau kamu tidak ingin dianggap merekomendasikan halaman tujuan.'),
			sponsored: tr('Untuk tautan berbayar: iklan atau kerja sama. Google mewajibkan penanda ini. Pakai kalau tautannya disponsori.')
		};
		let savedRange: Range | null = null;
		let imageModalOpen = $state(false);
		let mediaPickerOpen = $state(false);
		let imageDragOver = $state(false);

		$effect(() => {
			if (!imageModalOpen) return;
			const onKey = (e: KeyboardEvent) => {
				if (e.key === 'Escape' && !imageBusy) imageModalOpen = false;
			};
			const onPaste = (e: ClipboardEvent) => {
				if (imageBusy) return;
				const file = Array.from(e.clipboardData?.files ?? []).find((f) => f.type.startsWith('image/'));
				if (file) {
					e.preventDefault();
					void pickImage(file);
				}
			};
			window.addEventListener('keydown', onKey);
			window.addEventListener('paste', onPaste);
			return () => {
				window.removeEventListener('keydown', onKey);
				window.removeEventListener('paste', onPaste);
			};
		});

	// Isi awal & saat pergantian sumber luar (mis. pulihkan versi).
	let lastExternal = '';
	$effect(() => {
		if (value !== lastExternal && bodyEl && mode === 'rich') {
			clearImageSelection();
			bodyEl.innerHTML = sanitizeHtml(value);
			lastExternal = value;
			resetHistory();
			// Konten bisa berubah dari luar (mis. sisip gambar per H2 / pulihkan
			// versi) — statistik harus ikut segar, bukan hanya saat dipanggil sync().
			publishStats();
		}
	});

	$effect(() => {
		if (pendingRender && bodyEl) {
			bodyEl.innerHTML = sanitizeHtml(value);
			lastExternal = value;
			pendingRender = false;
			resetHistory();
		}
	});

	function sync() {
		if (!bodyEl) return;
		// execCommand kadang meninggalkan atribut style kosong (mis. saat melepas
		// <i> di dalam <u>) — dibuang supaya HTML artikel bersih.
		bodyEl.querySelectorAll('[style=""]').forEach((el) => el.removeAttribute('style'));
		lastExternal = bodyEl.innerHTML;
		value = bodyEl.innerHTML;
		publishStats();
		scheduleHistoryCommit();
	}

	// Statistik isi untuk footer pemanggil: kata, tautan aktif, gambar.
	function publishStats() {
		if (!bodyEl) return;
		const text = (bodyEl.innerText ?? '').replace(/\s+/g, ' ').trim();
		stats.words = text ? text.split(' ').length : 0;
		stats.links = bodyEl.querySelectorAll('a[href]').length;
		stats.images = bodyEl.querySelectorAll('img').length;
	}

	$effect(() => {
		// Setelah konten terisi/diganti dari luar, statistik ikut diperbarui.
		if (bodyEl) publishStats();
	});

	/**
	 * Rentang kata di bawah kursor (bila kursor hanya titik, tanpa seleksi).
	 * Dipakai B/I/U supaya format dikenakan ke kata tersebut, bukan sekadar
	 * menyiapkan format untuk ketikan berikutnya.
	 */
	function wordRangeAtCaret(): { range: Range; textNode: Text; word: string } | null {
		const body = bodyEl;
		const sel = window.getSelection();
		if (!body || !sel || sel.rangeCount === 0 || !sel.isCollapsed) return null;
		const range = sel.getRangeAt(0);
		const node = range.startContainer;
		if (node.nodeType !== Node.TEXT_NODE) return null;
		const textNode = node as Text;
		const text = textNode.data ?? '';
		const offset = range.startOffset;
		const WORD = /[\p{L}\p{N}]/u;
		let start = offset;
		let end = offset;
		while (start > 0 && WORD.test(text[start - 1])) start--;
		while (end < text.length && WORD.test(text[end])) end++;
		if (start === end) return null; // kursor di spasi/tanda baca — perilaku lama
		const wordRange = document.createRange();
		wordRange.setStart(textNode, start);
		wordRange.setEnd(textNode, end);
		return { range: wordRange, textNode, word: text.slice(start, end) };
	}

	/** Elemen inline (b/strong, i/em, u) terdekat yang membungkus node. */
	function inlineAncestor(node: Node, tags: string[]): Element | null {
		const body = bodyEl;
		for (let el = node.parentElement; el && el !== body; el = el.parentElement) {
			if (tags.includes(el.tagName.toLowerCase())) return el;
		}
		return null;
	}

	function exec(cmd: string, arg?: string) {
		const body = bodyEl;
		if (!body || disabled) return;
		body.focus();
		const inlineTags = cmd === 'bold' ? ['b', 'strong'] : cmd === 'italic' ? ['i', 'em'] : cmd === 'underline' ? ['u'] : null;
		if (inlineTags) {
			const word = wordRangeAtCaret();
			if (word) {
				const anc = inlineAncestor(word.textNode, inlineTags);
				if (!anc) {
					// Kata belum ber-tag: bungkus langsung. execCommand() tidak dipakai
					// di sini karena di dalam heading ia menghasilkan
					// <span style="font-weight: normal"> (tebal heading berasal dari
					// CSS), sampah inline style yang bisa ikut terbit.
					const tag = cmd === 'bold' ? 'b' : cmd === 'italic' ? 'i' : 'u';
					const wrapper = document.createElement(tag);
					word.range.surroundContents(wrapper);
					const after = document.createRange();
					const isiWrapper = wrapper.firstChild;
					if (isiWrapper && isiWrapper.nodeType === Node.TEXT_NODE) {
						// Kursor harus berada DI DALAM teks, bukan di elemen pembungkus:
						// kalau tidak, klik format berikutnya tidak menemukan kata.
						after.setStart(isiWrapper, (isiWrapper as Text).data.length);
					} else {
						after.selectNodeContents(wrapper);
					}
					after.collapse(true);
					const sel = window.getSelection();
					sel?.removeAllRanges();
					sel?.addRange(after);
					sync();
					refreshToolState();
					commitHistory();
					return;
				}
				// Pembungkus hanya berisi kata ini (hasil tombol toolbar sendiri) →
				// lepas manual. execCommand('bold') tidak selalu melepas tag saat
				// seleksinya persis seluruh isi elemen tersebut.
				if (anc.childNodes.length === 1 && anc.firstChild === word.textNode) {
					const parent = anc.parentNode;
					const kata = document.createTextNode(anc.textContent ?? '');
					anc.replaceWith(kata);
					const after = document.createRange();
					after.setStart(kata, kata.data.length);
					after.collapse(true);
					const sel = window.getSelection();
					sel?.removeAllRanges();
					sel?.addRange(after);
					parent?.normalize();
					sync();
					refreshToolState();
					commitHistory();
					return;
				}
				// Pembungkus memuat teks lain: pilih kata lalu biarkan execCommand
				// memecah tag-nya.
				const sel = window.getSelection();
				sel?.removeAllRanges();
				sel?.addRange(word.range);
			}
		}
		document.execCommand(cmd, false, arg);
		const after = window.getSelection();
		if (inlineTags && after && after.rangeCount > 0 && !after.isCollapsed) after.collapseToEnd();
		sync();
		refreshToolState();
		// Tindakan dari toolbar selalu jadi satu langkah riwayat.
		commitHistory();
	}

	// ---------- riwayat suntingan (undo/redo) ----------
	// Tumpukan snapshot HTML, bukan execCommand: perintah bawaan browser rusak
	// begitu isi editor ditulis ulang dari luar (sisip gambar per H2, pulihkan
	// versi, hapus gambar dari bar aksi).
	let history: string[] = [];
	let historyIndex = $state(-1);
	let applyingHistory = false;
	let snapshotTimer: number | null = null;
	const HISTORY_LIMIT = 80;

	const canUndo = $derived(historyIndex > 0);
	const canRedo = $derived(historyIndex >= 0 && historyIndex < history.length - 1);

	function resetHistory() {
		if (!bodyEl) return;
		history = [bodyEl.innerHTML];
		historyIndex = 0;
	}

	// "commitHistory": catat keadaan sekarang sebagai langkah baru.
	function commitHistory() {
		if (!bodyEl || applyingHistory) return;
		const html = bodyEl.innerHTML;
		if (historyIndex >= 0 && history[historyIndex] === html) return;
		history = history.slice(0, historyIndex + 1);
		history.push(html);
		if (history.length > HISTORY_LIMIT) history = history.slice(history.length - HISTORY_LIMIT);
		historyIndex = history.length - 1;
	}

	// Ketikan: gabungkan jeda mengetik jadi satu langkah (debounce).
	function scheduleHistoryCommit() {
		if (snapshotTimer) window.clearTimeout(snapshotTimer);
		snapshotTimer = window.setTimeout(() => {
			snapshotTimer = null;
			commitHistory();
		}, 500);
	}

	function applyHistory(index: number) {
		if (!bodyEl || index < 0 || index >= history.length) return;
		applyingHistory = true;
		historyIndex = index;
		clearImageSelection();
		bodyEl.innerHTML = history[index];
		lastExternal = bodyEl.innerHTML;
		value = bodyEl.innerHTML;
		publishStats();
		applyingHistory = false;
	}

	function undoEdit() {
		if (canUndo) applyHistory(historyIndex - 1);
	}

	function redoEdit() {
		if (canRedo) applyHistory(historyIndex + 1);
	}

	// Pintasan: Ctrl/Cmd+Z urungkan, Ctrl/Cmd+Shift+Z atau Ctrl+Y ulangi, dan
	// Delete/Backspace menghapus gambar yang sedang dipilih.
	function onBodyKeydown(e: KeyboardEvent) {
		const mod = e.metaKey || e.ctrlKey;
		if (mod && e.key.toLowerCase() === 'z') {
			e.preventDefault();
			if (e.shiftKey) redoEdit();
			else undoEdit();
			return;
		}
		if (mod && e.key.toLowerCase() === 'y') {
			e.preventDefault();
			redoEdit();
			return;
		}
		if ((e.key === 'Delete' || e.key === 'Backspace') && selectedImg) {
			e.preventDefault();
			deleteSelectedImage();
		}
	}

	const tools: Array<{ cmd: string; label?: string; icon?: Component; arg?: string; name: string }> = [
		{ cmd: 'bold', label: 'B', name: 'Tebal' },
		{ cmd: 'italic', label: 'I', name: 'Miring' },
		{ cmd: 'underline', label: 'U', name: 'Garis bawah' },
		{ cmd: 'formatBlock', label: 'H2', arg: 'h2', name: 'Judul bagian' },
		{ cmd: 'formatBlock', label: 'H3', arg: 'h3', name: 'Sub-judul' },
		{ cmd: 'formatBlock', arg: 'blockquote', icon: Quote, name: 'Kutipan' },
		{ cmd: 'insertUnorderedList', icon: List, name: 'Daftar poin' },
		{ cmd: 'insertOrderedList', icon: ListOrdered, name: 'Daftar bernomor' },
		{ cmd: 'undo', icon: RotateCcw, name: 'Urungkan' },
		{ cmd: 'redo', icon: RotateCw, name: 'Ulangi' }
	];

	/** Format yang berlaku di posisi kursor — dipakai menyalakan tombol toolbar. */
	type ToolState = {
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
	const TOOL_STATE_OFF: ToolState = {
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
	let toolState = $state<ToolState>({ ...TOOL_STATE_OFF });

	/** Tombol undo/redo bukan status format — jangan diberi penanda aktif. */
	const isToggleTool = (t: (typeof tools)[number]) => t.cmd !== 'undo' && t.cmd !== 'redo';

	function toolActive(t: (typeof tools)[number]): boolean {
		switch (t.cmd) {
			case 'bold':
				return toolState.bold;
			case 'italic':
				return toolState.italic;
			case 'underline':
				return toolState.underline;
			case 'formatBlock':
				return t.arg === 'h2' ? toolState.h2 : t.arg === 'h3' ? toolState.h3 : toolState.blockquote;
			case 'insertUnorderedList':
				return toolState.ul;
			case 'insertOrderedList':
				return toolState.ol;
			default:
				return false;
		}
	}

	/**
	 * Selaraskan tombol toolbar dengan format pada posisi kursor/seleksi.
	 * queryCommandState/queryCommandValue memang deprecated, tapi masih satu-
	 * satunya cara membaca format aktif di contenteditable tanpa library.
	 * Diperbarui saat seleksi berubah, saat tombol dipakai, dan saat klik/ketik.
	 */
	function refreshToolState() {
		const body = bodyEl;
		if (!body || disabled || mode === 'html') {
			toolState = { ...TOOL_STATE_OFF };
			return;
		}
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0) {
			toolState = { ...TOOL_STATE_OFF };
			return;
		}
		const anchor = sel.anchorNode;
		const el = anchor
			? anchor.nodeType === Node.ELEMENT_NODE
				? (anchor as Element)
				: anchor.parentElement
			: null;
		if (!el || !body.contains(el)) {
			toolState = { ...TOOL_STATE_OFF };
			return;
		}
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
		// queryCommandState('bold') membaca GAYA terhitung, sehingga heading yang
		// memang tebal lewat CSS selalu melaporkan bold. Untuk format inline,
		// deteksi berbasis tag lebih jujur: hanya <b>/<strong>, <i>/<em>, <u>.
		const punyaTagInduk = (tags: string[]) => {
			for (let node: Node | null = el; node && node !== body; node = node.parentNode) {
				if (node.nodeType === Node.ELEMENT_NODE && tags.includes((node as Element).tagName.toLowerCase())) {
					return true;
				}
			}
			return false;
		};
		toolState = {
			bold: punyaTagInduk(['b', 'strong']),
			italic: punyaTagInduk(['i', 'em']),
			underline: punyaTagInduk(['u']),
			h2: format === 'h2',
			h3: format === 'h3',
			blockquote: format === 'blockquote',
			ul: commandOn('insertUnorderedList'),
			ol: commandOn('insertOrderedList'),
			inTable: Boolean(el.closest('table'))
		};
	}

	$effect(() => {
		if (!bodyEl || disabled || mode !== 'rich') return;
		const sync = () => refreshToolState();
		document.addEventListener('selectionchange', sync);
		window.addEventListener('keyup', sync);
		window.addEventListener('mouseup', sync);
		sync();
		return () => {
			document.removeEventListener('selectionchange', sync);
			window.removeEventListener('keyup', sync);
			window.removeEventListener('mouseup', sync);
		};
	});

	function openImageModal() {
		if (!bodyEl || disabled || mode === 'html') return;
		const sel = window.getSelection();
		savedRange = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).cloneRange() : null;
		// Dengan scope artikel lengkap → pustaka gambar AI. Tanpa scope
		// (mis. halaman editor dokumen mandiri) → modal unggah berkas.
		if (brandId && campaignId && mediaId) {
			mediaPickerOpen = true;
			return;
		}
		imageError = '';
		imageDragOver = false;
		imageModalOpen = true;
	}

	/** Sisipkan gambar pada seleksi yang disimpan (atau di kursor saat ini). */
	function insertImageUrl(url: string) {
		if (!bodyEl || !url) return;
		bodyEl.focus();
		const sel = window.getSelection();
		if (sel && savedRange) {
			sel.removeAllRanges();
			try {
				sel.addRange(savedRange);
			} catch {
				/* range basi */
			}
		}
		document.execCommand('insertImage', false, url);
		sync();
		commitHistory();
	}

	// Tempel/tarik gambar langsung ke isi editor tetap didukung (unggah ke
	// /assets) tanpa membuka modal.
	function onBodyPaste(e: ClipboardEvent) {
		if (disabled || mode !== 'rich') return;
		const file = Array.from(e.clipboardData?.files ?? []).find((f) => f.type.startsWith('image/'));
		if (!file) return;
		e.preventDefault();
		void pickImage(file);
	}

	function onBodyDrop(e: DragEvent) {
		if (disabled || mode !== 'rich') return;
		const file = e.dataTransfer?.files?.[0];
		if (!file || !file.type.startsWith('image/')) return;
		e.preventDefault();
		const sel = window.getSelection();
		savedRange = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).cloneRange() : null;
		void pickImage(file);
	}

	async function kompresWebp(file: File): Promise<File> {
		if (!file.type.startsWith('image/') || file.type === 'image/gif' || file.type === 'image/webp') return file;
		try {
			const bitmap = await createImageBitmap(file);
			const maxSide = 1600;
			const skala = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
			const w = Math.round(bitmap.width * skala);
			const h = Math.round(bitmap.height * skala);
			const canvas = document.createElement('canvas');
			canvas.width = w;
			canvas.height = h;
			const ctx = canvas.getContext('2d');
			if (!ctx) return file;
			ctx.drawImage(bitmap, 0, 0, w, h);
			bitmap.close?.();
			const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.85));
			if (!blob || blob.size >= file.size) return file;
			return new File([blob], file.name.replace(/\.[^.]+$/, '') + '.webp', { type: 'image/webp' });
		} catch {
			return file;
		}
	}

	async function pickImage(file: File) {
		if (!file || imageBusy || !file.type.startsWith('image/')) return;
		imageBusy = true;
		imageError = '';
		try {
			const readyFile = await kompresWebp(file);
			if (uploadImage) {
				const url = await uploadImage(readyFile);
				if (!url) throw new Error();
				insertImageUrl(url);
			} else {
				const form = new FormData();
				form.append('file', readyFile);
				const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
				if (!res.ok) throw new Error();
				const data = (await res.json()) as { url?: string };
				if (data.url) insertImageUrl(data.url);
			}
			imageModalOpen = false;
		} catch {
			imageError = tr('Gagal mengunggah gambar. Periksa koneksi lalu coba lagi.');
		} finally {
			imageBusy = false;
			if (imageInput) imageInput.value = '';
		}
	}

	/** Anchor pada seleksi saat ini (bila ada). */
	function closestAnchor(): HTMLAnchorElement | null {
		const body = bodyEl;
		if (!body) return null;
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0) return null;
		for (const node of [sel.anchorNode, sel.focusNode]) {
			const el = node instanceof Element ? node : (node?.parentElement ?? null);
			const anchor = el?.closest('a');
			if (anchor && body.contains(anchor)) return anchor as HTMLAnchorElement;
		}
		return null;
	}

	function openLinkDialog() {
		if (!bodyEl || disabled) return;
		const sel = window.getSelection();
		savedRange = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).cloneRange() : null;
		const anchor = closestAnchor();
		// Auto-deteksi: teks yang sedang diseleksi dipakai sebagai teks tautan.
		const selectedText = savedRange && !savedRange.collapsed ? savedRange.toString().trim() : '';
		// rel tautan yang sedang diedit: sponsored menang, lalu nofollow, sisanya
		// dofollow (tanpa rel).
		const relAttr = (anchor?.getAttribute('rel') ?? '').toLowerCase();
		const rel: 'dofollow' | 'nofollow' | 'sponsored' = relAttr.includes('sponsored')
			? 'sponsored'
			: relAttr.includes('nofollow')
				? 'nofollow'
				: 'dofollow';
		linkDialog = {
			open: true,
			url: anchor?.getAttribute('href') ?? '',
			text: anchor ? (anchor.textContent ?? '').trim() : selectedText,
			isEdit: Boolean(anchor),
			rel,
			error: ''
		};
	}

	function closeLinkDialog() {
		linkDialog = { ...linkDialog, open: false, error: '' };
	}

	function applyRel(a: HTMLAnchorElement, rel: string) {
		if (rel === 'dofollow') a.removeAttribute('rel');
		else a.setAttribute('rel', rel);
	}

	function insertLinkNode(a: HTMLAnchorElement, range: Range | null) {
		if (range) {
			if (!range.collapsed) range.deleteContents();
			range.insertNode(a);
		} else {
			bodyEl?.appendChild(a);
		}
		// Letakkan caret tepat setelah tautan yang baru disisipkan.
		const sel = window.getSelection();
		if (sel) {
			const after = document.createRange();
			after.setStartAfter(a);
			after.collapse(true);
			sel.removeAllRanges();
			sel.addRange(after);
		}
	}

	function applyLink() {
		const body = bodyEl;
		if (!body) {
			closeLinkDialog();
			return;
		}
		const url = linkDialog.url.trim();
		const text = linkDialog.text.trim();
		body.focus();
		const sel = window.getSelection();
		if (sel && savedRange) {
			sel.removeAllRanges();
			try {
				sel.addRange(savedRange);
			} catch {
				/* range basi */
			}
		}
		const existing = closestAnchor();
		const range = sel && sel.rangeCount > 0 ? sel.getRangeAt(0) : null;

		// URL kosong = lepas tautan dari teks terpilih.
		if (!url) {
			if (existing) {
				const r = document.createRange();
				r.selectNodeContents(existing);
				sel?.removeAllRanges();
				sel?.addRange(r);
				document.execCommand('unlink', false);
			}
			sync();
			closeLinkDialog();
			return;
		}

		const hasSelection = Boolean(range && !range.collapsed);
		// Butuh teks tautan: dari field, atau dari teks yang diseleksi di artikel.
		if (!text && !hasSelection && !existing) {
			linkDialog = { ...linkDialog, error: tr('Blok teks di artikel dulu, atau isi Teks Tautan.') };
			return;
		}

		let target: HTMLAnchorElement | null = null;
		if (existing) {
			// Seleksi berada di dalam tautan yang sudah ada → perbarui tautan itu
			// (jangan membuat tautan bersarang). Teks hanya diganti bila seleksi
			// mencakup seluruh teks tautan.
			const fullText = (existing.textContent ?? '').trim();
			const coversWhole = !range || range.collapsed || range.toString().trim() === fullText;
			existing.setAttribute('href', url);
			if (text && text !== fullText && coversWhole) existing.textContent = text;
			target = existing;
		} else {
			// Buat tautan baru — membungkus seleksi atau menyisipkan di caret.
			const a = document.createElement('a');
			a.setAttribute('href', url);
			a.textContent = text || (range ? range.toString() : '');
			insertLinkNode(a, range);
			target = a;
		}
		if (target) applyRel(target, linkDialog.rel);
		sync();
		closeLinkDialog();
	}

	function handleBodyClick(e: MouseEvent) {
		const body = bodyEl;
		const el = e.target instanceof Element ? e.target : null;
		if (!body || !el) return;

		const img = el.closest('img');
		if (img && body.contains(img)) {
			e.preventDefault();
			selectImage(img as HTMLImageElement);
			return;
		}
		clearImageSelection();

		const anchor = el.closest('a');
		if (!anchor || !body.contains(anchor)) return;
		e.preventDefault();
		const range = document.createRange();
		range.selectNodeContents(anchor);
		const sel = window.getSelection();
		sel?.removeAllRanges();
		sel?.addRange(range);
	}
</script>

<div class="rte" class:rte-fill={fill} bind:this={rootEl}>
	<div class="rte-toolbar flex flex-wrap items-center gap-1 rounded-lg border p-1" role="toolbar" aria-label="Format teks">
		{#each tools as t, i (t.cmd + (t.arg ?? ''))}
			{@const ToolIcon = t.icon}
			{#if fill && (i === 3 || i === 5)}
				<span class="rte-sep" aria-hidden="true"></span>
			{/if}
			<button
				class="rte-tool"
				class:rte-tool-active={isToggleTool(t) && toolActive(t)}
				type="button"
				title={t.name}
				aria-label={t.name}
				aria-pressed={isToggleTool(t) ? toolActive(t) : undefined}
				onmousedown={(e: MouseEvent) => e.preventDefault()}
				disabled={disabled ||
					mode === 'html' ||
					(t.cmd === 'undo' && !canUndo) ||
					(t.cmd === 'redo' && !canRedo)}
				onclick={() => {
					if (t.cmd === 'undo') return undoEdit();
					if (t.cmd === 'redo') return redoEdit();
					exec(t.cmd, t.arg);
				}}
			>
				{#if ToolIcon}<ToolIcon size={15} />{:else}{t.label}{/if}
			</button>
		{/each}
		{#if mode === 'rich' && toolState.inTable}
			<!-- Penanda posisi: tabel tidak punya tombol toolbar, jadi statusnya
			     ditunjukkan chip ini saat kursor berada di dalam tabel. -->
			<span class="rte-table-hint" title="Kursor berada di dalam tabel">Tabel</span>
		{/if}
		<input
			bind:this={imageInput}
			type="file"
			accept="image/*"
			class="hidden"
			onchange={(e) => {
				const f = (e.currentTarget as HTMLInputElement).files?.[0];
				if (f) void pickImage(f);
			}}
		/>
			<button
				class="rte-tool"
				type="button"
				title={tr("Sisipkan gambar")}
				aria-label={tr("Sisipkan gambar")}
				disabled={disabled || imageBusy || mode === 'html'}
				onclick={openImageModal}
			>
				{#if imageBusy}<LoaderCircle size={15} class="animate-spin" />{:else}<Image size={15} />{/if}
			</button>
		<button
			class="rte-tool"
			type="button"
			title="Sisipkan tautan"
			aria-label="Sisipkan tautan"
			disabled={disabled || mode === 'html'}
			onmousedown={(e: MouseEvent) => e.preventDefault()}
			onclick={openLinkDialog}
		><Link size={15} /></button>
		<span class="flex-1"></span>
		<button
			class="rte-tool rte-tool-toggle"
			class:rte-tool-active={mode === 'html'}
			type="button"
			aria-pressed={mode === 'html'}
			title={mode === 'html' ? 'Kembali ke editor visual' : 'Edit HTML mentah'}
			onclick={() => {
				if (mode === 'html') {
					pendingRender = true;
					mode = 'rich';
				} else {
					mode = 'html';
				}
			}}
		>
			<span class="font-mono text-[11px]">&lt;/&gt;</span>
		</button>
	</div>

	{#if mode === 'rich'}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			bind:this={bodyEl}
			class="rte-body prose prose-sm overflow-y-auto rounded-lg border p-3 text-sm"
			style={`min-height: ${minHeight}px;${fill ? '' : ' max-height: 60vh;'}`}
			contenteditable={!disabled}
			role="textbox"
			tabindex="0"
			aria-label={ariaLabel ?? 'Isi artikel'}
			oninput={sync}
			onclick={handleBodyClick}
			onkeydown={onBodyKeydown}
			onscroll={placeImageBar}
			onpaste={onBodyPaste}
			ondragover={(e) => { if (!disabled && mode === 'rich') e.preventDefault(); }}
			ondrop={onBodyDrop}
		></div>

		{#if selectedImg}
			<!-- Bar aksi gambar: muncul saat gambar diklik di dalam artikel. -->
			<div class="rte-imgbar" style="top: {imgBarPos.top}px; left: {imgBarPos.left}px;">
				<button class="rte-imgbar-btn" type="button" onclick={deleteSelectedImage}>
					<Trash2 size={13} />{tr('Hapus gambar')}
				</button>
			</div>
		{/if}
	{:else}
		<textarea
			class="rte-body w-full resize-y rounded-lg border p-3 font-mono text-xs"
			style={`min-height: ${minHeight}px;${fill ? '' : ' max-height: 60vh;'}`}
			bind:value
			disabled={disabled}
			aria-label={ariaLabel ?? 'Isi artikel (HTML)'}
		></textarea>
	{/if}

	{#if imageError}<p class="mt-1 text-sm" style="color: var(--destructive);">{imageError}</p>{/if}

		{#if linkDialog.open}
			<div class="rte-link-backdrop" role="presentation" onclick={closeLinkDialog}></div>
			<div class="rte-link-modal" role="dialog" aria-modal="true" aria-label={tr('Sisipkan Tautan')}>
				<div class="rte-link-modal-head">
					<strong class="text-sm">{linkDialog.isEdit ? tr('Edit Tautan') : tr('Sisipkan Tautan')}</strong>
					<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" aria-label={tr('Tutup')} onclick={closeLinkDialog}>
						<X size={14} />
					</button>
				</div>

				<label class="mt-3 block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Teks Tautan (anchor)')}</span>
					<input
						class="app-input mt-1 w-full text-sm"
						placeholder={tr('Teks yang tampil sebagai tautan')}
						bind:value={linkDialog.text}
						onkeydown={(e) => { if (e.key === 'Escape') closeLinkDialog(); }}
					/>
					<span class="text-[11px] text-muted-foreground">
						{tr('Otomatis terisi dari teks yang kamu blok di artikel. Bisa diubah.')}
					</span>
				</label>

				<label class="mt-3 block">
					<span class="text-xs font-medium text-muted-foreground">URL</span>
					<input
						class="app-input mt-1 w-full text-sm"
						placeholder="https://contoh.com/artikel"
						bind:value={linkDialog.url}
						onkeydown={(e) => {
							if (e.key === 'Enter') { e.preventDefault(); applyLink(); }
							if (e.key === 'Escape') closeLinkDialog();
						}}
					/>
				</label>

				<fieldset class="mt-3">
					<legend class="text-xs font-medium text-muted-foreground">{tr('Jenis Tautan (rel)')}</legend>
					<div class="mt-1.5 flex flex-wrap gap-3">
						{#each [['dofollow', tr('Dofollow')], ['nofollow', 'Nofollow'], ['sponsored', 'Sponsored']] as [value, label] (value)}
							<label class="flex items-center gap-1.5 text-sm">
								<input
									type="radio"
									name="rte-link-rel"
									value={value}
									checked={linkDialog.rel === value}
									onchange={() => (linkDialog.rel = value)}
								/>
								{label}
							</label>
						{/each}
					</div>
					<p class="mt-1 text-[11px] text-muted-foreground">{REL_HINTS[linkDialog.rel] ?? ''}</p>
				</fieldset>

				{#if linkDialog.error}
					<p class="mt-2 text-sm" style="color: var(--destructive);">{linkDialog.error}</p>
				{/if}

				<div class="mt-4 flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={closeLinkDialog}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="button" onclick={applyLink}>{tr('Simpan')}</button>
				</div>
			</div>
		{/if}

		{#if imageModalOpen}
			<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
					<div
						class="fixed inset-0 z-[60] flex items-center justify-center p-4"
						style="background-color: rgba(15, 23, 42, 0.45);"
						role="dialog"
						tabindex="-1"
					aria-modal="true"
					aria-labelledby="rte-image-title"
					onclick={(e) => { if (e.target === e.currentTarget && !imageBusy) imageModalOpen = false; }}
				>
					<div
						class="flex w-full max-w-md flex-col overflow-hidden rounded-2xl border bg-card shadow-2xl"
						style="border-color: var(--border); background-color: var(--card, #fff);"
					>
					<!-- Header -->
					<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
						<div>
							<h3 id="rte-image-title" class="text-base font-semibold">{tr('Sisipkan Gambar ke Artikel')}</h3>
							<p class="mt-0.5 text-xs text-muted-foreground">{tr('Tarik berkas gambar, tempel dengan Ctrl/Cmd+V, atau klik untuk memilih.')}</p>
						</div>
						<button
							class="rounded-lg p-1.5 hover:bg-secondary"
							type="button"
							aria-label={tr('Tutup')}
							disabled={imageBusy}
							onclick={() => (imageModalOpen = false)}
						>
							<X size={18} />
						</button>
					</div>

					<!-- Body: Dropzone & Paste target -->
					<div class="p-5">
						<div
							class="flex cursor-pointer flex-col items-center gap-2 rounded-xl border-2 border-dashed p-8 text-center transition-colors"
							class:border-primary={imageDragOver}
							style="border-color: {imageDragOver ? 'var(--primary)' : 'var(--border)'}; background-color: var(--muted);"
							role="button"
							tabindex="0"
							aria-label={tr('Unggah gambar')}
							onclick={() => { if (!imageBusy) imageInput?.click(); }}
							onkeydown={(e) => { if ((e.key === 'Enter' || e.key === ' ') && !imageBusy) { e.preventDefault(); imageInput?.click(); } }}
							ondragover={(e) => { e.preventDefault(); imageDragOver = true; }}
							ondragleave={() => (imageDragOver = false)}
							ondrop={(e) => {
								e.preventDefault();
								imageDragOver = false;
								const file = e.dataTransfer?.files?.[0];
								if (file && file.type.startsWith('image/')) void pickImage(file);
							}}
						>
							{#if imageBusy}
								<LoaderCircle size={28} class="animate-spin" style="color: var(--primary);" />
								<p class="mt-2 text-sm font-semibold">{tr('Mengunggah gambar…')}</p>
							{:else}
								<div class="flex h-12 w-12 items-center justify-center rounded-full" style="background-color: var(--primary); color: var(--primary-foreground);">
									<Upload size={20} />
								</div>
								<p class="mt-2 text-sm font-bold">{tr('Klik, Tarik, atau Tempel Gambar')}</p>
								<p class="text-xs text-muted-foreground">{tr('Mendukung JPG, PNG, WebP (otomatis dikompres)')}</p>
							{/if}
						</div>

							{#if imageError}
								<p class="mt-3 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
									<CircleAlert size={15} /> {imageError}
								</p>
							{/if}
						</div>
					</div>
				</div>
			{/if}

		{#if campaignId && mediaId && !uploadImage}
			<MediaLibraryPicker
				open={mediaPickerOpen}
				{brandId}
				{campaignId}
				{mediaId}
				{articleId}
				mode="inline"
				onSelect={(url) => {
					insertImageUrl(url);
					mediaPickerOpen = false;
				}}
				onClose={() => (mediaPickerOpen = false)}
			/>
		{/if}
	</div>

<style>
	.rte {
		position: relative;
	}
	/* Toolbar menempel di atas saat SELURUH modal digulir (bukan scroll internal
	   editor) — container penggulirnya adalah grid overflow-y-auto di modal
	   Pengelola Naskah. Latar harus pekat supaya isi tidak tembus di baliknya. */
	.rte :global(.rte-img-selected) {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
		border-radius: 6px;
	}

	.rte-imgbar {
		position: absolute;
		z-index: 20;
		display: flex;
		gap: 4px;
		padding: 3px;
		border-radius: 9px;
		border: 1px solid var(--border);
		background: var(--card);
		box-shadow: 0 8px 22px rgba(15, 23, 42, 0.16);
	}
	.rte-imgbar-btn {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border-radius: 6px;
		font-size: 12px;
		color: var(--destructive);
	}
	.rte-imgbar-btn:hover { background: var(--secondary); }
	.rte-imgbar-btn:focus-visible { outline: 2px solid var(--primary); outline-offset: 1px; }

	.rte-toolbar {
		position: sticky;
		top: 0;
		z-index: 30;
		background: var(--card, #fff);
	}
	.rte-tool {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 32px;
		min-width: 32px;
		padding: 0 8px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}
	.rte-tool:hover:not(:disabled) {
		background: var(--secondary);
	}
	.rte-tool:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}
	.rte-tool:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}
	.rte-tool-toggle {
		border: 1px solid var(--border);
	}
	.rte-tool-active {
		background: var(--primary);
		color: var(--primary-foreground);
		border-color: var(--primary);
	}
	/* Tombol format yang menyala tetap menyala saat di-hover (jangan berubah
	   jadi latar terang dengan teks putih yang tak terbaca). */
	.rte-tool.rte-tool-active:hover:not(:disabled) {
		background: var(--primary);
		color: var(--primary-foreground);
	}
	.rte-table-hint {
		display: inline-flex;
		align-items: center;
		min-height: 24px;
		margin-left: 4px;
		padding: 0 8px;
		border-radius: 999px;
		background: var(--accent);
		color: var(--primary);
		font-size: 11px;
		font-weight: 600;
	}
	/* Toggle HTML saat AKTIF lalu di-hover: ikon tetap terlihat — biru di atas
	   latar terang. Dulu warna ikonnya tetap putih (primary-foreground) sementara
	   latar hover berubah terang, sehingga ikonnya hilang. */
	.rte-tool-toggle.rte-tool-active:hover:not(:disabled) {
		background: var(--secondary);
		color: var(--primary);
		border-color: var(--primary);
	}
		.rte-body {
			margin-top: 8px;
			background: var(--card, #fff);
		}
		:global(.rte-body img) {
			max-width: 100%;
			height: auto;
			aspect-ratio: 16 / 9;
			object-fit: cover;
			border-radius: 8px;
			margin: 12px 0;
			display: block;
		}
	.rte-link-backdrop {
		position: fixed;
		inset: 0;
		z-index: 1090;
		background: rgba(15, 23, 42, 0.45);
	}
	.rte-link-modal {
		position: fixed;
		z-index: 1100;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		width: min(92vw, 420px);
		max-height: 90vh;
		overflow-y: auto;
		padding: 16px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card, #fff);
		box-shadow: 0 18px 44px rgba(15, 23, 42, 0.22);
	}
	.rte-link-modal-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	/* ── Mode fill (modal editor draf mitra): root jadi kolom flex yang memakai
	   sisa tinggi kolom pemanggil — isi editor menggulir sendiri, batas 60vh
	   tidak berlaku. Toolbar berubah jadi pita putih dengan garis bawah tipis
	   (tanpa kotak border), dan pemisah grup ikon hanya muncul di mode ini. ── */
	.rte-fill {
		flex: 1 1 0;
		min-height: 0;
		display: flex;
		flex-direction: column;
		background: var(--card, #fff);
	}
	.rte-fill > .rte-toolbar {
		position: static;
		flex: none;
		border: none;
		border-bottom: 1px solid var(--border);
		border-radius: 0;
		padding: 0 2px 7px;
	}
	.rte-fill .rte-body {
		flex: 1 1 0;
		min-height: 0;
		max-height: none;
		margin-top: 0;
		border: none;
		border-radius: 0;
		overflow-y: auto;
	}
	.rte-sep {
		flex: none;
		width: 1px;
		height: 18px;
		margin: 0 5px 0 7px;
		background: var(--border);
	}
</style>
