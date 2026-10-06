<script lang="ts">
	// Modal "Pengelola Naskah" — satu tempat untuk menulis, mengunggah dokumen,
	// atau membuat draf AI untuk satu media target, dengan panel pengaturan
	// artikel (cover, kategori, meta deskripsi) dan footer validasi.
	// Reuse: rich-text-editor (toolbar + contentEditable), endpoint /files dan
	// /produce/{mediaId} lewat callback dari writer, serta saveEditor di writer.
	import { X, Upload, FileText, Sparkles, LoaderCircle, Image as ImageIcon, Trash2, CircleAlert, CircleCheck, Info, ShieldCheck, Link2, Plus } from '@lucide/svelte';
	import { untrack } from 'svelte';
	import { tr } from '$lib/i18n/phrases';
	import { apiRequest, readApiError } from '$lib/auth';
	import { ARTICLE_DEFAULT_COVER } from '$lib/media/cover';
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';
	import MediaLibraryPicker from '$lib/components/media-library-picker.svelte';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import Select from '$lib/components/select.svelte';
	import { textToHtml } from '../../_shared';

	let {
		mediaName,
		brandId,
		campaignId = '',
		mediaId = '',
		initialTab = 'tulis',
		initialTitle = '',
		initialHtml = '',
		initialCoverUrl = null,
		initialCategory = null,
		initialMetaDescription = null,
		categoryOptions = [],
		linkLimit,
		saving = false,
		saveError = null,
		onClose,
		onSave,
		onUploadCover,
		onUploadDoc,
		onGenerate,
		gatekeeperConfigured = null,
		onValidate,
		streamingHtml = null
	}: {
		mediaName: string;
		brandId: string;
		/** Scope pustaka gambar AI: brand + campaign + media. */
		campaignId?: string;
		mediaId?: string;
		initialTab?: 'tulis' | 'unggah' | 'generate';
		initialTitle?: string;
		initialHtml?: string;
		initialCoverUrl?: string | null;
		initialCategory?: string | null;
		initialMetaDescription?: string | null;
		categoryOptions?: string[];
		linkLimit: number;
		saving?: boolean;
		saveError?: string | null;
		onClose: () => void;
		onSave: (title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) => void;
		onUploadCover: (file: File) => Promise<string>;
		onUploadDoc: (file: File, onPhase?: (phase: 'extract' | 'clean') => void, links?: WriterLink[]) => Promise<{ title: string; html: string; warning?: string }>;
		onGenerate: (topic: string, links: WriterLink[]) => Promise<{ title: string; html: string }>;
		/** null = belum tahu; false = AI Gatekeeper belum dikonfigurasi admin. */
		gatekeeperConfigured?: boolean | null;
		/** Cek Validasi (QC) dari writer: simpan lalu pindai; hasil dibuka di panel QC. */
		onValidate: (title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) => Promise<{ ok: boolean; passed: boolean; message: string; error: string }>;
		/** Pratinjau streaming: html parsial yang tampil read-only saat AI menulis; null = tidak streaming. */
		streamingHtml?: string | null;
	} = $props();

// Nilai awal disalin sekali per mount — writer memakai {#key mediaId}
// sehingga setiap buka modal adalah instance baru.
	type Tab = 'tulis' | 'unggah' | 'generate';
	let tab = $state<Tab>(untrack(() => initialTab));
	let title = $state(untrack(() => initialTitle));
	let html = $state(untrack(() => initialHtml));
	let cover = $state<string | null>(untrack(() => initialCoverUrl));
	let category = $state<string>(untrack(() => initialCategory) ?? '');
	let meta = $state(untrack(() => initialMetaDescription) ?? '');
	let editorStats = $state({ words: 0, links: 0, images: 0 });

	let coverBusy = $state(false);
	let coverError = $state('');
	let coverNote = $state('');
	let dragging = $state(false);
	let coverInput = $state<HTMLInputElement | undefined>(undefined);
	let docInput = $state<HTMLInputElement | undefined>(undefined);

	let docBusy = $state(false);
	let docError = $state('');
	let docPhase = $state<'extract' | 'clean'>('extract');
	let docWarning = $state('');
	let genBusy = $state(false);
	let genError = $state('');
	let topic = $state('');
	let teksTempel = $state('');

	// ---------- tautan tambahan (anchor + URL) ----------
	// Dipakai di tab "Generate dengan AI" dan "Unggah Dokumen": AI diminta
	// menyisipkan tautan ini secara natural ke badan artikel.
	type WriterLink = { anchor_text: string; url: string };
	type LinkRow = { id: number; anchor: string; url: string };
	let linkRows = $state<LinkRow[]>([]);
	// Batas baris tautan mengikuti kuota tautan media (max_links). Media yang
	// hanya mengizinkan 2 tautan tidak bisa ditambah lebih dari 2. 0/null -> 10.
	const maxLinkRows = $derived(linkLimit && linkLimit > 0 ? linkLimit : 10);
	let linkSeq = 0;

	function addLinkRow() {
		if (linkRows.length >= maxLinkRows) return;
		linkRows = [...linkRows, { id: ++linkSeq, anchor: '', url: '' }];
	}
	function removeLinkRow(id: number) {
		linkRows = linkRows.filter((row) => row.id !== id);
	}
	function isHttpUrl(raw: string): boolean {
		const s = raw.trim();
		if (!/^https?:\/\//i.test(s)) return false;
		try {
			return Boolean(new URL(s).hostname);
		} catch {
			return false;
		}
	}
	// Pesan galat per baris; baris kosong diabaikan. Submit diblok selama ada galat.
	const linkErrors = $derived.by(() => {
		const errs: Record<number, string> = {};
		for (const row of linkRows) {
			const anchor = row.anchor.trim();
			const url = row.url.trim();
			if (!anchor && !url) continue;
			if (!url) {
				errs[row.id] = tr('URL wajib diisi bila anchor diisi.');
			} else if (!isHttpUrl(url)) {
				errs[row.id] = tr('URL tidak valid. Gunakan alamat http:// atau https://.');
			} else if (!anchor) {
				errs[row.id] = tr('Anchor wajib diisi bila URL diisi.');
			}
		}
		return errs;
	});
	const linksValid = $derived(Object.keys(linkErrors).length === 0);
	const payloadLinks = $derived(
		linkRows
			.map((row) => ({ anchor_text: row.anchor.trim(), url: row.url.trim() }))
			.filter((link) => link.anchor_text !== '' && link.url !== '')
	);

	// Hasil Cek Validasi (QC) untuk konten yang sedang diedit. Perubahan isi
	// membatalkan hasil lama supaya artikel tidak tersimpan tanpa QC ulang.
	let checking = $state(false);
	let checked = $state(false);
	let checkError = $state('');
	let checkOutcome = $state<{ passed: boolean; message: string } | null>(null);

	let panelEl = $state<HTMLElement | null>(null);

	// ---------- gambar AI (cover & per H2) ----------
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
		let coverPickerOpen = $state(false);
		let h2Busy = $state(false);
		let h2ConfirmOpen = $state(false);
		let h2ConfirmData = $state<{
			items: { label: string; prompt: string }[];
			cost: number;
			balance: number | null;
			total: number;
		} | null>(null);
		let h2Error = $state('');
		let h2Note = $state('');
		let imageCost = $state(0);

		function close() {
			if (!saving) onClose();
		}

		$effect(() => {
			const onKey = (e: KeyboardEvent) => {
				if (coverPickerOpen || h2ConfirmOpen) return;
				if (e.key === 'Escape') close();
			};
		const onPaste = (e: ClipboardEvent) => {
			const files = Array.from(e.clipboardData?.files ?? []);
			const gambar = files.find((f) => f.type.startsWith('image/'));
			if (gambar) {
				e.preventDefault();
				void pickCover(gambar);
				return;
			}
			const dokumen = files.find((f) => /\.[a-z0-9]+$/i.test(f.name) && /pdf|docx|txt/i.test(f.name.split('.').pop() ?? ''));
			if (dokumen) {
				e.preventDefault();
				tab = 'unggah';
				void uploadDoc(dokumen);
			}
		};
		window.addEventListener('keydown', onKey);
		window.addEventListener('paste', onPaste);
		const previous = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		panelEl?.focus();
		return () => {
			window.removeEventListener('keydown', onKey);
			window.removeEventListener('paste', onPaste);
			document.body.style.overflow = previous;
		};
	});

	// ---------- cover ----------
	const KB = (b: number) => (b >= 1024 * 1024 ? (b / 1024 / 1024).toFixed(1) + ' MB' : Math.max(1, Math.round(b / 1024)) + ' KB');

	// Kompres di browser sebelum diunggah: kanvas → WebP, sisi terpanjang
	// dibatasi 1600 px. GIF dibiarkan (animasinya hilang bila dikonversi), dan
	// bila hasil kompresi tidak lebih kecil, berkas asli yang dikirim.
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

	async function pickCover(file: File) {
		if (!file || coverBusy || !file.type.startsWith('image/')) return;
		coverBusy = true;
		coverError = '';
		coverNote = '';
		try {
			const siap = await kompresWebp(file);
			const url = await onUploadCover(siap);
			cover = url;
			coverNote =
				siap.size < file.size
					? `${tr('Dikompres ke WebP:')} ${KB(file.size)} → ${KB(siap.size)}`
					: `${tr('Diunggah sebagai')} ${siap.type === 'image/webp' ? 'WebP' : file.type.replace('image/', '').toUpperCase()} (${KB(siap.size)})`;
		} catch {
			coverError = tr('Gagal mengunggah gambar. Periksa koneksi lalu coba lagi.');
		} finally {
			coverBusy = false;
			if (coverInput) coverInput.value = '';
		}
	}
	function onDropCover(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		const file = e.dataTransfer?.files?.[0];
		if (file) void pickCover(file);
	}
	function useFallbackCover() {
		cover = ARTICLE_DEFAULT_COVER;
		coverError = '';
		coverNote = '';
	}

	// ---------- gambar AI (pustaka + per H2) ----------

	// Ambil biaya per gambar + saldo dari pustaka (untuk konfirmasi biaya).
	async function fetchImageCost(): Promise<{ cost: number; balance: number | null }> {
		try {
			const res = await apiRequest(`${apiBase}/images?media_id=${encodeURIComponent(mediaId)}`);
			if (!res.ok) return { cost: imageCost, balance: null };
			const d = await res.json().catch(() => null);
			imageCost = Number(d?.cost_credits ?? imageCost);
			return {
				cost: imageCost,
				balance: Number.isFinite(Number(d?.balance_remaining)) ? Number(d.balance_remaining) : null
			};
		} catch {
			return { cost: imageCost, balance: null };
		}
	}

	/** Buat satu gambar AI untuk satu heading, kembalikan URL-nya. */
	async function generateImageFor(heading: string, prompt: string): Promise<string> {
		const res = await apiRequest(
			`${apiBase}/images/generate`,
			{ method: 'POST', body: JSON.stringify({ media_id: mediaId, heading, prompt, role: 'inline' }) },
			180_000
		);
		const d = await res.json().catch(() => null);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat gambar AI.')));
		const url = String(d?.url ?? '');
		if (!url) throw new Error(tr('Server tidak mengembalikan URL gambar.'));
		imageCost = Number(d?.cost_credits ?? imageCost);
		return url;
	}

		// Ambil blok teks berisi isi artikel (dasar hitungan paragraf).
		// Editor bisa menghasilkan <p> maupun <div> tergantung browser, jadi
		// keduanya diterima — pembungkus yang isinya blok lain dibuang.
		const BLOK_TAG = new Set(['P', 'DIV', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'UL', 'OL', 'BLOCKQUOTE', 'TABLE', 'SECTION', 'ARTICLE', 'FIGURE']);
		function collectParagraphs(doc: Document): HTMLElement[] {
			const candidates = Array.from(doc.body.querySelectorAll('p, div')) as HTMLElement[];
			return candidates.filter((el) => {
				if ((el.textContent ?? '').trim().length === 0) return false;
				// Buang pembungkus yang isinya blok lain (mis. <div><p>…</p></div>).
				return !Array.from(el.children).some((c) => BLOK_TAG.has(c.tagName));
			});
		}

		// Penempatan gambar otomatis. Satu gambar di setiap subjudul H2 selalu
		// dipakai (anchor struktural); di antara anchor itu ditambah gambar isian
		// supaya tidak ada rentang lebih dari PARAGRAF_PER_GAMBAR paragraf tanpa
		// gambar. Ini menutup kasus artikel ber-H2 sedikit — mis. hanya satu —
		// yang dulu cuma dapat satu gambar, tanpa mengubah artikel tanpa H2.
		const PARAGRAF_PER_GAMBAR = 3;
		const MAX_GAMBAR_ISIAN = 6;

		function buildImagePrompt(judul: string, heading: string, konteks: string): string {
			const parts: string[] = [];
			if (judul) parts.push(judul);
			if (heading) parts.push(`Subjudul: ${heading}`);
			const cuplikan = konteks.trim().slice(0, 500);
			if (cuplikan) parts.push(`Ilustrasi untuk bagian artikel berikut: ${cuplikan}`);
			return parts.join('. ');
		}

		function ringkasTeks(teks: string): string {
			return teks.length > 70 ? `${teks.slice(0, 70)}…` : teks;
		}

		// Susun rencana penempatan sesuai urutan dokumen: { node, label, prompt }.
		// Dipakai dua kali — saat menghitung biaya dan saat menyisipkan — supaya
		// anchor yang dipakai identik.
		function planAutoImages(doc: Document, articleTitle: string): { node: Element; label: string; prompt: string }[] {
			const judul = articleTitle.trim();
			const paras = new Set<Element>(collectParagraphs(doc));
			const urut = (Array.from(doc.body.querySelectorAll('h2, p, div')) as HTMLElement[]).filter((el) =>
				el.tagName === 'H2' ? (el.textContent ?? '').trim().length > 0 : paras.has(el)
			);

			const plan: { node: Element; label: string; prompt: string }[] = [];
			let isian = 0;
			let sejakGambar = 0;
			urut.forEach((el, idx) => {
				if (el.tagName === 'H2') {
					const heading = (el.textContent ?? '').trim();
					const berikut = urut.slice(idx + 1).find((n) => n.tagName !== 'H2');
					const konteks = berikut ? (berikut.textContent ?? '').replace(/\s+/g, ' ').trim() : '';
					plan.push({ node: el, label: heading, prompt: buildImagePrompt(judul, heading, konteks) });
					sejakGambar = 0;
					return;
				}
				sejakGambar++;
				if (sejakGambar < PARAGRAF_PER_GAMBAR || isian >= MAX_GAMBAR_ISIAN) return;
				const teks = (el.textContent ?? '').replace(/\s+/g, ' ').trim();
				plan.push({ node: el, label: ringkasTeks(teks), prompt: buildImagePrompt(judul, '', teks) });
				sejakGambar = 0;
				isian++;
			});
			return plan;
		}

		async function promptGeneratePerH2() {
			if (h2Busy) return;
			h2Error = '';
			h2Note = '';
			const parsed = new DOMParser().parseFromString(html || '', 'text/html');
			const plan = planAutoImages(parsed, title);
			if (plan.length === 0) {
				h2Error = tr('Artikel belum punya subjudul H2 atau paragraf yang bisa diberi gambar.');
				return;
			}
			const { cost, balance } = await fetchImageCost();
			const total = cost * plan.length;
			h2ConfirmData = {
				items: plan.map((p) => ({ label: p.label, prompt: p.prompt })),
				cost,
				balance,
				total
			};
			h2ConfirmOpen = true;
		}

		async function executeGeneratePerH2() {
			if (h2Busy || !h2ConfirmData) return;
			const items = h2ConfirmData.items;
			h2ConfirmOpen = false;
			h2Busy = true;
			try {
				const urls: string[] = [];
				for (const item of items) {
					urls.push(await generateImageFor(item.label, item.prompt));
				}
				const doc = new DOMParser().parseFromString(html || '', 'text/html');
				const anchors = planAutoImages(doc, title).map((p) => p.node);
				anchors.forEach((node, i) => {
					if (!urls[i]) return;
					const img = doc.createElement('img');
					img.setAttribute('src', urls[i]);
					img.setAttribute('alt', items[i]?.label ?? '');
					img.setAttribute('loading', 'lazy');
					node.insertAdjacentElement('afterend', img);
				});
				html = doc.body.innerHTML;
				editorStats = { ...editorStats, images: (html.match(/<img\b/gi) ?? []).length };
				h2Note = `${items.length} ${tr('gambar ditambahkan ke artikel.')}`;
			} catch (reason) {
				h2Error = reason instanceof Error ? reason.message : tr('Gagal membuat gambar AI.');
			} finally {
				h2Busy = false;
			}
		}

	// ---------- unggah dokumen ----------
	function onDropDoc(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		const file = e.dataTransfer?.files?.[0];
		if (file) void uploadDoc(file);
	}
	async function uploadDoc(file: File) {
		if (docBusy) return;
		if (!linksValid) {
			docError = tr('Perbaiki tautan dulu sebelum mengunggah dokumen.');
			return;
		}
		const okType = /\.(pdf|docx|txt)$/i.test(file.name) || ['application/pdf', 'text/plain'].includes(file.type);
		if (!okType) {
			docError = tr('Format tidak didukung. Gunakan DOCX, TXT, atau PDF.');
			return;
		}
		if (file.size > 10 * 1024 * 1024) {
			docError = tr('Ukuran file melebihi 10 MB.');
			return;
		}
		docBusy = true;
		docError = '';
		docWarning = '';
		docPhase = 'extract';
		try {
			const hasil = await onUploadDoc(
				file,
				(phase) => {
					docPhase = phase;
				},
				payloadLinks
			);
			title = hasil.title;
			html = hasil.html;
			if (hasil.warning) docWarning = hasil.warning;
			tab = 'tulis';
		} catch (reason) {
			docError = reason instanceof Error ? reason.message : tr('Gagal mengunggah dokumen.');
		} finally {
			docBusy = false;
		}
	}

	// Tempel teks artikel langsung: judul diambil dari baris pertama bila pendek,
	// sisanya jadi isi — sama seperti pemisahan pada unggahan dokumen.
	function pakaiTeksTempel() {
		const teks = teksTempel.trim();
		if (!teks) return;
		const lines = teks.split('\n').map((l) => l.trim()).filter(Boolean);
		let judul = title;
		let isi = teks;
		if (lines.length > 1 && lines[0].length <= 120) {
			judul = lines[0];
			isi = lines.slice(1).join('\n');
		}
		title = judul;
		html = textToHtml(isi);
		teksTempel = '';
		tab = 'tulis';
	}

	// ---------- generate AI ----------
	// Respons /produce/{mediaId} hanya berisi {media_id, status, document_id},
	// tanpa isi artikel. Muat dokumen tersimpan lewat endpoint artikel yang sama
	// dengan pemuat naskah writer (GET /articles/{mediaId}). null = belum ada.
	async function fetchSavedArticle(): Promise<{ title: string; html: string } | null> {
		if (!campaignId || !mediaId) return null;
		try {
			const res = await apiRequest(`${apiBase}/articles/${mediaId}`);
			if (!res.ok) return null;
			const d = await res.json().catch(() => null);
			if (!d) return null;
			return { title: String(d.title ?? ''), html: String(d.content_html ?? '') };
		} catch {
			return null;
		}
	}

	async function generate() {
		if (genBusy || !topic.trim() || !linksValid) return;
		genBusy = true;
		genError = '';
		try {
			const hasil = await onGenerate(topic.trim(), payloadLinks);
			title = hasil.title || title;
			// Respons generate tidak memuat HTML, jadi ambil draf tersimpan agar
			// editor langsung terisi dan tab berpindah ke Tulis / Edit.
			if (hasil.html && hasil.html.trim()) {
				html = hasil.html;
			} else {
				const draf = await fetchSavedArticle();
				if (draf?.html) {
					html = draf.html;
					if (draf.title) title = draf.title;
				}
			}
			tab = 'tulis';
		} catch (reason) {
			genError = reason instanceof Error ? reason.message : tr('Gagal membuat draf AI.');
		} finally {
			genBusy = false;
		}
	}

	// ---------- validasi & simpan ----------
	// Sisa alasan (selain cover) tampil di footer sebagai satu pesan singkat.
	const alasanLain = $derived.by(() => {
		const list: string[] = [];
		if (!title.trim()) list.push(tr('Judul artikel wajib diisi.'));
		if (!category.trim()) list.push(tr('Kategori / rubrik wajib dipilih.'));
		if (editorStats.words === 0) list.push(tr('Isi artikel wajib ditulis atau diunggah.'));
		if (linkLimit > 0 && editorStats.links > linkLimit) {
			list.push(`${tr('Tautan melebihi batas media.')} (${editorStats.links}/${linkLimit})`);
		}
		return list;
	});
	const canSave = $derived(!!cover && alasanLain.length === 0 && !saving && !checking);
	// Cek Validasi butuh konten siap karena QC dijalankan pada salinan tersimpan.
	const canCheck = $derived(!!cover && alasanLain.length === 0 && !saving && !checking && gatekeeperConfigured !== false);

	// Perubahan isi membatalkan hasil Cek Validasi sebelumnya.
	$effect(() => {
		title;
		html;
		cover;
		category;
		meta;
		checked = false;
		checkOutcome = null;
	});

	// Pesan footer mengikuti apa yang masih kurang, bukan kalimat tetap.
	const pesanKurang = $derived.by(() => {
		if (linkLimit > 0 && editorStats.links > linkLimit) {
			return `${tr('Kurangi tautan agar sesuai batas media.')} (${editorStats.links}/${linkLimit})`;
		}
		const bagian: string[] = [];
		if (!title.trim()) bagian.push(tr('judul'));
		if (!category.trim()) bagian.push(tr('kategori'));
		if (editorStats.words === 0) bagian.push(tr('artikel'));
		if (!cover) bagian.push(tr('thumbnail'));
		if (bagian.length === 0) return '';
		return `${tr('Lengkapi')} ${bagian.join(' & ')} ${tr('untuk menyimpan.')}`;
	});

	/** Jalankan Cek Validasi lewat writer. true = hasil didapat (lulus/gagal). */
	async function runCheck(): Promise<boolean> {
		checking = true;
		checkError = '';
		checkOutcome = null;
		try {
			const outcome = await onValidate(title.trim(), html, cover, category, meta.trim());
			if (!outcome.ok) {
				checkError = outcome.error || tr('Gagal menjalankan pemindaian QC.');
				return false;
			}
			checked = true;
			checkOutcome = { passed: outcome.passed, message: outcome.message };
			return true;
		} finally {
			checking = false;
		}
	}

	/** Tombol "Cek Validasi Artikel": memindai artikel tanpa menyimpan. */
	function checkNow() {
		if (checking || saving || !canCheck) return;
		void runCheck();
	}

	/** Simpan: bila belum dicek, QC wajib jalan dulu sebelum menyimpan. */
	async function save() {
		if (!canSave || saving || checking) return;
		// QC tidak tersedia (belum dikonfigurasi) → jangan tahan simpan.
		if (gatekeeperConfigured === false) {
			onSave(title.trim(), html, cover, category, meta.trim());
			return;
		}
		if (!checked) {
			// Request QC gagal → tampilkan error dan jangan simpan. Kalau QC jalan
			// tapi belum lulus, simpan tetap lanjut; publikasi yang diblokir.
			const obtained = await runCheck();
			if (!obtained) return;
		}
		onSave(title.trim(), html, cover, category, meta.trim());
	}

	const kategoriOptions = $derived(categoryOptions.map((c) => ({ value: c, label: c })));
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6" style="background-color: rgba(15, 23, 42, 0.45);">
	<div
		bind:this={panelEl}
		class="flex max-h-[94vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border bg-card shadow-2xl"
		style="border-color: var(--border); background-color: var(--card, #fff);"
		tabindex="-1"
		role="dialog"
		aria-modal="true"
		aria-label="Pengelola Naskah"
	>
		<!-- Header tetap -->
		<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
			<div class="min-w-0">
				<h2 class="text-base font-semibold">{tr('Pengelola Naskah')}</h2>
				<p class="mt-0.5 truncate text-xs text-muted-foreground">{tr('Media Target:')} <strong class="text-foreground">{mediaName}</strong></p>
			</div>
			<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr('Tutup')} onclick={close}>
				<X size={18} />
			</button>
		</div>

		<!-- Tab -->
		<div class="border-b px-5 pt-3" style="border-color: var(--border);">
			<div class="flex flex-wrap gap-1" role="tablist" aria-label={tr('Cara menyusun naskah')}>
				{#each [
					{ key: 'unggah', label: tr('Unggah Dokumen') },
					{ key: 'generate', label: tr('Generate dengan AI') },
					{ key: 'tulis', label: tr('Tulis / Edit') }
				] as t (t.key)}
					<button
						class="-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors"
						style="border-color: {tab === t.key ? 'var(--primary)' : 'transparent'}; color: {tab === t.key ? 'var(--primary)' : 'var(--muted-foreground)'};"
						type="button"
						role="tab"
						aria-selected={tab === t.key}
						onclick={() => (tab = t.key as Tab)}
					>
						{#if t.key === 'tulis'}<FileText size={14} />{:else if t.key === 'unggah'}<Upload size={14} />{:else}<Sparkles size={14} />{/if}
						{t.label}
					</button>
				{/each}
			</div>
		</div>

		{#if docWarning}
			<div class="mx-5 mt-4 flex items-start gap-2 rounded-xl border px-3 py-2.5 text-xs" style="border-color: var(--destructive); background-color: color-mix(in srgb, var(--destructive) 8%, var(--card)); color: var(--destructive);" role="alert">
				<CircleAlert size={14} class="mt-0.5 shrink-0" />
				<span class="flex-1">{docWarning}</span>
				<button class="rounded p-0.5 hover:bg-secondary" type="button" aria-label={tr('Tutup')} onclick={() => (docWarning = '')}><X size={12} /></button>
			</div>
		{/if}

	{#snippet linkEditor()}
		<div class="mt-4 w-full rounded-xl border p-3 text-left" style="border-color: var(--border); background-color: var(--muted);">
			<div class="flex items-center justify-between gap-2">
				<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
					<Link2 size={13} /> {tr('Tautan & Anchor (Opsional)')}
				</p>
				<span
					class="rounded-full px-2 py-0.5 text-[10px] font-bold"
					style={linkRows.length >= maxLinkRows
						? 'background-color: var(--primary); color: #fff;'
						: 'background-color: var(--muted); color: var(--muted-foreground);'}
				>{linkRows.length}/{maxLinkRows}</span>
			</div>
			<p class="mt-1 text-[11px] leading-relaxed text-muted-foreground">
				{tr('AI menyisipkan tautan ini secara natural pada bagian yang relevan. Isi anchor dan URL secara berpasangan.')}
			</p>
			{#if linkRows.length === 0}
				<p class="mt-3 rounded-lg border border-dashed px-3 py-4 text-center text-xs text-muted-foreground" style="border-color: var(--border);">
					{tr('Belum ada tautan')}
				</p>
			{:else}
				<div class="mt-3 space-y-2">
					{#each linkRows as row (row.id)}
						<div class="rounded-lg border p-2" style="border-color: var(--border); background-color: var(--card, #fff);">
							<div class="flex items-start gap-2">
								<div class="grid min-w-0 flex-1 gap-2 sm:grid-cols-2">
									<label class="block min-w-0">
										<span class="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">{tr('Anchor Text')}</span>
										<input
											class="app-input mt-1 w-full text-sm"
											placeholder={tr('Contoh: sewa server NVMe murah')}
											bind:value={row.anchor}
											maxlength={120}
											aria-label={tr('Anchor Text')}
										/>
									</label>
									<label class="block min-w-0">
										<span class="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">{tr('URL')}</span>
										<input
											class="app-input mt-1 w-full text-sm"
											placeholder={tr('https://contoh.com/halaman')}
											bind:value={row.url}
											inputmode="url"
											aria-label={tr('URL')}
										/>
									</label>
								</div>
								<button
									class="flex h-9 w-9 shrink-0 items-center justify-center self-end rounded-lg border text-muted-foreground transition-colors hover:border-destructive hover:bg-destructive hover:text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1"
									style="border-color: var(--border);"
									type="button"
									aria-label={tr('Hapus tautan')}
									title={tr('Hapus tautan')}
									onclick={() => removeLinkRow(row.id)}
								>
									<Trash2 size={15} />
								</button>
							</div>
							{#if linkErrors[row.id]}
								<p class="mt-1 flex items-center gap-1 text-[11px]" style="color: var(--destructive);">
									<CircleAlert size={12} /> {linkErrors[row.id]}
								</p>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
			<button
				class="app-btn app-btn-soft mt-3 w-full !min-h-8 !px-3 !text-xs"
				type="button"
				disabled={linkRows.length >= maxLinkRows}
				onclick={addLinkRow}
			>
				<Plus size={14} /> {tr('Tambah tautan')}
			</button>
			{#if linkRows.length >= maxLinkRows}
				<p class="mt-1.5 text-center text-[11px] text-muted-foreground">
					{tr('Batas maksimal tautan media ini sudah tercapai.')}
				</p>
			{/if}
		</div>
	{/snippet}

			<!-- Body: dua kolom, scroll internal -->
		<div class="grid flex-1 gap-5 overflow-y-auto p-5 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
			<!-- Kolom kerja utama -->
			<div class="min-w-0">
				{#if tab === 'tulis'}
					<label class="block text-xs font-semibold uppercase tracking-wide text-muted-foreground" for="naskah-judul">
						{tr('Judul Artikel')}
					</label>
					<input
						id="naskah-judul"
						class="app-input mt-2 w-full"
						placeholder={tr('Masukkan judul yang memikat…')}
						bind:value={title}
						maxlength={300}
					/>
					<div class="mt-3 flex flex-wrap items-center gap-2 rounded-xl border p-3" style="border-color: var(--border); background-color: var(--muted);">
						<div class="min-w-0 flex-1">
							<p class="flex items-center gap-1.5 text-xs font-semibold text-foreground"><ShieldCheck size={13} /> {tr('Cek Validasi Artikel')}</p>
							{#if gatekeeperConfigured === false}
								<p class="mt-0.5 text-[11px]" style="color: var(--warning);">{tr('AI Gatekeeper belum dikonfigurasi oleh admin. Pemindaian tidak akan berjalan sampai admin mengaktifkannya.')}</p>
							{:else}
								<p class="mt-0.5 text-[11px] text-muted-foreground">{tr('Jalankan pemeriksaan QC sebelum menyimpan. Hasil lengkap tampil di panel Analisis QC.')}</p>
							{/if}
						</div>
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={!canCheck} onclick={checkNow}>
							{#if checking}<LoaderCircle size={14} class="animate-spin" />{:else}<ShieldCheck size={14} />{/if}
							{checking ? tr('Memeriksa…') : tr('Cek Validasi Artikel')}
						</button>
					</div>
					{#if checkError}
						<p class="mt-2 flex items-start gap-1.5 text-xs" style="color: var(--destructive);" role="alert"><CircleAlert size={14} class="mt-0.5 shrink-0" /> <span>{checkError}</span></p>
					{:else if checkOutcome}
						<p class="mt-2 flex items-start gap-1.5 text-xs" style="color: {checkOutcome.passed ? 'var(--success)' : 'var(--destructive)'};" role="status">
							{#if checkOutcome.passed}<CircleCheck size={14} class="mt-0.5 shrink-0" />{:else}<CircleAlert size={14} class="mt-0.5 shrink-0" />{/if}
							<span>{checkOutcome.passed ? tr('Artikel lolos QC.') : `${tr('QC belum lulus:')} ${checkOutcome.message}`}</span>
						</p>
					{/if}
					<div class="mt-4">
						<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{tr('Isi Artikel (Konten)')}</p>
						<div class="mt-2">
							{#if streamingHtml !== null && streamingHtml !== undefined}
								<div class="rounded-lg border p-4" style="border-color: var(--border);" aria-live="polite">
									<p class="mb-2 flex items-center gap-1.5 text-xs font-semibold" style="color: var(--primary);">
										<LoaderCircle size={13} class="animate-spin" /> {tr('AI sedang menulis artikel…')}
									</p>
									<div class="article-html max-w-none text-sm" style="min-height: 320px;">{@html sanitizeHtml(streamingHtml ?? '')}</div>
								</div>
							{:else}
								<RichTextEditor bind:value={html} bind:stats={editorStats} brandId={brandId} campaignId={campaignId} mediaId={mediaId} ariaLabel={tr('Isi artikel')} minHeight={360} />
							{/if}
						</div>
						{#if campaignId && mediaId}
							<div class="mt-3 rounded-xl border p-3" style="border-color: var(--border); background-color: var(--muted);">
								<div class="flex flex-wrap items-center justify-between gap-2">
									<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
										<Sparkles size={13} /> {tr('Gambar Otomatis Artikel')}
									</p>
										<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={h2Busy} onclick={promptGeneratePerH2}>
											{#if h2Busy}<LoaderCircle size={14} class="animate-spin" />{:else}<Sparkles size={14} />{/if}
											{h2Busy ? tr('Membuat gambar…') : tr('Generate gambar otomatis')}
										</button>
								</div>
									<p class="mt-1 text-[11px] text-muted-foreground">
										{tr('Satu gambar AI di setiap subjudul H2, ditambah gambar isian tiap 3 paragraf pada bagian yang panjang. Biaya dikonfirmasi dulu sebelum proses.')}
									</p>
								{#if h2Error}
									<p class="mt-2 flex items-center gap-1.5 text-xs" style="color: var(--destructive);"><CircleAlert size={14} /> {h2Error}</p>
								{:else if h2Note}
									<p class="mt-2 text-xs" style="color: var(--success);">{h2Note}</p>
								{/if}
							</div>
						{/if}
					</div>
				{:else if tab === 'unggah'}
					<div class="flex h-full min-h-[320px] flex-col items-center justify-center py-6 text-center">
						<h3 class="text-xl font-bold">{tr('Punya Draf Sendiri?')}</h3>
						<p class="mt-2 max-w-sm text-sm text-muted-foreground">
							{tr('Unggah file Word/TXT. AI Tauthub akan secara cerdas memisahkan Judul, Paragraf, dan Kategori secara otomatis.')}
						</p>
						<div
							class="mt-6 flex w-full max-w-md cursor-pointer flex-col items-center gap-2 rounded-xl border-2 border-dashed p-8 transition-colors"
							class:border-primary={dragging}
							style="border-color: {dragging ? 'var(--primary)' : 'var(--border)'}; background-color: var(--muted);"
							role="button"
							tabindex="0"
							aria-label={tr('Unggah dokumen')}
							onclick={() => docInput?.click()}
							onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); docInput?.click(); } }}
							ondragover={(e) => { e.preventDefault(); dragging = true; }}
							ondragleave={() => (dragging = false)}
							ondrop={onDropDoc}
						>
							<input
								bind:this={docInput}
								type="file"
								accept=".pdf,.docx,.txt"
								class="hidden"
								onchange={(e) => {
									const f = (e.currentTarget as HTMLInputElement).files?.[0];
									if (f) void uploadDoc(f);
								}}
							/>
							{#if docBusy}
								<LoaderCircle size={28} class="animate-spin" style="color: var(--primary);" />
								<p class="mt-2 text-sm font-semibold">{docPhase === 'clean' ? tr('Merapikan dokumen dengan AI...') : tr('Memproses dokumen…')}</p>
							{:else}
								<div class="flex h-12 w-12 items-center justify-center rounded-full" style="background-color: var(--primary); color: var(--primary-foreground);">
									<Upload size={20} />
								</div>
								<p class="mt-2 text-sm font-bold">{tr('Klik, Tarik, atau Tempel File ke sini')}</p>
								<p class="text-xs text-muted-foreground">{tr('Mendukung DOCX, TXT, PDF (Maks 10MB)')}</p>
							{/if}
						</div>
						{#if docError}
							<p class="mt-3 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
								<CircleAlert size={15} /> {docError}
							</p>
						{/if}

						<div class="mt-6 w-full max-w-md text-left">
							<div class="flex items-center gap-2">
								<span class="h-px flex-1" style="background-color: var(--border);"></span>
								<span class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{tr('atau tempel teks')}</span>
								<span class="h-px flex-1" style="background-color: var(--border);"></span>
							</div>
							<textarea
								class="app-input mt-3 min-h-[110px] w-full resize-y text-sm"
								placeholder={tr('Tempel teks artikel di sini (Ctrl/Cmd+V). Baris pertama yang pendek akan jadi judul.')}
								bind:value={teksTempel}
							></textarea>
							<button class="app-btn app-btn-soft mt-2 w-full" type="button" disabled={!teksTempel.trim()} onclick={pakaiTeksTempel}>
								{tr('Gunakan Teks Ini')}
							</button>
						{@render linkEditor()}
						</div>
					</div>
				{:else}
						<div class="flex h-full min-h-[320px] flex-col items-center py-4 text-center">
							<div class="flex h-12 w-12 items-center justify-center rounded-xl" style="background-color: var(--primary); color: #ffffff;">
								<Sparkles size={22} />
							</div>
							<h3 class="mt-3 text-xl font-bold">{tr('Delegasikan pada The Writer')}</h3>
							<p class="mt-2 max-w-md text-sm text-muted-foreground">
								{tr('Cukup berikan konteks singkat, AI kami akan merangkai artikel yang utuh, relevan, dan teroptimasi SEO.')}
							</p>
							<div class="mt-5 w-full rounded-lg p-3 text-left text-xs" style="background-color: var(--accent); color: var(--accent-foreground);">
								<p class="flex items-center gap-1.5 font-semibold" style="color: var(--primary);">
									<Info size={14} /> {tr('Penyesuaian Gaya Bahasa Otomatis')}
								</p>
								<p class="mt-1 leading-relaxed" style="color: var(--foreground);">
									{tr('AI Tauthub otomatis menyesuaikan gaya tulisan mengikuti kaidah media target: berita jurnalistik untuk media mainstream, format santai dengan heading untuk blog.')}
								</p>
							</div>
							<div class="mt-5 w-full text-left">
								<label class="text-sm font-semibold" for="naskah-topik">
									{tr('Topik & Konteks Pembahasan')} <span style="color: var(--destructive);">*</span>
								</label>
								<div class="relative mt-2">
									<textarea
										id="naskah-topik"
										class="app-input min-h-[120px] w-full resize-y pr-10"
										placeholder={tr('Contoh: Buatkan artikel pengumuman peluncuran software akuntansi baru dari PT Maju Bersama. Tekankan pada fitur rekonsiliasi otomatis dan targetkan pembaca UMKM…')}
										bind:value={topic}
										maxlength={2000}
									></textarea>
									<Sparkles size={16} class="absolute bottom-3 right-3 text-muted-foreground" />
								</div>
								{@render linkEditor()}
							{#if genError}
								<p class="mt-2 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
									<CircleAlert size={15} /> {genError}
								</p>
							{/if}
							<button
								class="app-btn app-btn-primary mt-3 w-full"
								type="button"
								disabled={genBusy || !topic.trim() || !linksValid}
								onclick={generate}
							>
								{#if genBusy}<LoaderCircle size={15} class="animate-spin" />{:else}<Sparkles size={15} />{/if}
								{genBusy ? tr('Menyusun draf…') : tr('Buat Draf dengan AI')}
							</button>
							<p class="mt-2 text-xs text-muted-foreground">
								{tr('Draf akan tampil di tab Tulis / Edit untuk kamu tinjau sebelum disimpan.')}
							</p>
						</div>
					</div>
				{/if}
			</div>

			<!-- Panel pengaturan artikel (kanan) -->
			<aside class="min-w-0 space-y-5 border-t pt-4 lg:border-l lg:border-t-0 lg:pl-5 lg:pt-0" style="border-color: var(--border);">
				<div>
					<div class="flex items-center justify-between gap-2">
						<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
							<ImageIcon size={13} /> {tr('Gambar Utama')}
						</p>
						<span class="rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--destructive); color: #fff;">{tr('WAJIB')}</span>
					</div>
						{#if cover}
							<div class="relative mt-3">
								<img src={cover} alt={tr('Gambar utama artikel')} class="aspect-video w-full rounded-lg border object-cover" style="border-color: var(--border);" />
								<button
								class="absolute right-2 top-2 rounded-lg border bg-card p-1.5 hover:bg-secondary"
								type="button"
								aria-label={tr('Hapus gambar utama')}
								onclick={() => {
								cover = null;
								coverNote = '';
							}}
							>
								<Trash2 size={14} />
							</button>
						</div>
					{:else}
						<div
							class="mt-3 flex cursor-pointer flex-col items-center gap-1.5 rounded-xl border-2 border-dashed py-8 text-center transition-colors"
							class:border-primary={dragging}
							style="border-color: {dragging ? 'var(--primary)' : 'var(--border)'}; background-color: var(--muted);"
							role="button"
							tabindex="0"
							aria-label={tr('Unggah gambar utama')}
							onclick={() => coverInput?.click()}
							onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); coverInput?.click(); } }}
							ondragover={(e) => { e.preventDefault(); dragging = true; }}
							ondragleave={() => (dragging = false)}
							ondrop={onDropCover}
						>
							{#if coverBusy}
								<LoaderCircle size={22} class="animate-spin" style="color: var(--primary);" />
							{:else}
								<Upload size={22} style="color: var(--muted-foreground);" />
							{/if}
							<p class="mt-1 text-xs font-semibold">{tr('Klik, Tarik, atau Tempel Gambar')}</p>
							<p class="text-[11px] text-muted-foreground">{tr('JPG/PNG · otomatis dikompres ke WebP')}</p>
						</div>
					{/if}
					{#if coverError}
						<p class="mt-2 text-xs" style="color: var(--destructive);">{coverError}</p>
					{:else if coverNote}
						<p class="mt-2 text-[11px] text-muted-foreground">{coverNote}</p>
					{/if}
					{#if campaignId && mediaId}
						<button class="app-btn app-btn-soft mt-2 w-full !min-h-8 !px-3 !text-xs" type="button" onclick={() => (coverPickerOpen = true)}>
							<Sparkles size={14} /> {tr('Generate Gambar (AI)')}
						</button>
					{/if}
					{#if !cover}
						<div class="mt-2 text-center">
							<button class="text-xs underline hover:no-underline" type="button" onclick={useFallbackCover}>
								{tr('Pakai thumbnail default Tauthub')}
							</button>
						</div>
					{/if}
					<input
						bind:this={coverInput}
						type="file"
						accept="image/*"
						class="hidden"
						onchange={(e) => {
							const f = (e.currentTarget as HTMLInputElement).files?.[0];
							if (f) void pickCover(f);
						}}
					/>
				</div>

				<div class="border-t pt-4" style="border-color: var(--border);">
					<div class="flex items-center justify-between gap-2">
						<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
							<FileText size={13} /> {tr('Kategori / Rubrik')}
						</p>
						<span class="rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--destructive); color: #fff;">{tr('WAJIB')}</span>
					</div>
					<div class="mt-2">
						<Select
							bind:value={category}
							ariaLabel={tr('Kategori / Rubrik')}
							placeholder={tr('Pilih Rubrik…')}
							options={kategoriOptions}
						/>
					</div>
					<p class="mt-1.5 text-[11px] leading-relaxed text-muted-foreground">
						{tr('*Pilihan kategori diambil sesuai ketersediaan direktori media target.')}
					</p>
				</div>

				<div class="border-t pt-4" style="border-color: var(--border);">
					<div class="flex items-center justify-between gap-2">
						<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr('CUPLIKAN SINGKAT (META DESC)')}</p>
						<span class="rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--muted); color: var(--muted-foreground);">{tr('OPSIONAL')}</span>
					</div>
					<textarea
						class="app-input mt-2 min-h-[90px] w-full resize-y text-sm"
						placeholder={tr('Tulis 1-2 kalimat ringkasan artikel…')}
						bind:value={meta}
						maxlength={500}
					></textarea>
				</div>
			</aside>
		</div>

		<!-- Footer tetap -->
		<div class="flex flex-wrap items-center gap-3 border-t px-5 py-3.5" style="border-color: var(--border);">
			<button class="app-btn app-btn-soft" type="button" disabled={saving} onclick={close}>{tr('Batal')}</button>
			<span class="flex-1"></span>
			{#if pesanKurang}
				<p class="flex min-w-0 items-center gap-1.5 text-xs" style="color: var(--warning, #b45309);">
					<CircleAlert size={14} class="flex-none" />
					<span class="truncate">{pesanKurang}</span>
				</p>
			{:else if saveError}
				<p class="text-xs" style="color: var(--destructive);">{saveError}</p>
			{/if}
			<button class="app-btn app-btn-primary" type="button" disabled={!canSave} onclick={save}>
				{#if saving}<LoaderCircle size={15} class="animate-spin" />{/if}
				{tr('Simpan Artikel')}
			</button>
		</div>
	</div>

		{#if campaignId && mediaId}
			<MediaLibraryPicker
				open={coverPickerOpen}
				{brandId}
				{campaignId}
				{mediaId}
				mode="cover"
				defaultHeading={title}
				onSelect={(url) => {
					cover = url;
					coverError = '';
					coverNote = tr('Diambil dari pustaka gambar AI.');
					coverPickerOpen = false;
				}}
				onClose={() => (coverPickerOpen = false)}
			/>
		{/if}

		<!-- Modal Konfirmasi Generate Gambar H2 (Bukan browser confirm bawaan) -->
		{#if h2ConfirmOpen && h2ConfirmData}
			<div
				class="fixed inset-0 z-[70] flex items-center justify-center p-4"
				style="background-color: rgba(15, 23, 42, 0.55);"
				role="presentation"
				onclick={(e) => {
					if (e.target === e.currentTarget && !h2Busy) h2ConfirmOpen = false;
				}}
				onkeydown={(e) => {
					if (e.key === 'Escape' && !h2Busy) {
						e.stopPropagation();
						h2ConfirmOpen = false;
					}
				}}
			>
					<div
						class="flex w-full max-w-md flex-col overflow-hidden rounded-2xl border shadow-2xl"
						style="border-color: var(--border); background-color: var(--card, #fff);"
						role="dialog"
						aria-modal="true"
						aria-label={tr('Gambar Otomatis Artikel')}
						tabindex="-1"
						onclick={(e) => e.stopPropagation()}
						onkeydown={(e) => e.stopPropagation()}
					>
					<!-- Header -->
					<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
						<div class="flex items-center gap-3">
							<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--accent); color: var(--primary);">
								<Sparkles size={20} />
							</div>
							<div>
								<h4 class="text-sm font-bold leading-tight text-foreground">
									{tr('Gambar Otomatis Artikel')}
								</h4>
								<p class="mt-0.5 text-xs text-muted-foreground">
									{tr('Sistem menyisipkan satu gambar di setiap subjudul H2, lalu menambah gambar isian pada bagian panjang supaya seluruh artikel tetap bergambar.')}
								</p>
							</div>
						</div>
						<button
							class="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
							type="button"
							aria-label={tr('Tutup')}
							disabled={h2Busy}
							onclick={() => (h2ConfirmOpen = false)}
						>
							<X size={16} />
						</button>
					</div>

					<!-- Body -->
					<div class="space-y-4 p-5">
						<!-- Rencana penempatan gambar -->
							<div>
								<div class="flex items-center justify-between text-[10px] font-bold uppercase tracking-wider text-muted-foreground mb-1.5">
									<span>{tr('Rencana Penempatan Gambar')}</span>
									<span>{h2ConfirmData.items.length} {tr('gambar')}</span>
								</div>
							<div class="max-h-36 overflow-y-auto rounded-xl border p-2.5 space-y-1.5" style="border-color: var(--border); background-color: var(--muted, #f8fafc);">
								{#each h2ConfirmData.items as item, idx (idx)}
									<div class="flex items-center gap-2 text-xs">
										<span class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-[9px] font-bold" style="background-color: var(--accent); color: var(--primary);">
											{idx + 1}
										</span>
										<span class="truncate font-medium text-foreground">{item.label}</span>
									</div>
								{/each}
							</div>
						</div>

						<!-- Financial breakdown -->
						<div class="rounded-xl border p-3.5 space-y-2.5" style="border-color: var(--border);">
							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Biaya per Gambar')}</span>
								<span class="font-medium text-foreground">{h2ConfirmData.cost} {tr('Kredit')}</span>
							</div>

							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Total Biaya')} ({h2ConfirmData.items.length} × {h2ConfirmData.cost})</span>
								<div class="text-right">
									<strong class="font-bold text-foreground">{h2ConfirmData.total} {tr('Kredit')}</strong>
									<span class="ml-1 text-[11px] text-muted-foreground">(≈ Rp {(h2ConfirmData.total * 1000).toLocaleString('id-ID')})</span>
								</div>
							</div>

							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Saldo Saat Ini')}</span>
								<span class="font-medium text-foreground">{h2ConfirmData.balance != null ? `${h2ConfirmData.balance.toLocaleString('id-ID')} ${tr('Kredit')}` : '—'}</span>
							</div>

							<div class="h-px w-full" style="background-color: var(--border);"></div>

							<div class="flex items-center justify-between text-xs">
								<span class="font-semibold text-foreground">{tr('Estimasi Sisa Saldo')}</span>
								<strong class="font-bold" style="color: var(--primary);">
									{h2ConfirmData.balance != null ? `${Math.max(0, h2ConfirmData.balance - h2ConfirmData.total).toLocaleString('id-ID')} ${tr('Kredit')}` : '—'}
								</strong>
							</div>
						</div>

						{#if h2ConfirmData.balance != null && h2ConfirmData.balance < h2ConfirmData.total}
							<div class="flex items-center gap-2 rounded-xl p-3 text-xs" style="background-color: var(--destructive-soft, #fef2f2); color: var(--destructive, #dc2626);">
								<CircleAlert size={15} class="shrink-0" />
								<span>{tr('Saldo kredit tidak mencukupi untuk membuat gambar. Silakan top up terlebih dahulu.')}</span>
							</div>
						{/if}
					</div>

					<!-- Footer Actions -->
					<div class="flex items-center justify-end gap-2.5 border-t px-5 py-3.5" style="border-color: var(--border); background-color: var(--muted, #f8fafc);">
						<button
							class="app-btn app-btn-soft text-xs"
							type="button"
							disabled={h2Busy}
							onclick={() => (h2ConfirmOpen = false)}
						>
							{tr('Batal')}
						</button>
						<button
							class="app-btn app-btn-primary text-xs"
							type="button"
							disabled={h2Busy || (h2ConfirmData.balance != null && h2ConfirmData.balance < h2ConfirmData.total)}
							onclick={executeGeneratePerH2}
						>
							{#if h2Busy}
								<LoaderCircle size={14} class="animate-spin" />
								<span>{tr('Memproses…')}</span>
							{:else}
								<Sparkles size={14} />
								<span>{tr('Ya, Generate Semua')} ({h2ConfirmData.total} {tr('Kredit')})</span>
							{/if}
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>
