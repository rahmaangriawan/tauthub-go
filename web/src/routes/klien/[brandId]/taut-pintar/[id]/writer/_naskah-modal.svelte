<script lang="ts">
	// Modal "Pengelola Naskah" — satu tempat untuk menulis, mengunggah dokumen,
	// atau membuat draf AI untuk satu media target, dengan panel pengaturan
	// artikel (cover, kategori, meta deskripsi) dan footer validasi.
	// Reuse: rich-text-editor (toolbar + contentEditable), endpoint /files dan
	// /produce/{mediaId} lewat callback dari writer, serta saveEditor di writer.
	import { X, Upload, FileText, Sparkles, LoaderCircle, Image as ImageIcon, Trash2, CircleAlert, CircleCheck, CircleDot, Info, Link2, Lightbulb, Plus , Zap, Crosshair, Plug  } from '@lucide/svelte';
	import { untrack } from 'svelte';
	import { tr } from '$lib/i18n/phrases';
	import { apiRequest, readApiError } from '$lib/auth';
	import { ARTICLE_DEFAULT_COVER, sameOriginAssetUrl } from '$lib/media/cover';
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';
	import MediaLibraryPicker from '$lib/components/media-library-picker.svelte';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import Select from '$lib/components/select.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { tcToRp } from '$lib/credits/rupiah';
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
		angleOptions = [],
		angleChannelName = '',
		initialAngleSlug = '',
		anglesLoading = false,
		anglesError = '',
		onRetryAngles,
		mediaType = '',
		linkLimit,
		sponsored = false,
		saving = false,
		saveError = null,
		onClose,
		/** Kaba writer bahwa ada aksi yang mengubah tagihan (mis. gambar AI) — bar biaya disegarkan. */
		onRefresh,
		onSave,
		onUploadCover,
		onUploadDoc,
		onGenerate,
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
		angleOptions?: Array<{ slug: string; name: string; description?: string | null; pick_count?: number }>;
		angleChannelName?: string;
		initialAngleSlug?: string;
		anglesLoading?: boolean;
		anglesError?: string;
		onRetryAngles: () => void;
		/** Jenis media target (mainstream | blogger) — untuk keterangan dinamis di header. */
		mediaType?: string;
		/** Media mitra berstatus sponsor: tautan ditulis rel="sponsored", bukan dofollow. */
		sponsored?: boolean;
		linkLimit: number;
		saving?: boolean;
		saveError?: string | null;
		onClose: () => void;
		/** Diberi tahu writer bahwa ada aksi yang mengubah tagihan (mis. gambar AI) — bar biaya disegarkan. */
		onRefresh?: () => void;
		onSave: (title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) => void;
		onUploadCover: (file: File) => Promise<string>;
		onUploadDoc: (file: File, onPhase?: (phase: 'extract' | 'clean') => void, links?: WriterLink[]) => Promise<{ title: string; html: string; warning?: string }>;
		onGenerate: (topic: string, links: WriterLink[], angleSlug: string, proper?: ProperGenPayload) => Promise<{ title: string; html: string; imagesGenerated?: number; imagesCostCredits?: number }>;
		/** null = belum tahu; false = AI Gatekeeper belum dikonfigurasi admin. */
			/** Pratinjau streaming: html parsial yang tampil read-only saat AI menulis; null = tidak streaming. */
		streamingHtml?: string | null;
	} = $props();

// Nilai awal disalin sekali per mount — writer memakai {#key mediaId}
// sehingga setiap buka modal adalah instance baru.
	type Tab = 'tulis' | 'unggah' | 'generate';
	/** Isian mode "Yang Proper" dikirim ke backend generate (grounding ke fakta). */
	type ProperGenPayload = { fakta: string; dateline: string; narasumber: string; kutipan: string; fokus: string; brief_id: string };
	let tab = $state<Tab>(untrack(() => initialTab));
	// Pilihan gaya (1. kamu mau buat artikel seperti apa?) — simpel = pipeline
	// lama; proper = strict grounding ke fakta terkunci/Brief Copilot.
	let gayaArtikel = $state<'simpel' | 'proper'>('simpel');
	// Isian mode Proper (copilot & paste) — UI dulu, logika menyusul.
	let properPaste = $state('');
	// Racik instruksi topik dari anchor (AI): ganti Tips/Contoh manual.
	let racikBusy = $state(false);
	let racikError = $state('');
	// Isian Proper: fakta terkunci, dateline & narasumber.
	let faktaTerkunci = $state('');
	let kotaRilis = $state('');
	let narasumber = $state('');
	let kutipanNarasumber = $state('');
	let fokusSisa = $state('');
	let title = $state(untrack(() => initialTitle));
	let html = $state(untrack(() => initialHtml));
	let cover = $state<string | null>(untrack(() => initialCoverUrl));
	let category = $state<string>(untrack(() => initialCategory) ?? '');
	// Rubrik yang dipilih otomatis setelah draf AI jadi (bukan pilihan user) —
	// dipakai untuk menampilkan keterangan kecil di bawah dropdown rubrik.
	let autoCategory = $state('');
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
	// Panel progres generate. Tahap mengikuti kondisi nyata request yang sudah
	// ada — respons /produce tidak membawa progres internal AI, jadi timeline
	// disusun dari siklus request (kirim → menunggu respons → memproses hasil):
	// tanpa persentase fiktif, tanpa interval palsu.
	let genStage = $state(0); // 0 idle; 1 mengirim; 2 menunggu respons; 3 memproses
	let genOutcome = $state<'idle' | 'success' | 'error'>('idle');
	const GEN_STAGES = [
		{ key: 'send', label: 'Mengirim permintaan', desc: 'Permintaan generate dikirim ke AI Writer.' },
		{ key: 'wait', label: 'Menunggu respons AI', desc: 'AI sedang menulis artikel sesuai topik dan angle yang kamu pilih.' },
		{ key: 'process', label: 'Memproses hasil', desc: 'Draf tersimpan diambil dan disiapkan di tab Editor Post.' }
	] as const;
	const genStagePct = $derived.by(() => {
		if (genOutcome === 'success') return 100;
		if (genOutcome === 'error') return 0;
		return Math.round(((genStage - 1) / GEN_STAGES.length) * 100);
	});
	let topic = $state('');
	// (Tips/Contoh manual digantikan tombol "Racik Instruksi dari Anchor" — AI.)
	let angleSlug = $state(untrack(() => initialAngleSlug));
	const selectedAngle = $derived(angleOptions.find((a) => a.slug === angleSlug));
	// Urut populer: jumlah pemilihan angle oleh seluruh klien (pick_count dari
	// server). Select menampilkan N teratas + tombol "Lihat semua" + pencarian.
	const angleSelectOptions = $derived(
		[...angleOptions]
			.sort((a, b) => (b.pick_count ?? 0) - (a.pick_count ?? 0) || tr(a.name).localeCompare(tr(b.name)))
			.map((a) => ({ value: a.slug, label: tr(a.name), description: tr(a.description ?? '') }))
	);
	const angleReady = $derived(!anglesLoading && !anglesError && !!selectedAngle);
	// Catat pilihan angle untuk statistik populer. Kegagalan diabaikan: ini
	// telemetri, bukan syarat generate.
	async function recordAnglePick(slug: string) {
		if (!slug || !brandId || !campaignId) return;
		try {
			await apiRequest(
				`/v1/brands/${brandId}/taut/campaigns/${campaignId}/angles/${encodeURIComponent(slug)}/pick`,
				{ method: 'POST', body: JSON.stringify({ media_id: mediaId }) }
			);
		} catch {
			/* opsional */
		}
	}
	$effect(() => {
		if (!anglesLoading && !anglesError && angleSlug && !selectedAngle) angleSlug = '';
	});
	// Transparansi biaya gambar isi: kalau topik/konteks meminta gambar, tampilkan
	// perkiraan biayanya. Harga per gambar diambil dari endpoint pustaka gambar.
	let imgCost = $state(0);
	let imgRateIdr = $state(0);
	let imgPricingLoaded = $state(false);
	let genNotice = $state('');
	const IMG_WORD_RE = /\b(gambar|images?|foto|ilustrasi|infografis)\b/i;
	const IMG_COUNT_RE = /(\d+)\s*(gambar|foto|images?|ilustrasi|infografis)/i;
	// Sanggahan: "tanpa gambar", "jangan pakai gambar", "tidak perlu ilustrasi".
	// Negasi dinilai per klausa (dipisah tanda baca) supaya klien yang menolak
	// gambar tidak dikenai estimasi biaya gambar — tapi permintaan campuran
	// ("tanpa gambar utama, tambahkan 2 gambar isi") tetap terdeteksi.
	const IMG_NEG_RE = /\b(tanpa|jangan|tidak|tdk|tak|gak|nggak|hindari|no|without)\b/i;
	const IMG_CLAUSE_SPLIT_RE = /[.,;!?()\[\]{}"\r\n]+/;
	function topicRequestsImages(text: string): boolean {
		for (const clause of text.split(IMG_CLAUSE_SPLIT_RE)) {
			if (!IMG_WORD_RE.test(clause)) continue;
			if (IMG_NEG_RE.test(clause)) continue;
			return true;
		}
		return false;
	}
	const imgRequestCount = $derived.by(() => {
		if (!topicRequestsImages(topic)) return 0;
		const m = IMG_COUNT_RE.exec(topic);
		const n = m ? Number.parseInt(m[1], 10) : 1;
		if (!Number.isFinite(n) || n < 1) return 1;
		return Math.min(n, 5);
	});
	const imgEstimateRupiah = $derived(imgRequestCount * imgCost * imgRateIdr);
	// Permintaan rel di teks topik vs cakupan media mitra: media sponsor menulis
	// rel="sponsored", media biasa menulis dofollow (tanpa rel). Pipeline
	// mengabaikan permintaan di luar cakupan itu, jadi klien diberi tahu di
	// bawah kolom topik alih-alih diam-diam tidak dipenuhi.
	const REL_TERMS: { term: 'sponsored' | 'nofollow' | 'dofollow'; re: RegExp }[] = [
		{ term: 'sponsored', re: /\bsponsored\b/i },
		{ term: 'nofollow', re: /\bno[-\s]?follow\b/i },
		{ term: 'dofollow', re: /\bdo[-\s]?follow\b/i }
	];
	const REL_NEG_RE = /\b(tanpa|jangan|tidak|tdk|tak|gak|nggak|hindari|bukan)\b/i;
	const relNotices = $derived.by(() => {
		if (linkLimit <= 0) return [];
		const out: string[] = [];
		const seen = new Set<string>();
		for (const clause of topic.split(IMG_CLAUSE_SPLIT_RE)) {
			if (REL_NEG_RE.test(clause)) continue;
			for (const { term, re } of REL_TERMS) {
				if (seen.has(term) || !re.test(clause)) continue;
				seen.add(term);
				if (term === 'sponsored' && !sponsored) {
					out.push(tr('rel="sponsored" tidak diperbolehkan di media ini — tautan ditulis dofollow (tanpa rel).'));
				} else if (term === 'dofollow' && sponsored) {
					out.push(tr('rel dofollow tidak diperbolehkan di media ini — tautan ditulis rel="sponsored".'));
				} else if (term === 'nofollow') {
					out.push(
						sponsored
							? tr('rel="nofollow" tidak diperbolehkan di media ini — tautan ditulis rel="sponsored".')
							: tr('rel="nofollow" tidak diperbolehkan di media ini — tautan ditulis dofollow (tanpa rel).')
					);
				}
			}
		}
		return out;
	});
	$effect(() => {
		if (imgPricingLoaded || !campaignId || !mediaId) return;
		imgPricingLoaded = true;
		void (async () => {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}/images?media_id=${mediaId}`);
			if (!res.ok) return;
			const d = await res.json().catch(() => null);
			if (d) {
				imgCost = Number(d.cost_credits ?? 0) || 0;
				imgRateIdr = Number(d.rate_idr ?? 0) || 0;
			}
		})();
	});
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
			rateIdr: number;
		} | null>(null);
		let h2Error = $state('');
		let h2Note = $state('');
		let imageCost = $state(0);
		let imageRateIdr = $state(0);

		// Biaya & saldo gambar selalu tampil dalam rupiah (kurs aktif dari server).
		const rpOf = (tc: number, rate: number) => (tc > 0 ? tcToRp(tc, rate) : tr('Gratis'));
		const h2CostRp = $derived(h2ConfirmData ? rpOf(h2ConfirmData.cost, h2ConfirmData.rateIdr) : '—');
		const h2BalanceRp = $derived(h2ConfirmData?.balance == null ? '—' : tcToRp(h2ConfirmData.balance, h2ConfirmData.rateIdr));

			// Jumlah gambar bisa diatur pengguna. Bawaannya 1 — rencana tetap
			// menghitung penempatan sampai N gambar, tapi pengguna yang menaikkan.
			let h2Count = $state(1);
			const h2Planned = $derived(h2ConfirmData?.items.length ?? 0);
			const h2Chosen = $derived(Math.max(1, Math.min(h2Count || 1, Math.max(1, h2Planned))));
		const h2TotalCredits = $derived((h2ConfirmData?.cost ?? 0) * h2Chosen);
		const h2TotalRp = $derived(h2ConfirmData ? rpOf(h2TotalCredits, h2ConfirmData.rateIdr) : '—');
		const h2RemainingRp = $derived(
			h2ConfirmData?.balance == null ? '—' : tcToRp(Math.max(0, h2ConfirmData.balance - h2TotalCredits), h2ConfirmData.rateIdr)
		);

		// Saat draf sedang disusun (generate AI / olah dokumen), modal tidak
		// boleh ditutup: prosesnya tetap berjalan di server dan artikel +
		// kreditnya tetap tercatat, jadi menutup di tengah proses hanya membuat
		// user mengira pekerjaannya batal. Klik tutup malah memunculkan
		// peringatan untuk menunggu.
		const composeBusy = $derived(genBusy || docBusy);
		let closeNotice = $state('');
		function close() {
			if (saving) return;
			if (composeBusy) {
				closeNotice = tr('Draf sedang disusun — tunggu sampai prosesnya selesai sebelum menutup.');
				return;
			}
			onClose();
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
	// Satuan paling ringkas + koma desimal ala Indonesia: 3,4MB, bukan 3422KB.
	const KB = (b: number) => (b >= 1024 * 1024 ? (b / 1024 / 1024).toFixed(1).replace('.', ',') + 'MB' : Math.max(1, Math.round(b / 1024)) + 'KB');

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

	// Ambil biaya per gambar + saldo + kurs dari pustaka (untuk konfirmasi biaya).
	async function fetchImageCost(): Promise<{ cost: number; balance: number | null }> {
		try {
			const res = await apiRequest(`${apiBase}/images?media_id=${encodeURIComponent(mediaId)}`);
			if (!res.ok) return { cost: imageCost, balance: null };
			const d = await res.json().catch(() => null);
			imageCost = Number(d?.cost_credits ?? imageCost);
			if (Number(d?.rate_idr) > 0) imageRateIdr = Number(d.rate_idr);
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
		if (Number(d?.rate_idr) > 0) imageRateIdr = Number(d.rate_idr);
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
			h2Count = 1;
			h2ConfirmData = {
				items: plan.map((p) => ({ label: p.label, prompt: p.prompt })),
				cost,
				balance,
				total,
				rateIdr: imageRateIdr
			};
			h2ConfirmOpen = true;
		}

		async function executeGeneratePerH2() {
			if (h2Busy || !h2ConfirmData) return;
			const chosen = Math.max(1, Math.min(h2Count || 1, h2ConfirmData.items.length));
			const items = h2ConfirmData.items.slice(0, chosen);
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
				// Setiap gambar AI ditagih saat dibuat — beri tahu writer agar bar
				// Total Tagihan Akhir ikut diperbarui.
				void onRefresh?.();
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
			closeNotice = '';
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
	async function fetchSavedArticle(): Promise<{ title: string; html: string; metaDescription: string } | null> {
		if (!campaignId || !mediaId) return null;
		try {
			const res = await apiRequest(`${apiBase}/articles/${mediaId}`);
			if (!res.ok) return null;
			const d = await res.json().catch(() => null);
			if (!d) return null;
			return {
				title: String(d.title ?? ''),
				html: String(d.content_html ?? ''),
				metaDescription: String(d.meta_description ?? '')
			};
		} catch {
			return null;
		}
	}

	// Susun instruksi topik otomatis dari anchor yang diisi user — memakai AI
	// yang dikonfigurasi admin (tier default). Aturan: judul/arahan jangan
	// menyalin anchor persis; pakai kata kunci semantik & kaidah SEO/EEAT, dan
	// bila ada tautan, instruksi menuntut penyisipan yang natural.
	async function racikInstruksi() {
		if (racikBusy) return;
		const rows = linkRows.filter((r) => (r.anchor ?? '').trim());
		if (rows.length === 0) return;
		racikBusy = true;
		try {
			const res = await apiRequest(`${apiBase}/produce/topic-instruction`, {
				method: 'POST',
				body: JSON.stringify({
					media_id: mediaId,
					media_type: mediaType,
					anchors: rows.map((r) => ({ anchor: r.anchor, url: r.url }))
				})
			}, 180_000);
			if (!res.ok) {
				racikError = await readApiError(res, tr('Gagal meracik instruksi. Coba lagi.'));
				return;
			}
			const data = await res.json().catch(() => null);
			if (data?.instruction) topic = String(data.instruction);
		} catch (reason) {
			racikError = reason instanceof Error ? reason.message : tr('Gagal meracik instruksi. Coba lagi.');
		} finally {
			racikBusy = false;
		}
	}

	// Memori Brand: fakta diserap dari Brief Copilot (Profiler) brand. Dropdown
	// milih brief mana yang jadi sumbernya, lalu tombol Tarik mengisi kotak
	// fakta dari nilai tersimpan brief tersebut — tanpa teks tetap di UI.
	const profilerBase = $derived(`/v1/brands/${brandId}/profiler`);
	// Alamat halaman Profiler untuk tombol di peringatan — dibuka tab baru
	// supaya isi popup nggak hilang saat user menyiapkan brief-nya. Memakai
	// brandId route (bukan cache brand global) agar selalu ke brand kampanye ini.
	const profilerHref = $derived(`/klien/${brandId}/profiler`);
	let copilotBriefs = $state<{ id: string; name: string }[]>([]);
	let copilotBriefsLoaded = $state(false);
	let copilotBriefId = $state('');
	let copilotBriefName = $state('');
	let copilotEntries = $state<{ key: string; label: string; value: string }[]>([]);
	let copilotLoadedBriefId = $state('');
	let copilotBusy = $state(false);
	let copilotError = $state('');
	const copilotBriefOptions = $derived(copilotBriefs.map((b) => ({ value: b.id, label: b.name })));
	// Nama pada baris Brand mengikuti nama brief (Profiler) yang dipilih —
	// nilai canonical_name di fakta jadi cadangan bila brief tidak punya nama.
	const copilotBrandName = $derived.by(() => {
		const canonical = copilotEntries.find((e) => e.key === 'canonical_name')?.value ?? '';
		return copilotBriefName || canonical;
	});
	const copilotSummary = $derived.by(() => {
		if (copilotEntries.length === 0) return '';
		const val = (k: string) => copilotEntries.find((e) => e.key === k)?.value ?? '';
		return [
			val('entity_category') || val('canonical_links') ? val('entity_category') : '',
			val('canonical_name'),
			`${copilotEntries.length} ${tr('fakta tersimpan')}`
		].filter(Boolean).join(' • ');
	});

	$effect(() => {
		if (gayaArtikel === 'proper' && copilotBriefs.length === 0 && !copilotBusy) {
			void loadCopilotBriefs();
		}
	});

	// Label panjang skema field diringkas: buang keterangan dalam tanda
	// kurung/plus/slash supaya pas dipakai sebagai awalan baris fakta.
	function copilotShortLabel(raw: string): string {
		const cut = raw.search(/\(|\[|\+|\//);
		const head = (cut > 0 ? raw.slice(0, cut) : raw).trim();
		return head || raw;
	}

	function copilotFallbackLabel(key: string): string {
		const known: Record<string, string> = {
			canonical_name: tr('Nama brand'),
			canonical_description: tr('Deskripsi brand'),
			entity_category: tr('Kategori entitas'),
			canonical_links: tr('Tautan resmi'),
			official_facts: tr('Fakta resmi'),
			allowed_claims: tr('Klaim yang boleh'),
			forbidden_claims: tr('Klaim yang dilarang')
		};
		return known[key] ?? key.replace(/_/g, ' ');
	}

	function copilotCollectEntries(values: Record<string, unknown>, fields: any[]): { key: string; label: string; value: string }[] {
		const meta = new Map<string, { label: string; order: number }>();
		fields.forEach((f, i) => {
			if (f?.key) meta.set(String(f.key), { label: String(f.label ?? f.key), order: Number(f.sort_order ?? i) });
		});
		const rows: { key: string; label: string; value: string; order: number }[] = [];
		for (const [key, raw] of Object.entries(values)) {
			let text = '';
			if (typeof raw === 'string') text = raw;
			else if (Array.isArray(raw)) text = raw.map((v) => (v == null ? '' : String(v))).filter(Boolean).join('; ');
			else if (raw != null) text = String(raw);
			if (!text.trim()) continue;
			const m = meta.get(key);
			rows.push({ key, label: m ? copilotShortLabel(m.label) : copilotFallbackLabel(key), value: text, order: m?.order ?? 900 + rows.length });
		}
		rows.sort((a, b) => a.order - b.order);
		return rows.map(({ key, label, value }) => ({ key, label, value }));
	}

	async function loadCopilotBriefs(): Promise<boolean> {
		if (copilotBriefsLoaded || copilotBusy) return copilotBriefs.length > 0;
		copilotBusy = true;
		try {
			const res = await apiRequest(`${profilerBase}/profiles`);
			if (!res.ok) {
				copilotError = await readApiError(res, tr('Gagal memuat data Brief Copilot.'));
				return false;
			}
			const data = await res.json().catch(() => null);
			copilotBriefs = (data?.profiles ?? []).map((p: any) => ({ id: String(p.id), name: String(p.name ?? tr('Brief')) }));
			return copilotBriefs.length > 0;
		} catch (reason) {
			copilotError = reason instanceof Error ? reason.message : tr('Gagal memuat data Brief Copilot.');
			return false;
		} finally {
			copilotBriefsLoaded = true;
			copilotBusy = false;
		}
	}

	async function pickCopilotBrief(id: string) {
		copilotBriefId = id;
		copilotBriefName = copilotBriefs.find((b) => b.id === id)?.name ?? copilotBriefName;
		copilotEntries = [];
		copilotLoadedBriefId = '';
		copilotError = '';
		if (!id) return;
		copilotBusy = true;
		try {
			const res = await apiRequest(`${profilerBase}/profiles/${encodeURIComponent(id)}/brief/chat`);
			if (!res.ok) {
				copilotError = await readApiError(res, tr('Gagal memuat data Brief Copilot.'));
				return;
			}
			const data = await res.json().catch(() => null);
			copilotBriefName = String(data?.profile?.name ?? copilotBriefName);
			copilotEntries = copilotCollectEntries(data?.values ?? {}, Array.isArray(data?.fields) ? data.fields : []);
			copilotLoadedBriefId = id;
		} catch (reason) {
			copilotError = reason instanceof Error ? reason.message : tr('Gagal memuat data Brief Copilot.');
		} finally {
			copilotBusy = false;
		}
	}

	async function tarikFaktaCopilot() {
		if (copilotBusy) return;
		copilotError = '';
		if (!(await loadCopilotBriefs())) return;
		if (copilotBriefs.length === 0) {
			// Peringatan "belum ada brief" sudah tampil di strip seksi ini.
			return;
		}
		if (!copilotBriefId || copilotLoadedBriefId !== copilotBriefId) {
			await pickCopilotBrief(copilotBriefId || copilotBriefs[0].id);
		}
		if (copilotEntries.length === 0) {
			copilotError = tr('Brief ini belum menyimpan fakta — lengkapi dulu percakapannya di halaman Taut Brief.');
			return;
		}
		faktaTerkunci = copilotEntries.map((e) => `${e.label}: ${e.value}`).join('\n\n');
	}

	/** Isian mode "Yang Proper" — grounding ke fakta (dikirim saat gaya proper). */
	const properLocked = $derived(
		faktaTerkunci.trim() === '' && (copilotBriefId === '' || copilotEntries.length === 0)
	);

	async function generate() {
		if (genBusy) return;
		const isProper = gayaArtikel === 'proper';
		const properPayload: ProperGenPayload = {
			fakta: faktaTerkunci.trim(),
			dateline: kotaRilis.trim(),
			narasumber: narasumber.trim(),
			kutipan: kutipanNarasumber.trim(),
			fokus: fokusSisa.trim(),
			brief_id: copilotBriefId
		};
		if (isProper) {
			// Proper wajib punya dasar fakta: manual ATAU Brief Copilot. Selama
			// keduanya kosong, ditolak sebelum menyia-nyiakan token AI.
			if (!properPayload.fakta && !properPayload.brief_id) {
				genError = tr('Isi fakta produk atau pilih Brief Copilot dulu — mode Proper ditulis terkunci ke fakta.');
				return;
			}
		} else if (!topic.trim()) {
			return;
		}
		if ((linkLimit > 0 && !linksValid) || !angleReady) return;
		genBusy = true;
		genError = '';
		genNotice = '';
		genOutcome = 'idle';
		genStage = 1; // permintaan dikirim
		try {
			genStage = 2; // menunggu respons AI
			const hasil = await onGenerate(isProper ? '' : topic.trim(), linkLimit > 0 ? payloadLinks : [], angleSlug, isProper ? properPayload : undefined);
			genStage = 3; // memproses hasil (muat draf tersimpan + isi editor)
			title = hasil.title || title;
			if (hasil.imagesGenerated && hasil.imagesGenerated > 0) {
				const kredit = hasil.imagesCostCredits ?? 0;
				genNotice = tr('Gambar isi dibuat otomatis') + `: ${hasil.imagesGenerated}` + (kredit > 0 ? ` (${kredit} ${tr('kredit')})` : '') + '.';
			}
			// Respons generate tidak memuat HTML, jadi ambil draf tersimpan agar
			// editor langsung terisi dan tab berpindah ke Editor Post.
			const draf = await fetchSavedArticle();
			if (hasil.html && hasil.html.trim()) {
				html = hasil.html;
			} else if (draf?.html) {
				html = draf.html;
				if (draf.title) title = draf.title;
			}
			// Tiap generate menulis versi + meta deskripsi BARU di server. Kolom
			// meta harus ikut disegarkan; kalau tidak, ringkasan artikel lama
			// tertinggal di sini dan simpan berikutnya malah menimpakannya ke
			// artikel baru (ikut terbawa sampai terbit).
			if (draf?.metaDescription) meta = draf.metaDescription;
			tab = 'tulis';
			// Rubrik wajib ada sebelum simpan; setelah draf AI jadi, pilih
			// otomatis dari rubrik yang tersedia agar tidak menahan proses.
			// User tetap bisa menggantinya di panel pengaturan.
			if (!category.trim() && categoryOptions.length > 0) {
				category = categoryOptions[0];
				autoCategory = category;
			}
			genOutcome = 'success';
		} catch (reason) {
			genError = reason instanceof Error ? tr(reason.message) : tr('Gagal membuat draf AI.');
			genOutcome = 'error';
		} finally {
			genBusy = false;
			closeNotice = '';
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
	const canSave = $derived(!!cover && alasanLain.length === 0 && !saving);

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

	/** Simpan artikel apa adanya. Pemeriksaan QC tidak lagi dipicu dari sini. */
	async function save() {
		if (!canSave || saving) return;
		onSave(title.trim(), html, cover, category, meta.trim());
	}

	const kategoriOptions = $derived(categoryOptions.map((c) => ({ value: c, label: c })));
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6" style="background-color: rgba(15, 23, 42, 0.45);">
	<div
		bind:this={panelEl}
		class="flex max-h-[94vh] w-full max-w-[71rem] flex-col overflow-hidden rounded-2xl border bg-card shadow-2xl"
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
				<p class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
					<span class="truncate">{tr('Media Target:')} <strong class="text-foreground">{mediaName}</strong></span>
					{#if mediaType}
						<span
							class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide"
							style="background: {mediaType === 'blogger' ? 'var(--warning-soft)' : 'var(--accent)'}; color: {mediaType === 'blogger' ? 'var(--warning)' : 'var(--primary)'};"
						>{mediaType === 'blogger' ? tr('Media Blogger') : tr('Media Mainstream')}</span>
					{/if}
				</p>
			</div>
			<button
				class={`rounded-lg p-1.5 ${composeBusy ? 'cursor-not-allowed opacity-40' : 'hover:bg-secondary'}`}
				type="button"
				aria-label={tr('Tutup')}
				aria-disabled={composeBusy || saving}
				onclick={close}
			>
				<X size={18} />
			</button>
		</div>

		<!-- Tab -->
		<div class="border-b px-5 pt-3" style="border-color: var(--border);">
			<div class="flex flex-wrap gap-1" role="tablist" aria-label={tr('Cara menyusun naskah')}>
				{#each [
					{ key: 'unggah', label: tr('Unggah Dokumen') },
					{ key: 'generate', label: tr('Generate dengan AI') },
					{ key: 'tulis', label: tr('Editor Post') }
				] as t (t.key)}
						<button
							class="relative -mb-px flex items-center gap-1.5 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors"
							style="border-color: {tab === t.key ? 'var(--primary)' : 'transparent'}; color: {tab === t.key ? 'var(--primary)' : 'var(--muted-foreground)'};"
							type="button"
							role="tab"
							aria-selected={tab === t.key}
							title={t.key === 'generate' ? tr('Fitur ini masih dalam tahap beta. Hasil draf artikel AI masih perlu dikaji dan diperiksa ulang sebelum diterbitkan.') : undefined}
							onclick={() => (tab = t.key as Tab)}
						>
							{#if t.key === 'generate'}
								<div class="pointer-events-auto absolute -top-1.5 right-0">
									<Tooltip content={tr('Fitur ini masih dalam tahap beta. Hasil draf artikel AI masih perlu dikaji dan diperiksa ulang sebelum diterbitkan.')} maxWidth={280}>
										<span
											class="cursor-help text-[9px] font-bold uppercase tracking-wide leading-none transition-opacity hover:opacity-80"
											style="color: var(--primary);">{tr('Beta')}</span
										>
									</Tooltip>
								</div>
							{/if}
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

	{#snippet numBadge(n: string)}
		<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-base font-bold" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">{n}</span>
	{/snippet}

	{#snippet linkEditor(sub = ' (Opsional)', note = '')}
		<div class="w-full text-left">
				<div class="flex items-center gap-3">
					{@render numBadge('3')}
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between gap-2">
							<p class="min-w-0 text-sm font-semibold leading-5">
								{tr('Tautan & Anchor')}{sub}
							</p>
							<span
								class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-bold"
								style={linkRows.length >= maxLinkRows
									? 'background-color: var(--primary); color: #fff;'
									: 'background-color: var(--muted); color: var(--muted-foreground);'}
							>{linkRows.length}/{maxLinkRows}</span>
						</div>
						{#if note}
							<p class="mt-1 text-[11px] leading-relaxed text-muted-foreground">{note}</p>
						{/if}
					</div>
				</div>

			{#if linkRows.length === 0}
				<p class="mt-3 rounded-lg border border-dashed px-3 py-4 text-left text-xs text-muted-foreground" style="border-color: var(--border);">
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
		<!-- Padding ATAS sengaja 0: toolbar rich-text editor di kolom kiri sticky
		     top-0, jadi harus menempel persis di bawah tab tanpa celah padding
		     (dulu isi artikel terlihat lewat celah 20px itu). Jaraknya dipindah
		     ke padding kolom. -->
		<!-- Kolom kanan hanya ada di tab Editor Post; tab lain menempati lebar penuh. -->
		<div class="grid flex-1 gap-5 overflow-y-auto px-5 pb-5 {tab === 'tulis' ? 'lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]' : ''}">
			<!-- Kolom kerja utama -->
			<div class="min-w-0 pt-5">
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
					<div class="mt-4">
						<div class="flex items-center justify-between gap-4">
							<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{tr('Isi Artikel (Konten)')}</p>
							<p class="shrink-0 text-xs text-muted-foreground">{editorStats.words} {tr('kata')}</p>
						</div>
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
					<!-- Dua jalur berdampingan: file upload (kiri) & tempel teks (kanan). -->
					<div class="flex h-full min-h-[320px] flex-col justify-center py-6">
						<div class="text-center">
							<h3 class="text-xl font-bold">{tr('Punya Draf Sendiri?')}</h3>
						</div>
						<!-- Kolom kiri & kanan sama tinggi (stretch). -->
						<div class="mt-6 grid flex-1 gap-5 lg:grid-cols-2">
							<div class="flex flex-col">
						<div
							class="flex h-full w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed p-8 transition-colors"
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
								<p class="mt-3 flex items-center justify-center gap-1.5 text-sm" style="color: var(--destructive);">
									<CircleAlert size={15} /> {docError}
								</p>
							{/if}
							</div>

							<div class="flex h-full flex-col text-left">
								<textarea
									class="app-input mt-3 min-h-[220px] w-full flex-1 resize-y text-sm"
									placeholder={tr('Tempel teks artikel di sini (Ctrl/Cmd+V). Baris pertama yang pendek akan jadi judul.')}
									bind:value={teksTempel}
								></textarea>
								<button class="app-btn app-btn-soft mt-2 w-full" type="button" disabled={!teksTempel.trim()} onclick={pakaiTeksTempel}>
									{tr('Gunakan Teks Ini')}
								</button>
								<p class="mt-2 text-left text-xs leading-relaxed text-muted-foreground">
									{tr('Artikel akan 100% memakai dokumen yang kamu unggah — tanpa ditulis ulang oleh AI. Sistem tetap memeriksa tautan dan memvalidasi bahwa dokumen ini aman untuk diterbitkan.')}
								</p>
							</div>
						</div>
					</div>
				{:else}
					<div class="flex h-full min-h-[320px] flex-col py-4">
						<!-- Banner standar penulisan (menggantikan hero + BETA + kotak info). -->
						<div class="rounded-xl border p-4 text-left" style="border-color: color-mix(in srgb, var(--primary) 25%, transparent); background-color: color-mix(in srgb, var(--primary) 6%, transparent);">
							<div class="flex items-start gap-3">
								<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--primary); color: #ffffff;">
									<Sparkles size={18} />
								</div>
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<p class="text-sm font-bold">{tr('Delegasikan pada The Writer')}</p>
										<span class="rounded-full px-2.5 py-0.5 text-[10px] font-bold" style="background-color: color-mix(in srgb, var(--primary) 12%, transparent); color: var(--primary);">
											{mediaType === 'blogger' ? tr('Standar Blog Santai') : mediaType === 'mainstream' ? tr('Standar Jurnalistik Mainstream') : tr('Standar Penulisan Artikel')}
										</span>
									</div>
									<p class="mt-0.5 text-xs leading-relaxed text-muted-foreground">
										{#if mediaType === 'blogger'}
											{tr('AI otomatis mengikuti kaidah')} <strong>{tr('Media Blogger')}</strong> {tr('(gaya santai mengalir dengan heading, tetap akurat dan bebas bahasa sales berlebihan).')}
										{:else if mediaType === 'mainstream'}
											{tr('AI otomatis mengikuti kaidah')} <strong>{tr('Media Mainstream')}</strong> {tr('(gaya berita/feature objektif, piramida terbalik, dan bebas bahasa sales berlebihan).')}
										{:else}
											{tr('AI otomatis mengikuti kaidah kanal media terpilih — berita mengalir tanpa heading, tanpa bahasa sales berlebihan.')}
										{/if}
									</p>
								</div>
							</div>
						</div>

						<!-- 1. Pilihan gaya artikel: dua kartu besar dengan ilustrasi
						     aset 3D + chip keunggulan. -->
						<div class="mt-6">
							<div class="mb-3 flex flex-wrap items-start justify-between gap-3">
								<div class="flex items-start gap-3">
									<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-base font-bold" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">1</span>
									<div>
										<p class="text-sm font-bold">{tr('Kamu mau buat artikel seperti apa?')} <span style="color: var(--destructive);">*</span></p>
										<p class="mt-0.5 text-[11px] text-muted-foreground">{tr('Pilih sesuai tujuan publikasi dan kebutuhan konten kamu.')}</p>
									</div>
								</div>
								<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-semibold" style="background-color: color-mix(in srgb, var(--primary) 8%, transparent); color: var(--primary);">
									<CircleDot size={12} /> {tr('Pilih sesuai tujuan publikasi')}
								</span>
							</div>
							<div class="grid gap-4 lg:grid-cols-2">
								<button
									type="button"
									class="relative overflow-hidden rounded-xl border-2 p-4 text-left transition-colors {gayaArtikel === 'simpel'
										? 'border-amber-400 bg-amber-50/50'
										: 'border-slate-200 bg-slate-50 hover:border-slate-300'}"
									onclick={() => (gayaArtikel = 'simpel')}
								>
									<img src="/img/writer/gaya-sederhana.webp" alt={tr('Ilustrasi artikel sederhana')} loading="lazy" class="pointer-events-none absolute inset-y-0 right-0 w-[44%] select-none object-cover object-right transition-all {gayaArtikel === 'simpel' ? '' : 'opacity-55 grayscale'}" />
									<div class="relative flex items-start gap-3 {gayaArtikel === 'simpel' ? '' : 'opacity-70'}">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning, #b45309);">
											<Zap size={18} />
										</div>
										<div>
											<p class="text-sm font-bold">{tr('Sederhana Aja')}</p>
											<span class="mt-1 inline-flex items-center rounded-full px-2 py-0.5 text-[10.5px] font-semibold" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning, #b45309);">
												{tr('Cepat & Praktis')}
											</span>
										</div>
									</div>
									<span class="absolute right-3 top-3 inline-flex h-4.5 w-4.5 shrink-0 items-center justify-center rounded-full border-2 {gayaArtikel === 'simpel' ? 'border-amber-500' : 'border-slate-300'} bg-white">
										{#if gayaArtikel === 'simpel'}<span class="h-2 w-2 rounded-full bg-amber-500"></span>{/if}
									</span>
									<p class="relative mt-2 max-w-[58%] text-xs leading-relaxed text-muted-foreground">
										{tr('Cukup instruksi sederhana, cocok untuk artikel brief ringan')}
									</p>
									<span class="relative mt-3 inline-flex items-center gap-1.5 rounded-full bg-blue-50 px-2.5 py-1 text-[10.5px] font-semibold text-blue-700 {gayaArtikel === 'simpel' ? '' : 'opacity-70'}">
										<Zap size={12} class="shrink-0" /> {tr('Tanpa Perlu data perusahaan / Copilot')}
									</span>
								</button>
								<button
									type="button"
									class="relative overflow-hidden rounded-xl border-2 p-4 text-left transition-colors {gayaArtikel === 'proper'
										? 'border-blue-500 bg-blue-50/40'
										: 'border-slate-200 bg-slate-50 hover:border-slate-300'}"
									onclick={() => (gayaArtikel = 'proper')}
								>
									<img src="/img/writer/gaya-proper.webp" alt={tr('Ilustrasi artikel yang proper')} loading="lazy" class="pointer-events-none absolute inset-y-0 right-0 w-[44%] select-none object-cover object-right transition-all {gayaArtikel === 'proper' ? '' : 'opacity-55 grayscale'}" />
									<div class="relative flex items-start gap-3 {gayaArtikel === 'proper' ? '' : 'opacity-70'}">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">
											<Crosshair size={17} />
										</div>
										<div>
											<p class="text-sm font-bold">{tr('Yang Proper')}</p>
											<span class="mt-1 inline-flex items-center rounded-full px-2 py-0.5 text-[10.5px] font-semibold" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">
												{tr('Lebih Lengkap & Presisi')}
											</span>
										</div>
									</div>
									<span class="absolute right-3 top-3 inline-flex h-4.5 w-4.5 shrink-0 items-center justify-center rounded-full border-2 {gayaArtikel === 'proper' ? 'border-blue-600' : 'border-slate-300'} bg-white">
										{#if gayaArtikel === 'proper'}<span class="h-2 w-2 rounded-full bg-blue-600"></span>{/if}
									</span>
									<p class="relative mt-2 max-w-[58%] text-xs leading-relaxed text-muted-foreground">
										{tr('Rilis resmi brand, peluncuran produk, atau artikel yang butuh fakta presisi.')}
									</p>
									<span class="relative mt-3 inline-flex items-center gap-1.5 rounded-full border bg-white px-2.5 py-1 text-[10.5px] font-semibold text-blue-700 {gayaArtikel === 'proper' ? '' : 'opacity-70'}" style="border-color: color-mix(in srgb, var(--primary) 25%, transparent);">
										<Plug size={12} class="shrink-0" /> {tr('Terhubung Copilot & Paste Mentahan')}
									</span>
								</button>
							</div>
						</div>
				{#snippet genStagePanel()}
				{#if genBusy || genOutcome !== 'idle'}
					<div class="genPanel mt-3" role="status" aria-live="polite">
						<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
							<div class="min-w-0 flex-1">
								<div class="flex items-start gap-2">
									{#if genOutcome === 'error'}
										<CircleAlert size={17} class="mt-0.5 shrink-0" style="color: var(--destructive);" />
									{:else if genOutcome === 'success'}
										<CircleCheck size={17} class="mt-0.5 shrink-0" style="color: var(--success);" />
									{:else}
										<LoaderCircle size={17} class="mt-0.5 shrink-0 animate-spin" style="color: var(--primary);" />
									{/if}
									<div class="min-w-0">
										<p class="text-sm font-bold">
											{genOutcome === 'error'
												? tr('Gagal membuat draf artikel')
												: genOutcome === 'success'
													? tr('Draf artikel berhasil dibuat')
													: tr('AI sedang menyusun draf artikel...')}
										</p>
										<p class="mt-0.5 text-xs text-muted-foreground">
											{genOutcome === 'error'
												? genError
												: genOutcome === 'success'
													? tr('Draf siap ditinjau di tab Editor Post.')
													: tr('Proses ini berjalan di server AI. Kamu bisa menunggu di halaman ini — draf akan muncul otomatis setelah selesai.')}
										</p>
									</div>
								</div>

								<!-- Bar progres: lebar mengikuti tahap nyata yang sedang berjalan,
								     tanpa angka persentase karena backend tidak mengirimkannya. -->
								<div class="genTrack mt-3" aria-hidden="true">
									<div class="genFill" class:fillError={genOutcome === 'error'} style="width: {genStagePct}%"></div>
								</div>

								<ul class="mt-4 space-y-0">
									{#each GEN_STAGES as s, i}
										{@const done = genOutcome === 'success' || i < genStage - 1}
										{@const active = genOutcome === 'idle' && i === genStage - 1}
										{@const failed = genOutcome === 'error' && i === genStage - 1}
										<li class="flex gap-2.5">
											<div class="flex flex-col items-center">
												{#if done}
													<span class="genDot genDotDone"><CircleCheck size={13} /></span>
												{:else if failed}
													<span class="genDot genDotError"><CircleAlert size={13} /></span>
												{:else if active}
													<span class="genDot genDotActive"></span>
												{:else}
													<span class="genDot genDotPending"></span>
												{/if}
												{#if i < GEN_STAGES.length - 1}
													<!-- Garis terisi hijau saat tahap selesai; saat tahap
													     sedang berjalan, aliran gradasi biru mengalir turun
													     menandai proses yang benar-benar aktif. -->
													<span class="genLine" class:genLineDone={done} class:genLineActive={active}></span>
												{/if}
											</div>
											<div class="pb-3">
												<p class="text-xs font-semibold" class:text-muted-foreground={!done && !active && !failed}>
													{tr(s.label)}
												</p>
												<p class="mt-0.5 text-[11px] leading-snug text-muted-foreground">{tr(s.desc)}</p>
											</div>
										</li>
									{/each}
								</ul>

								{#if genOutcome === 'error' && !genBusy}
									<button class="app-btn app-btn-soft mt-1 !min-h-8 !px-3 !text-xs" type="button" onclick={generate}>
										{tr('Coba lagi')}
									</button>
								{/if}
							</div>
						</div>
					</div>
				{/if}
				{/snippet}
						{#if gayaArtikel === 'simpel'}
						<div class="mt-6 w-full text-left">
								<div class="mb-6 min-w-0">
										<div class="flex items-start gap-3">
											{@render numBadge('2')}
											<div class="min-w-0">
												<p class="text-sm font-semibold">{tr('Angle Artikel')} <span style="color: var(--destructive);">*</span></p>
												{#if angleChannelName}
													<p class="mt-0.5 break-words text-xs text-muted-foreground">{tr(angleChannelName)}</p>
												{/if}
											</div>
										</div>
										<div class="angle-field mt-2">
										<Select
											ariaLabel={tr('Angle Artikel')}
											bind:value={angleSlug}
											options={angleSelectOptions}
											placeholder={tr('Pilih angle artikel')}
											searchable
											searchPlaceholder={tr('Cari angle…')}
											popularCount={5}
											card
											allOptionsLabel={tr('Semua angle')}
											popularLabel={tr('Angle paling populer')}
											showAllLabel={tr('Lihat semua angle')}
											showLessLabel={tr('Tampilkan lebih sedikit')}
											onchange={(v) => void recordAnglePick(v)}
											disabled={anglesLoading || !!anglesError || angleOptions.length === 0 || genBusy}
										/>
									</div>
									<div id="naskah-angle-help" class="mt-2 break-words text-xs" aria-live="polite">
										{#if anglesLoading}
											<p class="text-muted-foreground">{tr('Memuat pilihan angle…')}</p>
										{:else if anglesError}
											<p role="alert" style="color: var(--destructive);">{tr(anglesError)}</p>
											<button type="button" class="app-btn app-btn-soft mt-2 min-h-11" onclick={onRetryAngles}>{tr('Coba lagi')}</button>
										{:else if angleOptions.length === 0}
											<p class="text-muted-foreground">{tr('Belum ada angle dengan prompt aktif untuk media ini. Hubungi admin Tauthub.')}</p>
										{:else}
											{#if selectedAngle?.description}
												<p class="text-muted-foreground">{tr(selectedAngle.description)}</p>
											{/if}
										{/if}
									</div>
								</div>
									<!-- Tautan & anchor: tampil setelah Angle, sebelum Topik. -->
								<div>
									{#if linkLimit > 0}
										{@render linkEditor()}
									{:else}
										<p class="text-xs text-muted-foreground">{tr('Media ini tidak mengizinkan tautan dalam artikel.')}</p>
									{/if}
								</div>
							<div class="mt-5 flex items-center gap-3">
									{@render numBadge('4')}
									<label class="text-sm font-semibold" for="naskah-topik">
										{tr('Topik & Konteks Pembahasan')} <span style="color: var(--destructive);">*</span>
									</label>
<span class="ml-auto flex items-center gap-1.5">
										<button
											class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]"
											type="button"
											disabled={racikBusy || linkRows.length === 0}
											title={linkRows.length === 0 ? tr('Isi minimal 1 tautan di bagian Tautan & Anchor dulu — instruksinya diracik dari anchor.') : tr('Susun instruksi topik dari anchor + prinsip SEO/EEAT')}
											onclick={() => void racikInstruksi()}
										>
											{#if racikBusy}<LoaderCircle size={13} class="animate-spin" />{:else}<Sparkles size={13} />{/if}
											{racikBusy ? tr('Meracik…') : tr('Racik Instruksi dari Anchor')}
										</button>
									</span>
								</div>
								{#if racikError}
									<p class="mt-1 text-xs" style="color: var(--destructive);">{racikError}</p>
								{/if}
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
								{#if imgRequestCount > 0 && !genNotice}
									<div class="mt-3 rounded-lg px-3 py-2 text-xs" style="background-color: var(--accent); color: var(--accent-foreground);">
										<p class="font-semibold" style="color: var(--primary);">
											{tr('Terdeteksi permintaan gambar')} (~{imgRequestCount} {tr('gambar isi')})
										</p>
										<p class="mt-0.5">
											{#if imgCost > 0}
												{tr('Perkiraan biaya')}: {imgRequestCount} × {imgCost} {tr('kredit')}{imgRateIdr > 0
													? ` ≈ Rp ${imgEstimateRupiah.toLocaleString('id-ID')}`
													: ''}.
											{:else}
												{tr('Biaya gambar mengikuti tarif yang berlaku saat generate.')}
											{/if}
											{tr('Gambar utama (cover) tetap kamu atur manual atau unggah sendiri.')}
										</p>
									</div>
								{/if}
								{#if genNotice}
									<p class="mt-2 text-xs" style="color: var(--success);">{genNotice}</p>
								{/if}
								{#if relNotices.length > 0}
									<div class="mt-2 rounded-lg px-3 py-2 text-xs" style="background-color: var(--warning-soft); color: var(--warning);" aria-live="polite">
										<p class="font-semibold">{tr('Permintaan rel di luar cakupan media mitra')}</p>
										<ul class="mt-0.5 list-disc space-y-0.5 pl-4">
											{#each relNotices as notice (notice)}
												<li>{notice}</li>
											{/each}
										</ul>
									</div>
								{/if}
							{#if genError}
								<p class="mt-2 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
									<CircleAlert size={15} /> {genError}
								</p>
							{/if}
							<button
								class="app-btn app-btn-primary mt-4 w-full"
								type="button"
								disabled={genBusy || !topic.trim() || (linkLimit > 0 && !linksValid) || !angleReady}
								onclick={generate}
							>
								{#if genBusy}<Sparkles size={15} class="genSparkle" /><LoaderCircle size={15} class="animate-spin" />{:else}<Sparkles size={15} />{/if}
								{genBusy
									? tr('Sedang membuat draf artikel...')
									: `${tr('Buat Draf dengan AI')} (${tr('Gaya')} ${mediaType === 'blogger' ? tr('Media Blogger') : tr('Media Mainstream')})`}
							</button>

							{@render genStagePanel()}

							<p class="mt-3 text-xs text-muted-foreground">
								{tr('Draf akan tampil di tab Editor Post untuk kamu tinjau sebelum disimpan.')}
							</p>
						</div>
						{:else}
					<!-- PANE "YANG PROPER": isian berbeda dari Sederhana — grounding ke Copilot & fakta. -->
							<div class="mt-6 w-full text-left">
									<div class="min-w-0">
										<div class="flex items-start gap-3">
											{@render numBadge('2')}
											<div class="min-w-0">
												<p class="text-sm font-semibold">{tr('Angle Artikel')} <span style="color: var(--destructive);">*</span></p>
												{#if angleChannelName}
													<p class="mt-0.5 break-words text-xs text-muted-foreground">{tr(angleChannelName)}</p>
												{/if}
											</div>
										</div>
										<div class="angle-field mt-2">
										<Select
											ariaLabel={tr('Angle Artikel')}
											bind:value={angleSlug}
											options={angleSelectOptions}
											placeholder={tr('Pilih angle artikel')}
											searchable
											searchPlaceholder={tr('Cari angle…')}
											popularCount={5}
											card
											allOptionsLabel={tr('Semua angle')}
											popularLabel={tr('Angle paling populer')}
											showAllLabel={tr('Lihat semua angle')}
											showLessLabel={tr('Tampilkan lebih sedikit')}
											onchange={(v) => void recordAnglePick(v)}
											disabled={anglesLoading || !!anglesError || angleOptions.length === 0 || genBusy}
										/>
									</div>
									<div id="naskah-angle-help" class="mt-2 break-words text-xs" aria-live="polite">
										{#if anglesLoading}
											<p class="text-muted-foreground">{tr('Memuat pilihan angle…')}</p>
										{:else if anglesError}
											<p role="alert" style="color: var(--destructive);">{tr(anglesError)}</p>
											<button type="button" class="app-btn app-btn-soft mt-2 min-h-11" onclick={onRetryAngles}>{tr('Coba lagi')}</button>
										{:else if angleOptions.length === 0}
											<p class="text-muted-foreground">{tr('Belum ada angle dengan prompt aktif untuk media ini. Hubungi admin Tauthub.')}</p>
										{:else}
											{#if selectedAngle?.description}
												<p class="text-muted-foreground">{tr(selectedAngle.description)}</p>
											{/if}
										{/if}
									</div>
								</div>

								<!-- Memori Brand (Copilot) + Fakta Terkunci -->
								<div class="mt-6">
									<div class="flex flex-wrap items-center justify-between gap-2">
										<span class="text-sm font-semibold">{tr('Memori Brand (Tauthub Copilot)')}</span>
										<span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[10.5px] font-semibold" style="background-color: var(--success-soft, #e8f7ee); color: var(--success);">
											<span class="h-1.5 w-1.5 rounded-full" style="background-color: var(--success);"></span>
											{tr('Anti-Capek Ketik')}
										</span>
									</div>
										<div class="mt-3 w-full">
											{#if copilotBriefsLoaded && copilotBriefs.length === 0}
												<!-- Peringatan: brand belum punya Brief Copilot (Profiler).
												     Tombol membuka halaman Profiler di tab baru — isi
												     popup tetap aman. -->
												<div class="flex flex-wrap items-center justify-between gap-3 rounded-lg px-3 py-2.5 text-xs" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning);">
													<div class="flex min-w-0 items-start gap-2">
														<CircleAlert size={14} class="mt-0.5 shrink-0" />
														<div class="min-w-0">
															<p class="font-semibold">{tr('Belum ada Brief Copilot untuk brand ini')}</p>
															<p class="mt-0.5 leading-relaxed">{tr('Buat dulu Taut Brief-nya — setelah ada, fakta bisa ditarik otomatis ke sini.')}</p>
														</div>
													</div>
													<a
														href={profilerHref}
														target="_blank"
														rel="noopener noreferrer"
														class="app-btn app-btn-soft !min-h-7 shrink-0 border px-2.5 text-[11px] font-semibold"
														style="border-color: var(--warning);"
													>
														{tr('Buka Taut Brief')}
													</a>
												</div>
											{:else}
												<Select
													ariaLabel={tr('Brief Copilot')}
													bind:value={copilotBriefId}
													options={copilotBriefOptions}
													placeholder={tr('Pilih Brief Copilot…')}
													disabled={copilotBusy || genBusy}
													onchange={(v) => void pickCopilotBrief(v)}
												/>
											{/if}
										</div>
										<div class="mt-3 rounded-xl border p-4" style="border-color: var(--border);">
											{#if copilotBriefId}
												<p class="flex items-center gap-2 text-xs font-semibold">
													<span class="inline-flex items-center gap-1.5">
														<span class="h-2 w-2 rounded-full" style="background-color: var(--success);"></span>
														<span>{tr('Brand:')} <strong>{copilotBusy ? '…' : copilotBrandName || copilotBriefName}</strong> ({tr('Profil & Produk Terkunci')})</span>
													</span>
												</p>
												<p class="mt-1 leading-relaxed">
													<span class="text-xs font-semibold">{tr('Data Copilot Aktif')}:</span>
													{#if copilotBusy}
														<span class="text-xs text-muted-foreground"> {tr('Memuat fakta dari Brief ini…')}</span>
													{:else if copilotSummary}
														<span class="text-xs text-muted-foreground"> {copilotSummary}</span>
													{:else}
														<span class="text-xs text-muted-foreground"> {tr('Brief ini belum menyimpan fakta.')}</span>
													{/if}
												</p>
											{:else}
												<p class="text-xs text-muted-foreground">{tr('Pilih Brief Copilot dulu — nama brand, fakta & klaim diambil dari sana, bukan teks tetap.')}</p>
											{/if}
											{#if copilotError}
												<p class="mt-1 text-xs" style="color: var(--destructive);">{copilotError}</p>
											{/if}

											<div class="mt-4 flex flex-wrap items-center justify-between gap-2">
												<span class="text-xs font-semibold">{tr('Fakta / Data Produk yang Wajib Masuk')} <span style="color: var(--destructive);">*</span></span>
												<span class="flex items-center gap-1.5">
													<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" disabled={copilotBusy || genBusy} title={tr('Serap fakta dari Brief Copilot yang dipilih')} onclick={() => void tarikFaktaCopilot()}>
														{#if copilotBusy}<LoaderCircle size={12} class="animate-spin" />{:else}<Sparkles size={12} />{/if}
														{copilotBusy ? tr('Menyerap…') : tr('Tarik dari Copilot')}
													</button>
													<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" title={tr('Logika contoh menyusul')}>
														<Lightbulb size={12} /> {tr('Contoh Paste Chat WA')}
													</button>
												</span>
											</div>
										<textarea
											class="app-input mt-2 min-h-[130px] w-full resize-y text-sm"
											placeholder={tr('Malas merangkai kalimat? Copy-paste saja teks mentahan dari chat WA, brosur, atau spesifikasi produk di sini. AI hanya akan menggunakan fakta dari kotak ini & Copilot…')}
											bind:value={faktaTerkunci}
										></textarea>
									</div>
								</div>

									<!-- Pengunci standar redaksi -->
									<div class="mt-6">
										<div class="flex flex-wrap items-center gap-2">
											<span class="text-sm font-semibold">{tr('Pengunci Standar Redaksi')}</span>
											<span class="text-[11px] text-muted-foreground">{tr('Opsional • Cegah Kutipan Fiktif')}</span>
										</div>
										<div class="mt-3 grid gap-4 sm:grid-cols-2">
										<label class="block">
											<span class="text-xs font-semibold text-muted-foreground">{tr('Kota Rilis (Dateline)')}</span>
											<input class="app-input mt-1 w-full text-sm uppercase" placeholder="JAKARTA" bind:value={kotaRilis} />
										</label>
										<label class="block">
											<span class="text-xs font-semibold text-muted-foreground">{tr('Nama & Jabatan Narasumber Kutipan')}</span>
											<input class="app-input mt-1 w-full text-sm" placeholder="Budi Santoso, CEO Tuathub Indonesia" bind:value={narasumber} />
										</label>
										<label class="block sm:col-span-2">
											<span class="text-xs font-semibold text-muted-foreground">{tr('Isi Kutipan (Apa Adanya)')}</span>
											<textarea
												rows="3"
												class="app-input mt-1 w-full resize-y text-sm"
												placeholder={tr('Pernyataan yang diketik AI pakai persis seperti tulisanmu — tanpa menambah kutipan fiktif.')}
												bind:value={kutipanNarasumber}
											></textarea>
										</label>
										</div>
									</div>

									<!-- 3. Tautan & Anchor (Backlink) -->
									<div class="mt-6">
										{#if linkLimit > 0}
											{@render linkEditor(' (Backlink)', tr('Disisipkan natural di paragraf yang relevan'))}
										{:else}
											<p class="text-xs text-muted-foreground">{tr('Media ini tidak mengizinkan tautan dalam artikel.')}</p>
										{/if}
									</div>

									<!-- 4. Instruksi tambahan -->
									<div class="mt-6">
										<div class="flex flex-wrap items-center justify-between gap-2">
											<div class="flex items-center gap-3">
												{@render numBadge('4')}
												<span class="text-sm font-semibold">{tr('Instruksi Tambahan / Fokus Cerita')}</span>
											</div>
											<span class="flex flex-wrap items-center gap-2">
												<span class="text-[11px] text-muted-foreground">{tr('Opsional jika Copilot/Fakta sudah terisi')}</span>
												<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" title={tr('Logika contoh menyusul')}>
													<Lightbulb size={12} /> {tr('Contoh')}
												</button>
											</span>
										</div>
										<textarea
											class="app-input mt-3 min-h-[120px] w-full resize-y text-sm"
											placeholder={tr('Bahas tentang tren dan keunggulan penggunaan epoxy lantai pada bangunan modern (mulai dari ketahanan beban, estetika seamless, hingga kemudahan perawatan), lalu sisipkan tautan jasa epoxy lantai secara natural.')}
											bind:value={fokusSisa}
										></textarea>
									</div>

									{#if genError}
										<p class="mt-2 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
											<CircleAlert size={15} /> {genError}
										</p>
									{/if}
									<button
										class="app-btn app-btn-primary mt-6 w-full"
										type="button"
										disabled={genBusy || properLocked || (linkLimit > 0 && !linksValid) || !angleReady}
										title={properLocked
											? tr('Isi fakta produk atau pilih Brief Copilot dulu — mode Proper ditulis terkunci ke fakta.')
											: ''}
										onclick={generate}
									>
										{#if genBusy}<Sparkles size={15} class="genSparkle" /><LoaderCircle size={15} class="animate-spin" />{:else}<Sparkles size={15} />{/if}
										{genBusy
											? tr('Sedang membuat draf artikel...')
											: `${tr('Buat Draf dengan AI')} (${tr('Gaya')} ${mediaType === 'blogger' ? tr('Media Blogger') : tr('Media Mainstream')})`}
									</button>

									{@render genStagePanel()}
								</div>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Panel pengaturan artikel (kanan) — hanya di tab Editor Post. -->
			{#if tab === 'tulis'}
			<aside class="min-w-0 space-y-5 border-t pt-4 lg:border-l lg:border-t-0 lg:pl-5 lg:pt-5" style="border-color: var(--border);">
				<div>
					<div class="flex items-center justify-between gap-2">
						<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
							<ImageIcon size={13} /> {tr('Gambar Utama')}
						</p>
						<span class="rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--destructive); color: #fff;">{tr('WAJIB')}</span>
					</div>
						{#if cover}
							<div class="relative mt-3">
								<img src={sameOriginAssetUrl(cover) ?? ''} alt={tr('Gambar utama artikel')} class="aspect-video w-full rounded-lg border object-cover" style="border-color: var(--border);" />
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
							onchange={() => (autoCategory = '')}
						/>
					</div>
					{#if autoCategory}
						<p class="mt-1 text-[11px] font-medium" style="color: var(--primary);">
							{tr('Dipilih otomatis saat draf AI jadi — ubah bila perlu.')}
						</p>
					{/if}
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
			{/if}
		</div>

		<!-- Footer tetap -->
		<div class="flex flex-wrap items-center gap-3 border-t px-5 py-3.5" style="border-color: var(--border);">
			<button
				class={`app-btn app-btn-soft${composeBusy ? ' cursor-not-allowed opacity-50' : ''}`}
				type="button"
				disabled={saving}
				aria-disabled={composeBusy || saving}
				onclick={close}
			>{tr('Batal')}</button>
			<span class="flex-1"></span>
			{#if closeNotice}
				<p class="flex min-w-0 items-center gap-1.5 text-xs" style="color: var(--warning, #b45309);">
					<LoaderCircle size={14} class="flex-none animate-spin" />
					<span class="truncate">{closeNotice}</span>
				</p>
			{:else if pesanKurang}
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
									<span class="flex items-center gap-1.5">
										<label class="text-muted-foreground" for="h2-image-count">{tr('Jumlah gambar')}</label>
										<input
											id="h2-image-count"
											class="app-input !min-h-7 !w-16 !py-0.5 text-center text-xs"
											type="number"
											min="1"
											max={h2ConfirmData.items.length}
											bind:value={h2Count}
										/>
									</span>
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
								<span class="font-medium text-foreground">{h2CostRp}</span>
							</div>

							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Total Biaya')} ({h2Chosen} × {h2CostRp})</span>
								<strong class="font-bold text-foreground">{h2TotalRp}</strong>
							</div>

							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Saldo Saat Ini')}</span>
								<span class="font-medium text-foreground">{h2BalanceRp}</span>
							</div>

							<div class="h-px w-full" style="background-color: var(--border);"></div>

							<div class="flex items-center justify-between text-xs">
								<span class="font-semibold text-foreground">{tr('Estimasi Sisa Saldo')}</span>
								<strong class="font-bold" style="color: var(--primary);">
									{h2RemainingRp}
								</strong>
							</div>
						</div>

						{#if h2ConfirmData.balance != null && h2ConfirmData.balance < h2TotalCredits}
							<div class="flex items-center gap-2 rounded-xl p-3 text-xs" style="background-color: var(--destructive-soft, #fef2f2); color: var(--destructive, #dc2626);">
								<CircleAlert size={15} class="shrink-0" />
								<span>{tr('Saldo tidak mencukupi untuk membuat gambar. Silakan top up terlebih dahulu.')}</span>
							</div>
						{/if}
						<div
							class="flex items-start gap-2 rounded-lg px-3 py-2 text-xs leading-relaxed"
							style="background-color: var(--accent); color: var(--accent-foreground);"
						>
							<Info size={14} class="mt-px flex-none" />
							<p>
								{tr('Anda sebaiknya pakai gambar asli, bukan hasil generate AI. Mitra media lebih sering menyetujui artikel yang gambarnya asli, misalnya foto produk atau materi brand Anda sendiri.')}
							</p>
						</div>
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
							disabled={h2Busy || (h2ConfirmData.balance != null && h2ConfirmData.balance < h2TotalCredits)}
							onclick={executeGeneratePerH2}
						>
							{#if h2Busy}
								<LoaderCircle size={14} class="animate-spin" />
								<span>{tr('Memproses…')}</span>
							{:else}
								<Sparkles size={14} />
								<span>{tr('Ya, Generate')} {h2Chosen} {tr('gambar')} · {h2TotalRp}</span>
							{/if}
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>

<style>
	/* Dropdown "Angle Artikel" memakai komponen Select bawaan app. Di dalam
	   modal ini trigger-nya dilebarkan mengikuti lebar field supaya sejajar
	   dengan textarea Topik & Konteks; warna, hover, dan menu mengambang
	   tetap dari komponen. */
	.angle-field :global(.zs-select-trigger) {
		width: 100%;
		max-width: 100%;
		height: 44px;
		font-size: 14px;
	}
	.angle-field :global(.zs-select-label) {
		flex: 1;
		max-width: none;
		text-align: left;
	}

	/* Panel progres generate AI: kartu putih-biru muda dengan radius 14px, muncul
	   fade + slide-up singkat; timeline-nya mengikuti status request yang nyata. */
	.genPanel {
		border: 1px solid color-mix(in srgb, var(--primary) 22%, var(--border));
		border-radius: 14px;
		background: color-mix(in srgb, var(--primary) 4%, var(--card, #fff));
		padding: 18px;
		box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
		animation: gen-panel-in 0.3s ease-out;
	}
	.genTrack {
		height: 6px;
		width: 100%;
		border-radius: 999px;
		background: color-mix(in srgb, var(--primary) 12%, var(--muted));
		overflow: hidden;
	}
	.genFill {
		height: 100%;
		border-radius: 999px;
		background: linear-gradient(90deg, var(--primary), color-mix(in srgb, var(--primary) 60%, #67e8f9));
		transition: width 300ms ease;
	}
	.genFill.fillError { background: var(--destructive); }
	.genDot {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 999px;
		flex-shrink: 0;
	}
	.genDotDone {
		color: var(--success);
		background: color-mix(in srgb, var(--success) 14%, transparent);
		animation: gen-dot-pop 0.25s ease-out;
	}
	.genDotError {
		color: var(--destructive);
		background: color-mix(in srgb, var(--destructive) 14%, transparent);
	}
	.genDotActive {
		position: relative;
		background: color-mix(in srgb, var(--primary) 22%, transparent);
		animation: gen-dot-pulse 1.8s ease-in-out infinite;
	}
	.genDotActive::after {
		content: '';
		width: 8px;
		height: 8px;
		border-radius: 999px;
		background: var(--primary);
	}
	.genDotPending { background: color-mix(in srgb, var(--muted-foreground) 18%, transparent); }
	.genLine {
		width: 2px;
		flex: 1;
		min-height: 18px;
		margin: 2px 0;
		background: color-mix(in srgb, var(--muted-foreground) 18%, transparent);
		transition: background-color 300ms ease;
	}
	.genLine.genLineDone { background: color-mix(in srgb, var(--success) 55%, transparent); }
	/* Tahap berjalan: garis timeline mengalir turun (bukan pajangan statis). */
	.genLine.genLineActive {
		background: linear-gradient(180deg, var(--primary) 0%, color-mix(in srgb, var(--primary) 20%, transparent) 70%, transparent 100%);
		background-size: 100% 26px;
		background-repeat: no-repeat;
		animation: gen-line-flow 1.6s linear infinite;
	}
	.genSparkle { animation: gen-sparkle 2.4s ease-in-out infinite; }
	@keyframes gen-panel-in {
		from { opacity: 0; transform: translateY(8px); }
		to { opacity: 1; transform: translateY(0); }
	}
	@keyframes gen-dot-pop {
		from { transform: scale(0.7); }
		to { transform: scale(1); }
	}
	@keyframes gen-dot-pulse {
		0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--primary) 32%, transparent); }
		50% { box-shadow: 0 0 0 5px color-mix(in srgb, var(--primary) 0%, transparent); }
	}
	@keyframes gen-line-flow {
		0% { background-position: 0 -26px; }
		100% { background-position: 0 0; }
	}
	@keyframes gen-sparkle {
		0%, 100% { opacity: 1; transform: scale(1); }
		50% { opacity: 0.78; transform: scale(0.94); }
	}
	@media (min-width: 640px) {
		.genPanel { padding: 22px; }
	}
	@media (prefers-reduced-motion: reduce) {
		.genPanel, .genSparkle, .genDotDone, .genDotActive, .genLine.genLineActive { animation: none; }
		.genFill, .genLine { transition: none; }
	}
</style>
