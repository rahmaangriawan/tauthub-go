<script lang="ts">
	// Modal editor rich untuk Writer — dipakai untuk Preview hasil generate
	// maupun edit hasil Unggah Artikel. Port writer/editor-modal.tsx.
	// contentEditable + execCommand (tanpa dependensi baru).
	import { Image, Link, List, ListOrdered, LoaderCircle, Quote, Redo2, RotateCcw, TriangleAlert, Upload, Video } from '@lucide/svelte';
	import type { Component } from 'svelte';

	// Fallback saat user memilih "pakai logo Tauthub" sebagai cover (asset publik).
	const TAUTHUB_FALLBACK_COVER = '/tauthub-logo.webp';

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
		regenLabel = 'Generate Ulang (2 Kredit)',
		regenTitle,
		onRegenerate
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
	} = $props();

	function extractH1(html: string): { h1: string; body: string } {
		const m = /<h1[^>]*>([\s\S]*?)<\/h1>/i.exec(html);
		if (!m) return { h1: '', body: html };
		return { h1: m[1].replace(/<[^>]+>/g, '').trim(), body: html.replace(m[0], '') };
	}

	let bodyEl = $state<HTMLDivElement | null>(null);
	let coverInput = $state<HTMLInputElement | null>(null);
	let articleImageInput = $state<HTMLInputElement | null>(null);

	let title = $state('');
	let words = $state(0);
	let coverUrl = $state<string | null>(null);
	let category = $state('');
	let metaDescription = $state('');
	// Nilai awal disalin dari prop lewat $effect (menghindari peringatan
	// state_referenced_locally): setelah itu form dikendalikan state lokal.
	$effect(() => {
		title = initialTitle ?? '';
		coverUrl = initialCoverUrl ?? null;
		category = initialCategory ?? categoryOptions?.[0] ?? '';
		metaDescription = initialMetaDescription ?? '';
	});
	let coverBusy = $state(false);
	let articleBusy = $state(false);
	let uploadError = $state('');
	let linkCount = $state(0);
	let linkActive = $state(false);
	let coverConfirmOpen = $state(false);
	let warnOpen = $state(false);
	let linkDialog = $state<{ open: boolean; url: string; isEdit: boolean }>({ open: false, url: '', isEdit: false });
	let embedDialog = $state<{ open: boolean; url: string; error: string }>({ open: false, url: '', error: '' });
	let savedRange: Range | null = null;

	const linkOverLimit = $derived(linkCount > linkLimit);

	// Isi body sekali saat mount: ekstrak H1 ke field judul, sisanya jadi body.
	$effect(() => {
		const body = bodyEl;
		if (!body) return;
		const { h1, body: rest } = extractH1(initialHtml);
		body.innerHTML = rest || initialHtml;
		const t = window.setTimeout(() => {
			if (h1) title = h1;
			sync();
		}, 0);
		return () => window.clearTimeout(t);
	});

	// Escape menutup modal.
	$effect(() => {
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape' && !saving) onClose();
		}
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	function sync() {
		const body = bodyEl;
		if (!body) return;
		const text = body.innerText || '';
		words = text.trim() ? text.trim().split(/\s+/).filter(Boolean).length : 0;
		linkCount = body.querySelectorAll('a').length;
	}

	function exec(cmd: string, arg?: string) {
		const body = bodyEl;
		if (!body) return;
		body.focus();
		document.execCommand(cmd, false, arg);
		sync();
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
		{ cmd: 'undo', icon: Redo2, name: 'Urungkan' },
		{ cmd: 'redo', icon: RotateCcw, name: 'Ulangi' }
	];

	async function pickCover(file: File) {
		if (!file || coverBusy) return;
		if (!file.type.startsWith('image/')) return;
		coverBusy = true;
		uploadError = '';
		try {
			coverUrl = await onUploadCover(file);
		} catch {
			uploadError = 'Gagal mengunggah cover. Periksa koneksi lalu coba lagi.';
		} finally {
			coverBusy = false;
			if (coverInput) coverInput.value = '';
		}
	}

	async function insertArticleImage(file: File) {
		if (!file || articleBusy) return;
		const body = bodyEl;
		if (!body) return;
		body.focus();
		articleBusy = true;
		uploadError = '';
		try {
			const url = await onUploadCover(file);
			document.execCommand('insertImage', false, url);
			sync();
		} catch {
			uploadError = 'Gagal mengunggah gambar artikel.';
		} finally {
			articleBusy = false;
			if (articleImageInput) articleImageInput.value = '';
		}
	}

	/** Anchor di bawah seleksi saat ini (bila ada). */
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
		const body = bodyEl;
		if (!body || saving) return;
		const sel = window.getSelection();
		savedRange = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).cloneRange() : null;
		const anchor = closestAnchor();
		linkDialog = { open: true, url: anchor?.getAttribute('href') ?? '', isEdit: Boolean(anchor) };
	}

	function applyLinkDialog() {
		const body = bodyEl;
		if (!body) {
			linkDialog = { ...linkDialog, open: false };
			return;
		}
		const url = linkDialog.url.trim();
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
		const anchor = closestAnchor();
		if (!url) {
			if (anchor) {
				const range = document.createRange();
				range.selectNodeContents(anchor);
				sel?.removeAllRanges();
				sel?.addRange(range);
				document.execCommand('unlink', false);
			}
		} else {
			if (anchor && sel && sel.isCollapsed) {
				const range = document.createRange();
				range.selectNodeContents(anchor);
				sel.removeAllRanges();
				sel.addRange(range);
			}
			document.execCommand('createLink', false, url);
		}
		savedRange = null;
		linkDialog = { ...linkDialog, open: false };
		sync();
	}

	function openEmbedDialog() {
		if (!bodyEl || saving) return;
		const sel = window.getSelection();
		savedRange = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).cloneRange() : null;
		embedDialog = { open: true, url: '', error: '' };
	}

	/**
	 * Sisipkan embed video/peta di posisi kursor. Hanya url dari penyedia yang
	 * dikenal yang diterima (parity lib/media/embed).
	 */
	function resolveEmbed(url: string): string | null {
		const raw = url.trim();
		if (!raw) return null;
		let u: URL;
		try {
			u = new URL(raw);
		} catch {
			return null;
		}
		if (u.protocol !== 'https:' && u.protocol !== 'http:') return null;
		const host = u.hostname.toLowerCase();
		let videoId: string | null = null;
		if (host.includes('youtube.com')) videoId = u.searchParams.get('v');
		else if (host === 'youtu.be') videoId = u.pathname.slice(1);
		if (videoId) {
			return `<iframe width="560" height="315" src="https://www.youtube.com/embed/${encodeURIComponent(videoId)}" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`;
		}
		if (host.includes('vimeo.com')) {
			const id = u.pathname.split('/').filter(Boolean)[0];
			if (id) return `<iframe width="560" height="315" src="https://player.vimeo.com/video/${encodeURIComponent(id)}" allow="autoplay; fullscreen; picture-in-picture" allowfullscreen></iframe>`;
		}
		if (host.includes('google.') && (u.pathname.startsWith('/maps') || u.searchParams.has('pb'))) {
			return `<iframe width="600" height="450" src="${u.toString()}" loading="lazy" allowfullscreen></iframe>`;
		}
		return null;
	}

	function applyEmbedDialog() {
		const body = bodyEl;
		if (!body) {
			embedDialog = { ...embedDialog, open: false };
			return;
		}
		const html = resolveEmbed(embedDialog.url);
		if (!html) {
			embedDialog = { ...embedDialog, error: 'Tautan tidak dikenali. Yang didukung: YouTube, Vimeo, Google Maps.' };
			return;
		}
		// Bangun node dari string hasil resolveEmbed (bukan input mentah).
		const holder = document.createElement('div');
		holder.innerHTML = html;
		const node = holder.firstElementChild;
		if (!node) {
			embedDialog = { ...embedDialog, error: 'Gagal menyiapkan embed.' };
			return;
		}
		body.focus();
		const sel = window.getSelection();
		if (sel && savedRange && body.contains(savedRange.commonAncestorContainer)) {
			sel.removeAllRanges();
			sel.addRange(savedRange);
		}
		const range = sel && sel.rangeCount > 0 ? sel.getRangeAt(0) : null;
		if (range) {
			range.deleteContents();
			range.insertNode(node);
			range.setStartAfter(node);
			range.collapse(true);
			sel?.removeAllRanges();
			sel?.addRange(range);
		} else {
			body.appendChild(node);
		}
		sync();
		embedDialog = { open: false, url: '', error: '' };
	}

	function handleSave() {
		if (linkOverLimit) {
			warnOpen = true;
			return;
		}
		if (!coverUrl?.trim()) {
			coverConfirmOpen = true;
			return;
		}
		onSave(title.trim() || 'Artikel', bodyEl?.innerHTML ?? '', coverUrl, category.trim(), metaDescription.trim());
	}

	function saveWithFallbackCover() {
		coverUrl = TAUTHUB_FALLBACK_COVER;
		coverConfirmOpen = false;
		onSave(title.trim() || 'Artikel', bodyEl?.innerHTML ?? '', TAUTHUB_FALLBACK_COVER, category.trim(), metaDescription.trim());
	}

	// Pantau seleksi: tombol link aktif bila kursor di dalam <a>.
	$effect(() => {
		function onSelectionChange() {
			linkActive = Boolean(closestAnchor());
		}
		document.addEventListener('selectionchange', onSelectionChange);
		return () => document.removeEventListener('selectionchange', onSelectionChange);
	});
</script>

<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.55);" role="dialog" aria-modal="true" aria-label={`Editor artikel ${mediaName}`}>
	<div class="app-card my-4 w-full max-w-3xl p-5">
		<div>
			<p class="font-bold">{mediaName}</p>
			<p class="text-xs text-muted-foreground">Edit artikel lalu simpan.</p>
		</div>

		<div class="mt-3 space-y-3">
			<input class="app-input w-full text-base font-semibold" aria-label="Judul artikel" maxlength="300" placeholder="Judul artikel" bind:value={title} />

			<!-- Cover artikel -->
			<div>
				<input
					bind:this={coverInput}
					type="file"
					accept="image/*"
					class="hidden"
					onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void pickCover(f); }}
				/>
				{#if coverUrl}
					<div class="relative">
						<img src={coverUrl} alt="Cover artikel" class="max-h-56 w-full rounded-lg object-cover" />
						<button class="app-btn app-btn-soft absolute right-2 bottom-2 !min-h-7 !px-2 !text-xs" type="button" disabled={coverBusy} onclick={() => coverInput?.click()}>
							{coverBusy ? 'Mengunggah…' : 'Ganti Cover'}
						</button>
					</div>
				{:else}
					<button class="flex w-full flex-col items-center gap-1 rounded-xl border-2 border-dashed p-5 text-center" type="button" disabled={coverBusy} onclick={() => coverInput?.click()}>
						<Upload size={20} class="text-muted-foreground" />
						<span class="text-sm font-medium">{coverBusy ? 'Mengunggah cover…' : 'Cover artikel belum diunggah'}</span>
						<span class="text-xs text-muted-foreground">Klik untuk mengunggah, atau tempel gambar (Ctrl+V)</span>
					</button>
				{/if}
			</div>

			<div class="flex flex-wrap items-center gap-2">
				<span class="text-xs font-medium text-muted-foreground">Kategori artikel</span>
				{#if categoryOptions.length > 0}
					<select class="app-input !w-auto min-w-[200px]" aria-label="Kategori artikel" bind:value={category}>
						{#if category && !categoryOptions.includes(category)}<option value={category}>{category}</option>{/if}
						{#each categoryOptions as c (c)}<option value={c}>{c}</option>{/each}
					</select>
				{:else}
					<span class="text-xs text-muted-foreground">Media ini belum punya kategori dari mitra — atur di dashboard mitra.</span>
				{/if}
			</div>

			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Meta deskripsi</span>
				<textarea class="app-input mt-1 w-full" rows="2" maxlength="500" placeholder="Ringkasan 120-160 karakter untuk hasil pencarian — terisi otomatis oleh AI" bind:value={metaDescription}></textarea>
			</label>

			{#if uploadError}<p class="text-sm" style="color: var(--destructive);">{uploadError}</p>{/if}

			<!-- Toolbar -->
			<div class="flex flex-wrap items-center gap-1 rounded-lg border p-1" role="toolbar" aria-label="Format teks">
				{#each tools as t (t.cmd + (t.arg ?? ''))}
					{@const ToolIcon = t.icon}
					<button class="min-h-8 min-w-8 rounded px-2 text-sm font-semibold hover:bg-secondary" type="button" title={t.name} onclick={() => exec(t.cmd, t.arg)}>
						{#if ToolIcon}
							<ToolIcon size={15} />
						{:else}
							{t.label}
						{/if}
					</button>
				{/each}
				<input
					bind:this={articleImageInput}
					type="file"
					accept="image/*"
					class="hidden"
					onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void insertArticleImage(f); }}
				/>
				<button class="min-h-8 min-w-8 rounded px-2 hover:bg-secondary" type="button" title={articleBusy ? 'Mengunggah…' : 'Unggah gambar'} disabled={articleBusy} onclick={() => articleImageInput?.click()}>
					{#if articleBusy}<LoaderCircle size={15} class="animate-spin" />{:else}<Image size={15} />{/if}
				</button>
				<button
					class="min-h-8 min-w-8 rounded px-2 hover:bg-secondary"
					style="color: {linkActive ? 'var(--primary)' : 'inherit'};"
					type="button"
					onmousedown={(e: MouseEvent) => e.preventDefault()}
					title={linkActive ? 'Ubah / hapus tautan' : 'Sisipkan tautan'}
					aria-pressed={linkActive}
					onclick={openLinkDialog}
				><Link size={15} /></button>
				{#if allowEmbed}
					<button class="min-h-8 min-w-8 rounded px-2 hover:bg-secondary" type="button" onmousedown={(e: MouseEvent) => e.preventDefault()} title="Sisipkan video atau peta" onclick={openEmbedDialog}><Video size={15} /></button>
				{/if}
			</div>

			<div
				bind:this={bodyEl}
				class="prose prose-sm max-h-[46vh] min-h-[200px] overflow-y-auto rounded-lg border p-3 text-sm"
				contenteditable={!saving}
				role="textbox"
				aria-label="Isi artikel"
				oninput={sync}
			></div>

			{#if saveError}<p class="text-sm" style="color: var(--destructive);">{saveError}</p>{/if}
			{#if linkOverLimit}
				<p class="text-sm" style="color: var(--destructive);"><TriangleAlert size={13} class="inline" /> Terlalu banyak link aktif. Maks {linkLimit} tautan per media.</p>
			{/if}
		</div>

		<div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t pt-3">
			<span class="text-xs text-muted-foreground">
				{words} kata · Link
				<span style="color: {linkOverLimit ? 'var(--destructive)' : 'var(--muted-foreground)'}; font-weight: {linkOverLimit ? 700 : 400};">{linkCount}/{linkLimit}</span>
			</span>
			<div class="flex flex-wrap items-center gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={saving || regenDisabled || !onRegenerate} title={regenDisabled ? regenTitle : undefined} onclick={onRegenerate}>
					<RotateCcw size={14} /> {regenLabel}
				</button>
				<button class="app-btn app-btn-soft" type="button" disabled={saving} onclick={onClose}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" disabled={saving} onclick={handleSave}>
					{#if saving}<LoaderCircle size={14} class="animate-spin" />{/if}
					{saving ? 'Menyimpan…' : 'Simpan'}
				</button>
			</div>
		</div>
	</div>
</div>

{#if warnOpen}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="flex items-center gap-2 font-bold" style="color: var(--warning);"><TriangleAlert size={18} /> Link Terlalu Banyak</p>
			<p class="mt-2 text-sm">
				Artikel ini berisi <strong>{linkCount}</strong> tautan aktif, tetapi media ini hanya memperbolehkan maksimal <strong>{linkLimit}</strong> tautan.
				Kurangi URL aktif ({linkCount - linkLimit} tautan berlebih) sebelum menyimpan.
			</p>
			<p class="mt-2 text-xs" style="color: var(--destructive);">Artikel tidak akan tersimpan sampai jumlah link sesuai batas media.</p>
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (warnOpen = false)}>Edisi Ulang</button>
			</div>
		</div>
	</div>
{/if}

{#if coverConfirmOpen}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="flex items-center gap-2 font-bold" style="color: var(--destructive);"><TriangleAlert size={18} /> Cover Artikel Masih Kosong</p>
			<p class="mt-2 text-sm" style="color: var(--destructive);">
				Artikel ini <strong>wajib memiliki gambar cover</strong>. Tanpa cover, artikel akan otomatis memakai thumbnail/logo bawaan Tauthub.
			</p>
			<p class="mt-2 text-sm">Lanjutkan tanpa cover (artikel memakai gambar fallback Tauthub), atau tambahkan gambar sekarang?</p>
			<div class="mt-4 flex justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => { coverConfirmOpen = false; coverInput?.click(); }}>Tidak, saya tambah gambar</button>
				<button class="app-btn app-btn-primary" type="button" onclick={saveWithFallbackCover}>Ya, pakai logo Tauthub</button>
			</div>
		</div>
	</div>
{/if}

{#if linkDialog.open}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="font-bold">{linkDialog.isEdit ? 'Ubah tautan' : 'Sisipkan tautan'}</p>
			<p class="mt-1 text-sm text-muted-foreground">
				{linkDialog.isEdit ? 'Ubah URL di bawah, atau kosongkan untuk melepas tautan.' : 'Masukkan URL untuk teks yang dipilih.'}
			</p>
			<input
				class="app-input mt-3 w-full"
				aria-label="URL tautan"
				placeholder="https://contoh.com/halaman"
				inputmode="url"
				bind:value={linkDialog.url}
				onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); applyLinkDialog(); } }}
			/>
			<div class="mt-4 flex justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (linkDialog = { ...linkDialog, open: false })}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" onclick={applyLinkDialog}>{linkDialog.url.trim() ? 'Terapkan' : 'Lepas tautan'}</button>
			</div>
		</div>
	</div>
{/if}

{#if embedDialog.open}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-lg p-5">
			<p class="font-bold">Sisipkan video atau peta</p>
			<p class="mt-1 text-sm text-muted-foreground">Tempel tautan video (YouTube/Vimeo) atau tautan embed peta dari Google Maps.</p>
			<input
				class="app-input mt-3 w-full"
				aria-label="URL embed"
				placeholder="https://www.youtube.com/watch?v=... atau https://www.google.com/maps/...&pb=..."
				bind:value={embedDialog.url}
				oninput={() => (embedDialog = { ...embedDialog, error: '' })}
				onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); applyEmbedDialog(); } }}
			/>
			{#if embedDialog.error}<p class="mt-2 text-xs" style="color: var(--destructive);">{embedDialog.error}</p>{/if}
			<p class="mt-2 text-xs text-muted-foreground">
				Embed hanya bisa disisipkan pada media yang mengizinkannya, dan hanya dari penyedia yang didukung.
			</p>
			<div class="mt-4 flex justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (embedDialog = { ...embedDialog, open: false })}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" onclick={applyEmbedDialog}>Sisipkan</button>
			</div>
		</div>
	</div>
{/if}
