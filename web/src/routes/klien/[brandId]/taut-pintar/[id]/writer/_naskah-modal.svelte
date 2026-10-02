<script lang="ts">
	// Modal "Pengelola Naskah" — satu tempat untuk menulis, mengunggah dokumen,
	// atau membuat draf AI untuk satu media target, dengan panel pengaturan
	// artikel (cover, kategori, meta deskripsi) dan footer validasi.
	// Reuse: rich-text-editor (toolbar + contentEditable), endpoint /files dan
	// /produce/{mediaId} lewat callback dari writer, serta saveEditor di writer.
	import { X, Upload, FileText, Sparkles, LoaderCircle, Image as ImageIcon, Trash2, CircleAlert, Info } from '@lucide/svelte';
	import { untrack } from 'svelte';
	import { tr } from '$lib/i18n/phrases';
	import { ARTICLE_DEFAULT_COVER } from '$lib/media/cover';
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';
	import Select from '$lib/components/select.svelte';

	let {
		mediaName,
		brandId,
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
		onGenerate
	}: {
		mediaName: string;
		brandId: string;
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
		onUploadDoc: (file: File) => Promise<{ title: string; html: string }>;
		onGenerate: (topic: string) => Promise<{ title: string; html: string }>;
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
	let dragging = $state(false);
	let coverInput = $state<HTMLInputElement | undefined>(undefined);
	let docInput = $state<HTMLInputElement | undefined>(undefined);

	let docBusy = $state(false);
	let docError = $state('');
	let genBusy = $state(false);
	let genError = $state('');
	let topic = $state('');

	let panelEl = $state<HTMLElement | null>(null);

	function close() {
		if (!saving) onClose();
	}

	$effect(() => {
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') close();
		};
		const onPaste = (e: ClipboardEvent) => {
			const file = Array.from(e.clipboardData?.files ?? []).find((f) => f.type.startsWith('image/'));
			if (file) {
				e.preventDefault();
				void pickCover(file);
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
	async function pickCover(file: File) {
		if (!file || coverBusy || !file.type.startsWith('image/')) return;
		coverBusy = true;
		coverError = '';
		try {
			cover = await onUploadCover(file);
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
		try {
			const hasil = await onUploadDoc(file);
			title = hasil.title;
			html = hasil.html;
			tab = 'tulis';
		} catch (reason) {
			docError = reason instanceof Error ? reason.message : tr('Gagal mengunggah dokumen.');
		} finally {
			docBusy = false;
		}
	}

	// ---------- generate AI ----------
	async function generate() {
		if (genBusy || !topic.trim()) return;
		genBusy = true;
		genError = '';
		try {
			const hasil = await onGenerate(topic.trim());
			title = hasil.title || title;
			html = hasil.html || html;
			tab = 'tulis';
		} catch (reason) {
			genError = reason instanceof Error ? reason.message : tr('Gagal membuat draf AI.');
		} finally {
			genBusy = false;
		}
	}

	// ---------- validasi & simpan ----------
	// Alasan cover tampil di panel gambar; sisanya di footer.
	const alasanCover = $derived(!cover ? tr('Gambar utama (thumbnail) wajib diunggah.') : '');
	const alasanLain = $derived.by(() => {
		const list: string[] = [];
		if (!title.trim()) list.push(tr('Judul artikel wajib diisi.'));
		if (editorStats.words === 0) list.push(tr('Isi artikel wajib ditulis atau diunggah.'));
		if (linkLimit > 0 && editorStats.links > linkLimit) {
			list.push(`${tr('Tautan melebihi batas media.')} (${editorStats.links}/${linkLimit})`);
		}
		return list;
	});
	const canSave = $derived(!!cover && alasanLain.length === 0 && !saving);

	function save() {
		if (!canSave) return;
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
					{ key: 'tulis', label: tr('Tulis / Edit') },
					{ key: 'unggah', label: tr('Unggah Dokumen') },
					{ key: 'generate', label: tr('Generate dengan AI') }
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
					<div class="mt-4">
						<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{tr('Isi Artikel (Konten)')}</p>
						<div class="mt-2">
							<RichTextEditor bind:value={html} bind:stats={editorStats} brandId={brandId} ariaLabel={tr('Isi artikel')} minHeight={360} />
						</div>
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
								<p class="mt-2 text-sm font-semibold">{tr('Memproses dokumen…')}</p>
							{:else}
								<div class="flex h-12 w-12 items-center justify-center rounded-full" style="background-color: var(--primary); color: var(--primary-foreground);">
									<Upload size={20} />
								</div>
								<p class="mt-2 text-sm font-bold">{tr('Klik atau Tarik File ke sini')}</p>
								<p class="text-xs text-muted-foreground">{tr('Mendukung DOCX, TXT, PDF (Maks 10MB)')}</p>
							{/if}
						</div>
						{#if docError}
							<p class="mt-3 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
								<CircleAlert size={15} /> {docError}
							</p>
						{/if}
					</div>
				{:else}
					<div class="flex h-full min-h-[320px] flex-col items-center py-4 text-center">
						<div class="flex h-12 w-12 items-center justify-center rounded-xl" style="background-color: var(--accent-soft, #f3e8ff); color: var(--accent);">
							<Sparkles size={22} />
						</div>
						<h3 class="mt-3 text-xl font-bold">{tr('Delegasikan pada The Writer')}</h3>
						<p class="mt-2 max-w-md text-sm text-muted-foreground">
							{tr('Cukup berikan konteks singkat, AI kami akan merangkai artikel yang utuh, relevan, dan teroptimasi SEO.')}
						</p>
						<div class="mt-5 w-full rounded-lg p-3 text-left text-xs" style="background-color: var(--accent-soft, #eef2ff);">
							<p class="flex items-center gap-1.5 font-semibold" style="color: var(--primary);">
								<Info size={14} /> {tr('Penyesuaian Gaya Bahasa Otomatis')}
							</p>
							<p class="mt-1 leading-relaxed" style="color: var(--primary);">
								{tr('AI Tauthub otomatis menyesuaikan gaya tulisan mengikuti kaidah media target — berita jurnalistik untuk media mainstream, format santai dengan heading untuk blog.')}
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
								<Sparkles size={16} class="absolute bottom-3 right-3" style="color: var(--accent);" />
							</div>
							{#if genError}
								<p class="mt-2 flex items-center gap-1.5 text-sm" style="color: var(--destructive);">
									<CircleAlert size={15} /> {genError}
								</p>
							{/if}
							<button
								class="app-btn app-btn-primary mt-3 w-full"
								type="button"
								disabled={genBusy || !topic.trim()}
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
							<img src={cover} alt={tr('Gambar utama artikel')} class="h-36 w-full rounded-lg border object-cover" style="border-color: var(--border);" />
							<button
								class="absolute right-2 top-2 rounded-lg border bg-card p-1.5 hover:bg-secondary"
								type="button"
								aria-label={tr('Hapus gambar utama')}
								onclick={() => (cover = null)}
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
							<p class="mt-1 text-xs font-semibold">{tr('Unggah Gambar (JPG/PNG)')}</p>
							<p class="text-[11px] text-muted-foreground">{tr('Media redaksi membutuhkan min. 1 gambar resolusi tinggi.')}</p>
						</div>
					{/if}
					{#if coverError}
						<p class="mt-2 text-xs" style="color: var(--destructive);">{coverError}</p>
					{/if}
					{#if !cover}
						<p class="mt-2 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-xs" style="color: var(--warning, #b45309);">
							<CircleAlert size={13} class="flex-none" />
							{alasanCover}
							<button class="underline hover:no-underline" type="button" onclick={useFallbackCover}>
								{tr('Pakai thumbnail default Tauthub')}
							</button>
						</p>
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
					<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
						<FileText size={13} /> {tr('Kategori / Rubrik')}
					</p>
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
					<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr('CUPLIKAN SINGKAT (META DESC)')}</p>
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
			{#if alasanLain.length > 0 || !cover}
				<p class="flex min-w-0 items-center gap-1.5 text-xs" style="color: var(--warning, #b45309);">
					<CircleAlert size={14} class="flex-none" />
					<span class="truncate">{tr('Lengkapi data & thumbnail untuk menyimpan.')}</span>
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
</div>
