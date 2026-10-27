<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Modal editor rich untuk Writer — dipakai untuk Preview hasil generate
	// maupun edit hasil Unggah Artikel. Port writer/editor-modal.tsx.
	// contentEditable + execCommand (tanpa dependensi baru).
	import { Image, Info, Link, LoaderCircle, RotateCcw, Sparkles, TriangleAlert, Upload, X } from '@lucide/svelte';
	import type { Snippet } from 'svelte';
	import Select from '$lib/components/select.svelte';
	import MediaLibraryPicker from '$lib/components/media-library-picker.svelte';
	// Toolbar + isi artikel memakai komponen yang sama dengan Pengelola Naskah
	// (tabel, gambar, tautan, peta/video, mode HTML) supaya tidak ada dua editor
	// dengan kemampuan berbeda di portal klien.
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';
	import { sameOriginAssetUrl, TAUTHUB_FALLBACK_COVER } from '$lib/media/cover';

	// Fallback cover "pakai logo Tauthub": konstantanya di $lib/media/cover agar
	// sama dengan yang dipakai konfirmasi "lanjut tanpa cover" di The Writer.

	let {
		mediaName,
		initialTitle,
		initialHtml,
		initialCoverUrl,
		initialCategory,
		initialMetaDescription,
		categoryOptions = [],
		linkLimit,
		allowEmbed = false,
		saving = false,
		saveError = null,
		onClose,
		onSave,
		onUploadCover,
		regenDisabled,
		regenLabel = tr('Generate Ulang'),
		regenTitle,
		onRegenerate,
		// Kebijakan yang bisa dilonggarkan konteks lain (mis. modal revisi di
		// halaman Ringkasan). Default-nya tetap perilaku The Writer.
		requireCover = true,
		enforceLinkLimit = true,
		subtitle = tr('Edit artikel lalu simpan.'),
		saveLabel = tr('Simpan'),
		showSaveButton = true,
		readOnly = false,
		// Konteks untuk opsi "Generate Gambar AI" pada tombol Ganti Cover.
		// Null = perilaku lama: tombol langsung membuka pemilih berkas.
		coverAi = null,
		// Snippet di kolom kanan, MENGGANTIKAN blok "Kemampuan editor artikel".
		// Dipakai editor yang dibuka dari obrolan: yang lebih relevan di sana
		// adalah riwayat permintaan mitra, bukan daftar kemampuan editor.
		asideExtra,
		// Scope brand/campaign/media untuk tombol gambar di editor. Kosong =
		// editor memakai unggahan berkas biasa (mis. editor dokumen mandiri).
		brandId = '',
		campaignId = '',
		mediaId = ''
	}: {
		mediaName: string;
		initialTitle: string;
		initialHtml: string;
		initialCoverUrl?: string | null;
		initialCategory?: string | null;
		initialMetaDescription?: string | null;
		categoryOptions?: string[];
		linkLimit: number;
		allowEmbed?: boolean;
		saving?: boolean;
		saveError?: string | null;
		onClose: () => void;
		onSave: (title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) => void;
		onUploadCover: (file: File) => Promise<string>;
		regenDisabled?: boolean;
		regenLabel?: string;
		regenTitle?: string;
		onRegenerate?: () => void;
		requireCover?: boolean;
		enforceLinkLimit?: boolean;
		subtitle?: string;
		saveLabel?: string;
		showSaveButton?: boolean;
		readOnly?: boolean;
		coverAi?: { brandId: string; campaignId: string; mediaId: string; articleId?: string | null } | null;
		asideExtra?: Snippet;
		brandId?: string;
		campaignId?: string;
		mediaId?: string;
	} = $props();

	function extractH1(html: string): { h1: string; body: string } {
		const m = /<h1[^>]*>([\s\S]*?)<\/h1>/i.exec(html);
		if (!m) return { h1: '', body: html };
		return { h1: m[1].replace(/<[^>]+>/g, '').trim(), body: html.replace(m[0], '') };
	}

	let coverInput = $state<HTMLInputElement | null>(null);

	// Isi artikel dikelola RichTextEditor lewat nilai dua arah, bukan DOM lokal.
	let html = $state('');
	let editorStats = $state({ words: 0, links: 0, images: 0 });

	let title = $state('');
	let words = $state(0);
	let coverUrl = $state<string | null>(null);
	let category = $state('');
	let metaDescription = $state('');
	// Nilai awal disalin dari prop lewat $effect (menghindari peringatan
	// state_referenced_locally): setelah itu form dikendalikan state lokal.
	// H1 pertama di isi artikel diangkat jadi judul, sisanya jadi isi editor.
	let seeded = false;
	$effect(() => {
		if (seeded) return;
		seeded = true;
		const parsed = extractH1(initialHtml);
		html = parsed.body || initialHtml;
		title = initialTitle || parsed.h1;
		coverUrl = initialCoverUrl ?? null;
		category = initialCategory ?? categoryOptions?.[0] ?? '';
		metaDescription = initialMetaDescription ?? '';
	});
	let coverBusy = $state(false);
	let uploadError = $state('');
	const linkCount = $derived(editorStats.links);
	let coverConfirmOpen = $state(false);
	let warnOpen = $state(false);
	let coverInfoOpen = $state(false);
	// Menu dua opsi pada tombol Ganti Cover (unggah sendiri / generate AI) dan
	// status buka modal pustaka gambar AI. Menu dirender sebagai popup mengambang
	// yang di-anchor ke tombolnya (posisi dihitung dari rect tombol).
	let coverMenuOpen = $state(false);
	let coverMenuPos = $state<{ left: number; top: number; width: number } | null>(null);
	let coverAiOpen = $state(false);
	function onCoverPickClick(event: MouseEvent) {
		if (coverBusy || readOnly) return;
		if (coverAi) {
			if (coverMenuOpen) {
				coverMenuOpen = false;
				return;
			}
			const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
			const width = Math.min(340, window.innerWidth - 24);
			const left = Math.min(Math.max(12, rect.right - width), window.innerWidth - width - 12);
			coverMenuPos = { left, top: Math.min(rect.bottom + 6, window.innerHeight - 132), width };
			coverMenuOpen = true;
			return;
		}
		coverInput?.click();
	}
	function pickOwnCover() {
		coverMenuOpen = false;
		coverInput?.click();
	}
	function openCoverAi() {
		coverMenuOpen = false;
		coverAiOpen = true;
	}
	// Ukuran gambar (byte) — cover + gambar di dalam artikel, untuk footer editor.
	let coverBytes = $state(0);
	let articleBytes = $state(0);

	const linkOverLimit = $derived(linkCount > linkLimit);
	const totalImageKb = $derived(coverBytes + articleBytes);
	// Satuan paling ringkas + koma desimal ala Indonesia: 3,4MB, bukan 3422KB.
	const fmtKb = (b: number) => {
		if (b <= 0) return '0KB';
		if (b >= 1024 * 1024) return `${(b / 1024 / 1024).toFixed(1).replace('.', ',')}MB`;
		return `${Math.max(1, Math.round(b / 1024))}KB`;
	};

	async function measureBytes(src: string): Promise<number> {
		try {
			const res = await fetch(src, { method: 'GET', credentials: 'include' });
			if (!res.ok) return 0;
			return (await res.arrayBuffer()).byteLength;
		} catch {
			return 0;
		}
	}

	async function measureArticleImages() {
		const imgs = Array.from(html.matchAll(/<img[^>]+src="([^"]+)"/gi)).map((m) => m[1]).filter(Boolean);
		articleBytes = (await Promise.all(imgs.map(measureBytes))).reduce((a, b) => a + b, 0);
	}

	// Ukur gambar di dalam artikel setiap isi berubah.
	$effect(() => {
		html;
		void measureArticleImages();
	});

	// Ukur cover saat berubah. URL di-relatifkan dulu: cover tersimpan bisa
	// absolut ke origin publik (produksi) sementara file ada di server lokal.
	$effect(() => {
		let alive = true;
		const url = sameOriginAssetUrl(coverUrl);
		if (!url) {
			coverBytes = 0;
			return;
		}
		void measureBytes(url).then((b) => {
			if (alive) coverBytes = b;
		});
		return () => {
			alive = false;
		};
	});

	// Tempel (Ctrl+V) gambar dari clipboard → jadi cover, bila cover masih kosong.
	// Tempelan di dalam isi artikel diurus editor (menjadi gambar artikel).
	$effect(() => {
		function onPaste(e: ClipboardEvent) {
			if (coverUrl || coverBusy) return;
			const target = e.target as Node | null;
			if (target instanceof Element && target.closest('.rte-body')) return;
			const img = Array.from(e.clipboardData?.items ?? []).find((i) => i.type.startsWith('image/'));
			const file = img?.getAsFile();
			if (!file) return;
			e.preventDefault();
			void pickCover(file);
		}
		document.addEventListener('paste', onPaste);
		return () => document.removeEventListener('paste', onPaste);
	});

	// Statistik editor (kata/tautan) dipakai footer & validasi batas tautan.
	$effect(() => {
		words = editorStats.words;
	});

	// Escape menutup modal.
	$effect(() => {
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape' && !saving && !coverInfoOpen) onClose();
		}
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	async function pickCover(file: File) {
		if (!file || coverBusy) return;
		if (!file.type.startsWith('image/')) return;
		coverBusy = true;
		uploadError = '';
		try {
			coverUrl = await onUploadCover(file);
		} catch {
			uploadError = tr('Gagal mengunggah cover. Periksa koneksi lalu coba lagi.');
		} finally {
			coverBusy = false;
			if (coverInput) coverInput.value = '';
		}
	}

	function handleSave() {
		if (enforceLinkLimit && linkOverLimit) {
			warnOpen = true;
			return;
		}
		if (requireCover && !coverUrl?.trim()) {
			coverConfirmOpen = true;
			return;
		}
		onSave(title.trim() || tr('Artikel'), html, coverUrl, category.trim(), metaDescription.trim());
	}

	function saveWithFallbackCover() {
		coverUrl = TAUTHUB_FALLBACK_COVER;
		coverConfirmOpen = false;
		onSave(title.trim() || tr('Artikel'), html, TAUTHUB_FALLBACK_COVER, category.trim(), metaDescription.trim());
	}
</script>

<!-- Menu dua opsi untuk Ganti Cover sebagai POPUP mengambang, anchor tepat di
     bawah tombol yang diklik (posisi dihitung dari rect tombol di JS). Di
     belakangnya ada backdrop transparan untuk menutup saat klik di luar. -->
{#snippet coverMenu()}
	{#if coverAi && coverMenuPos}
		<div class="fixed inset-0 z-[90]" role="presentation" onclick={() => (coverMenuOpen = false)}></div>
		<div
			class="fixed z-[91] overflow-hidden rounded-xl border shadow-xl"
			style="left: {coverMenuPos.left}px; top: {coverMenuPos.top}px; width: {coverMenuPos.width}px; border-color: var(--border); background-color: var(--card, #fff);"
			role="menu"
		>
			<button type="button" class="flex w-full items-start gap-2.5 px-3 py-2.5 text-left transition-colors hover:bg-secondary" role="menuitem" onclick={pickOwnCover}>
				<Upload size={15} class="mt-0.5 shrink-0 text-muted-foreground" />
				<span class="min-w-0">
					<span class="block text-xs font-semibold">{tr('Unggah gambar sendiri')}</span>
					<span class="block text-[11px] text-muted-foreground">{tr('Gratis · JPG/PNG, otomatis dikompres ke WebP')}</span>
				</span>
			</button>
			<button type="button" class="flex w-full items-start gap-2.5 border-t px-3 py-2.5 text-left transition-colors hover:bg-secondary" style="border-color: var(--border);" role="menuitem" onclick={openCoverAi}>
				<Sparkles size={15} class="mt-0.5 shrink-0" style="color: var(--primary);" />
				<span class="min-w-0">
					<span class="block text-xs font-semibold">{tr('Generate Gambar AI')}</span>
					<span class="block text-[11px] text-muted-foreground">{tr('Berbayar — biaya & saldo ditampilkan sebelum diproses; saldo terpotong otomatis bila saldo cukup')}</span>
				</span>
			</button>
		</div>
	{/if}
{/snippet}

<div
	class="fixed inset-0 z-50 flex items-stretch justify-center p-0 sm:items-center sm:p-4"
	style="background-color: rgba(15, 23, 42, 0.45);"
	role="dialog"
	aria-modal="true"
	aria-label={`${tr('Editor artikel')} ${mediaName}`}
	onscroll={() => (coverMenuOpen = false)}
>
	<div class="app-card flex h-full w-full flex-col overflow-hidden rounded-none p-5 sm:h-auto sm:max-h-[92dvh] sm:max-w-[min(95vw,1200px)] sm:rounded-2xl">
		<div class="flex items-start justify-between gap-3">
			<div>
				<p class="font-bold">{mediaName}</p>
				<p class="text-xs text-muted-foreground">{subtitle}</p>
			</div>
			<button
				class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary"
				type="button"
				aria-label={tr('Tutup')}
				disabled={saving}
				onclick={onClose}
			>
				<X size={18} />
			</button>
		</div>

			<div class="mt-3 flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto lg:flex-row lg:overflow-hidden">
				<!-- Kolom kiri: judul + toolbar + isi artikel -->
				<div class="flex min-w-0 flex-1 flex-col gap-3 lg:overflow-y-auto lg:pr-1">
					<input class="app-input w-full text-base font-semibold" aria-label="Judul artikel" maxlength="300" placeholder={tr("Judul artikel")} readonly={readOnly} bind:value={title} />

					<!-- Editor yang sama dengan Pengelola Naskah: toolbar lengkap
					     (tabel, gambar, tautan, peta/video, mode HTML) dan isi bergaya
					     .rte-body termasuk tabel. -->
					<RichTextEditor
						bind:value={html}
						bind:stats={editorStats}
						{brandId}
						{campaignId}
						{mediaId}
						uploadImage={brandId ? undefined : onUploadCover}
						embedAllowed={allowEmbed}
						disabled={saving || readOnly}
						minHeight={360}
						ariaLabel={tr('Isi artikel')}
					/>

					{#if saveError}<p class="text-sm" style="color: var(--destructive);">{saveError}</p>{/if}
					{#if linkOverLimit}
						<p class="text-sm" style="color: var(--destructive);"><TriangleAlert size={13} class="inline" /> {tr('Terlalu banyak link aktif. Maks')} {linkLimit} {tr("tautan per media.")}</p>
					{/if}
				</div>

				<!-- Kolom kanan: gambar utama, kategori, meta, info editor -->
				<aside class="flex w-full flex-none flex-col gap-4 lg:w-[340px] lg:overflow-y-auto lg:pr-1 xl:w-[380px]">
					<!-- Cover artikel -->
					<div>
						<p class="text-[13px] font-semibold">{tr('Gambar Utama')}</p>
						<p class="mt-0.5 text-xs text-muted-foreground">{tr('Gambar akan ditampilkan di Tauthub & dikirim ke media.')}</p>
						<div class="mt-2">
						{#if !readOnly}
							<input
								bind:this={coverInput}
								type="file"
								accept="image/*"
								class="hidden"
								onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void pickCover(f); }}
							/>
						{/if}
						{#if coverUrl}
							<div class="relative">
								<img src={sameOriginAssetUrl(coverUrl) ?? ''} alt={tr('Cover artikel')} class="aspect-video w-full rounded-lg object-cover" />
								{#if !readOnly}
									<button class="app-btn absolute right-2.5 bottom-2.5 !min-h-7 !px-3 !text-xs" type="button" style="background-color: rgba(15, 23, 42, 0.72); color: #fff;" disabled={coverBusy} onclick={onCoverPickClick}>
										{coverBusy ? tr('Mengunggah…') : tr('Ganti Cover')}
									</button>
									{#if coverMenuOpen}
										{@render coverMenu()}
									{/if}
								{/if}
							</div>
						{:else if readOnly}
							<div class="flex min-h-[72px] w-full flex-col items-center justify-center rounded-xl border border-dashed p-4 text-center text-xs text-muted-foreground">
								{tr('Cover artikel belum diunggah')}
							</div>
						{:else}
							<div class="relative">
								<button class="flex min-h-[132px] w-full flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed p-5 text-center" type="button" disabled={coverBusy} onclick={onCoverPickClick}>
									<Upload size={20} class="text-muted-foreground" />
									<span class="text-sm font-medium">{coverBusy ? tr('Mengunggah cover…') : tr('Cover artikel belum diunggah')}</span>
									<span class="text-xs text-muted-foreground">{tr("Klik untuk mengunggah, atau tempel gambar (Ctrl+V) · Maksimal 200KB · Lebih dari itu otomatis dikompres & diubah ke WebP oleh sistem TautHub")}</span>
								</button>
								{#if coverMenuOpen}
									{@render coverMenu()}
								{/if}
							</div>
						{/if}
						</div>
					</div>

					<div>
						<p class="text-[13px] font-semibold">{tr('Kategori artikel')}</p>
						<div class="mt-1.5">
						{#if categoryOptions.length > 0}
							<Select
								bind:value={category}
								ariaLabel={tr('Kategori artikel')}
								minWidth={200}
								disabled={readOnly}
								options={[
									...(category && !categoryOptions.includes(category) ? [{ value: category, label: category }] : []),
									...categoryOptions.map((c) => ({ value: c, label: c }))
								]}
							/>
						{:else}
							<span class="text-xs text-muted-foreground">{tr("Media ini belum punya kategori dari mitra — atur di dashboard mitra.")}</span>
						{/if}
						</div>
					</div>

					<label class="block">
						<span class="text-[13px] font-semibold">{tr('Meta deskripsi')}</span>
						<textarea class="app-input mt-1.5 w-full" rows="3" maxlength="500" readonly={readOnly} placeholder={tr("Ringkasan 120-160 karakter untuk hasil pencarian — terisi otomatis oleh AI")} bind:value={metaDescription}></textarea>
					</label>

					{#if uploadError}<p class="text-sm" style="color: var(--destructive);">{uploadError}</p>{/if}

					{#if asideExtra}
						{@render asideExtra()}
					{:else if !readOnly}
						<!-- Info kemampuan editor: caption + bullet, dibuka lewat trigger kecil. -->
						<div class="relative">
							<button class="inline-flex items-center gap-1.5 self-start text-xs text-muted-foreground hover:text-foreground" type="button" onclick={() => (coverInfoOpen = !coverInfoOpen)}>
								<Info size={13} />
								{tr('Kemampuan editor artikel')}
							</button>
							{#if coverInfoOpen}
								<div class="mt-2 flex flex-col gap-2 rounded-lg border p-3" style="background-color: var(--card);">
									<div class="flex items-center gap-1.5">
										<Info size={14} style="color: var(--primary);" />
										<p class="text-sm font-bold">{tr('Kemampuan editor artikel')}</p>
									</div>
									<ul class="list-disc space-y-1 pl-5 text-xs text-muted-foreground">
										<li>{tr('Cover adalah gambar utama artikel yang tampil sebagai thumbnail di media')}</li>
										<li>{tr('Bisa mengubah judul dan seluruh isi teks jika diperlukan')}</li>
										<li>{tr('Bisa tambahkan gambar, tabel, dan kutipan dalam artikel')}</li>
										<li>{tr('Bisa menambah atau mengubah hyperlink jika fitur tersedia')}</li>
										<li>{tr('Bisa menyisipkan video atau peta pada media yang mengizinkannya')}</li>
										<li>{tr('Jika cover tidak diisi, artikel memakai gambar fallback bawaan saat ditinjau & didistribusikan')}</li>
									</ul>
								</div>
							{/if}
						</div>
					{/if}
				</aside>
			</div>

			<div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t pt-3">
				<span class="inline-flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-xs text-muted-foreground">
					{words} {tr('kata')}
					<span style="opacity: .6;">·</span>
					<Image size={13} />
					<span>{fmtKb(totalImageKb)}</span>
					{#if enforceLinkLimit}
						<span style="opacity: .6;">·</span>
						<Link size={13} />
						<span style="color: {linkOverLimit ? 'var(--destructive)' : 'var(--muted-foreground)'}; font-weight: {linkOverLimit ? 700 : 400};">{tr('Link')} {linkCount}/{linkLimit}</span>
					{/if}
				</span>
				<div class="flex flex-wrap items-center gap-2">
					{#if onRegenerate && !readOnly}
						<button class="app-btn app-btn-soft" type="button" disabled={saving || regenDisabled} title={regenDisabled ? regenTitle : undefined} onclick={onRegenerate}>
							<RotateCcw size={14} /> {regenLabel}
						</button>
					{/if}
					<button class="app-btn app-btn-soft" type="button" disabled={saving} onclick={onClose}>{showSaveButton ? tr("Batal") : tr("Tutup")}</button>
					{#if showSaveButton}
						<button class="app-btn app-btn-primary" type="button" disabled={saving} onclick={handleSave}>
							{#if saving}<LoaderCircle size={14} class="animate-spin" />{/if}
							{saving ? tr('Menyimpan…') : saveLabel}
						</button>
					{/if}
				</div>
			</div>
	</div>
</div>

{#if warnOpen}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="flex items-center gap-2 font-bold" style="color: var(--warning);"><TriangleAlert size={18} /> {tr('Link Terlalu Banyak')}</p>
			<p class="mt-2 text-sm">
				{tr('Artikel ini berisi')} <strong>{linkCount}</strong> {tr("tautan aktif, tetapi media ini hanya memperbolehkan maksimal")} <strong>{linkLimit}</strong> {tr('tautan.')}
				{tr('Kurangi URL aktif')} ({linkCount - linkLimit} {tr('tautan berlebih')}) {tr('sebelum menyimpan.')}
			</p>
			<p class="mt-2 text-xs" style="color: var(--destructive);">{tr("Artikel tidak akan tersimpan sampai jumlah link sesuai batas media.")}</p>
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (warnOpen = false)}>{tr("Edisi Ulang")}</button>
			</div>
		</div>
	</div>
{/if}

{#if coverConfirmOpen}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="flex items-center gap-2 font-bold" style="color: var(--destructive);"><TriangleAlert size={18} /> {tr('Cover Artikel Masih Kosong')}</p>
			<p class="mt-2 text-sm" style="color: var(--destructive);">
				{tr('Artikel ini')} <strong>{tr('wajib memiliki gambar cover')}</strong>{tr(". Tanpa cover, artikel akan otomatis memakai thumbnail/logo bawaan Tauthub.")}</p>
			<p class="mt-2 text-sm">{tr("Lanjutkan tanpa cover (artikel memakai gambar fallback Tauthub), atau tambahkan gambar sekarang?")}</p>
			<div class="mt-4 flex justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => { coverConfirmOpen = false; coverInput?.click(); }}>{tr("Tidak, saya tambah gambar")}</button>
				<button class="app-btn app-btn-primary" type="button" onclick={saveWithFallbackCover}>{tr('Ya, pakai logo Tauthub')}</button>
			</div>
		</div>
	</div>
{/if}

<!-- Pustaka + generate gambar AI untuk cover (mode 'cover'): menampilkan biaya
     per gambar & saldo dalam rupiah, memblokir generate saat saldo kurang. -->
{#if coverAi && coverAiOpen}
	<MediaLibraryPicker
		open={coverAiOpen}
		brandId={coverAi.brandId}
		campaignId={coverAi.campaignId}
		mediaId={coverAi.mediaId}
		articleId={coverAi.articleId ?? null}
		mode="cover"
		defaultHeading={title}
		onSelect={(url) => {
			coverUrl = url;
			coverAiOpen = false;
			coverMenuOpen = false;
		}}
		onClose={() => (coverAiOpen = false)}
	/>
{/if}
