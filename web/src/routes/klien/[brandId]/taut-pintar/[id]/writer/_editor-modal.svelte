<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Modal editor rich untuk Writer — dipakai untuk Preview hasil generate
	// maupun edit hasil Unggah Artikel. Port writer/editor-modal.tsx.
	// contentEditable + execCommand (tanpa dependensi baru).
	import { Image, Info, Link, List, ListOrdered, LoaderCircle, Quote, RotateCcw, RotateCw, TriangleAlert, Upload, Video, X } from '@lucide/svelte';
	import type { Component, Snippet } from 'svelte';
	import Select from '$lib/components/select.svelte';
	import { TAUTHUB_FALLBACK_COVER } from '$lib/media/cover';
	import { sanitizeHtml } from '$lib/sanitize-html';

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
		extraFields
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
		extraFields?: Snippet;
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
	let coverInfoOpen = $state(false);
	// Ukuran gambar (byte) — cover + gambar di dalam artikel, untuk footer editor.
	let coverBytes = $state(0);
	let articleBytes = $state(0);
	let linkDialog = $state<{ open: boolean; url: string; isEdit: boolean }>({ open: false, url: '', isEdit: false });
	let embedDialog = $state<{ open: boolean; url: string; error: string }>({ open: false, url: '', error: '' });
	let savedRange: Range | null = null;

	const linkOverLimit = $derived(linkCount > linkLimit);
	const totalImageKb = $derived(coverBytes + articleBytes);
	const fmtKb = (b: number) => (b > 0 ? `${Math.round(b / 1024)}KB` : '0KB');

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
		const imgs = Array.from(bodyEl?.querySelectorAll('img') ?? [])
			.map((i) => i.getAttribute('src') ?? '')
			.filter(Boolean);
		articleBytes = (await Promise.all(imgs.map(measureBytes))).reduce((a, b) => a + b, 0);
	}

	// Ukur gambar di dalam artikel setiap isi berubah (words jadi penanda isi).
	$effect(() => {
		words;
		void measureArticleImages();
	});

	// Ukur cover saat berubah.
	$effect(() => {
		let alive = true;
		const url = coverUrl;
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
	$effect(() => {
		function onPaste(e: ClipboardEvent) {
			if (coverUrl || coverBusy) return;
			const target = e.target as Node | null;
			if (target && bodyEl?.contains(target)) return;
			const img = Array.from(e.clipboardData?.items ?? []).find((i) => i.type.startsWith('image/'));
			const file = img?.getAsFile();
			if (!file) return;
			e.preventDefault();
			void pickCover(file);
		}
		document.addEventListener('paste', onPaste);
		return () => document.removeEventListener('paste', onPaste);
	});

	// Isi body sekali saat mount: ekstrak H1 ke field judul, sisanya jadi body.
	// Sanitasi di sini defense-in-depth — konten tersimpan sudah disanitasi
	// server-side, dan sanitizer sama dipakai rich-text-editor.
	$effect(() => {
		const body = bodyEl;
		if (!body) return;
		const { h1, body: rest } = extractH1(initialHtml);
		body.innerHTML = sanitizeHtml(rest || initialHtml);
		const t = window.setTimeout(() => {
			if (h1) title = h1;
			sync();
		}, 0);
		return () => window.clearTimeout(t);
	});

	// Escape menutup modal.
	$effect(() => {
		function onKey(e: KeyboardEvent) {
			if (e.key === 'Escape' && !saving && !coverInfoOpen) onClose();
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

	/* Klik pada tautan: cegah navigasi, seleksi isi link agar terlihat bedanya. */
	function handleBodyClick(e: MouseEvent) {
		const body = bodyEl;
		const el = e.target instanceof Element ? e.target : null;
		const anchor = el?.closest('a');
		if (!body || !anchor || !body.contains(anchor)) return;
		e.preventDefault();
		const range = document.createRange();
		range.selectNodeContents(anchor);
		const sel = window.getSelection();
		sel?.removeAllRanges();
		sel?.addRange(range);
	}

	function exec(cmd: string, arg?: string) {
		const body = bodyEl;
		if (!body) return;
		body.focus();
		document.execCommand(cmd, false, arg);
		sync();
	}

	const tools: Array<{ cmd: string; label?: string; icon?: Component; arg?: string; name: string }> = $derived([
		{ cmd: 'bold', label: 'B', name: tr('Tebal') },
		{ cmd: 'italic', label: 'I', name: tr('Miring') },
		{ cmd: 'underline', label: 'U', name: tr('Garis bawah') },
		{ cmd: 'formatBlock', label: 'H2', arg: 'h2', name: tr('Judul bagian') },
		{ cmd: 'formatBlock', label: 'H3', arg: 'h3', name: tr('Sub-judul') },
		{ cmd: 'formatBlock', arg: 'blockquote', icon: Quote, name: tr('Kutipan') },
		{ cmd: 'insertUnorderedList', icon: List, name: tr('Daftar poin') },
		{ cmd: 'insertOrderedList', icon: ListOrdered, name: tr('Daftar bernomor') },
		{ cmd: 'undo', icon: RotateCcw, name: tr('Urungkan') },
		{ cmd: 'redo', icon: RotateCw, name: tr('Ulangi') }
	]);

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
			uploadError = tr('Gagal mengunggah gambar artikel.');
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
			embedDialog = { ...embedDialog, error: tr('Tautan tidak dikenali. Yang didukung: YouTube, Vimeo, Google Maps.') };
			return;
		}
		// Bangun node dari string hasil resolveEmbed (bukan input mentah).
		const holder = document.createElement('div');
		holder.innerHTML = html;
		const node = holder.firstElementChild;
		if (!node) {
			embedDialog = { ...embedDialog, error: tr('Gagal menyiapkan embed.') };
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
		if (enforceLinkLimit && linkOverLimit) {
			warnOpen = true;
			return;
		}
		if (requireCover && !coverUrl?.trim()) {
			coverConfirmOpen = true;
			return;
		}
		onSave(title.trim() || tr('Artikel'), bodyEl?.innerHTML ?? '', coverUrl, category.trim(), metaDescription.trim());
	}

	function saveWithFallbackCover() {
		coverUrl = TAUTHUB_FALLBACK_COVER;
		coverConfirmOpen = false;
		onSave(title.trim() || tr('Artikel'), bodyEl?.innerHTML ?? '', TAUTHUB_FALLBACK_COVER, category.trim(), metaDescription.trim());
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

<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(15, 23, 42, 0.45);" role="dialog" aria-modal="true" aria-label={`${tr('Editor artikel')} ${mediaName}`}>
	<div class="app-card my-auto w-full max-w-3xl p-5">
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

		<div class="mt-3 space-y-3">
			<input class="app-input w-full text-base font-semibold" aria-label="Judul artikel" maxlength="300" placeholder={tr("Judul artikel")} bind:value={title} />

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
						<img src={coverUrl} alt={tr('Cover artikel')} class="max-h-56 w-full rounded-lg object-cover" />
						<button class="app-btn absolute right-2.5 bottom-2.5 !min-h-7 !px-3 !text-xs" type="button" style="background-color: rgba(15, 23, 42, 0.72); color: #fff;" disabled={coverBusy} onclick={() => coverInput?.click()}>
							{coverBusy ? tr('Mengunggah…') : tr('Ganti Cover')}
						</button>
					</div>
				{:else}
					<button class="flex min-h-[132px] w-full flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed p-5 text-center" type="button" disabled={coverBusy} onclick={() => coverInput?.click()}>
						<Upload size={20} class="text-muted-foreground" />
						<span class="text-sm font-medium">{coverBusy ? tr('Mengunggah cover…') : tr('Cover artikel belum diunggah')}</span>
						<span class="text-xs text-muted-foreground">{tr("Klik untuk mengunggah, atau tempel gambar (Ctrl+V) · Maksimal 200KB · Lebih dari itu otomatis dikompres & diubah ke WebP oleh sistem TautHub")}</span>
					</button>
				{/if}
			</div>

			<div class="flex flex-wrap items-center gap-2">
				<span class="text-xs font-medium text-muted-foreground">{tr('Kategori artikel')}</span>
				{#if categoryOptions.length > 0}
					<Select
						bind:value={category}
						ariaLabel={tr('Kategori artikel')}
						minWidth={200}
						options={[
							...(category && !categoryOptions.includes(category) ? [{ value: category, label: category }] : []),
							...categoryOptions.map((c) => ({ value: c, label: c }))
						]}
					/>
				{:else}
					<span class="text-xs text-muted-foreground">{tr("Media ini belum punya kategori dari mitra — atur di dashboard mitra.")}</span>
				{/if}
			</div>

			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">{tr('Meta deskripsi')}</span>
				<textarea class="app-input mt-1 w-full" rows="2" maxlength="500" placeholder={tr("Ringkasan 120-160 karakter untuk hasil pencarian — terisi otomatis oleh AI")} bind:value={metaDescription}></textarea>
			</label>

			{#if uploadError}<p class="text-sm" style="color: var(--destructive);">{uploadError}</p>{/if}

			<!-- Info kemampuan editor: caption + bullet, dibuka lewat trigger kecil. -->
			<div class="relative">
				<button class="inline-flex items-center gap-1.5 self-start text-xs text-muted-foreground hover:text-foreground" type="button" onclick={() => (coverInfoOpen = !coverInfoOpen)}>
					<Info size={13} />
					{tr('Kemampuan editor artikel')}
				</button>
				{#if coverInfoOpen}
					<div class="mt-2 flex max-w-[360px] flex-col gap-2 rounded-lg border p-3" style="background-color: var(--card);">
						<div class="flex items-center gap-1.5">
							<Info size={14} style="color: var(--primary);" />
							<p class="text-sm font-bold">{tr('Kemampuan editor artikel')}</p>
						</div>
						<ul class="list-disc space-y-1 pl-5 text-xs text-muted-foreground">
							<li>{tr('Cover adalah gambar utama artikel yang tampil sebagai thumbnail di media')}</li>
							<li>{tr('Bisa mengubah judul dan seluruh isi teks jika diperlukan')}</li>
							<li>{tr('Bisa tambahkan gambar dalam artikel')}</li>
							<li>{tr('Bisa menambah atau mengubah hyperlink jika fitur tersedia')}</li>
							<li>{tr('Jika cover tidak diisi, artikel memakai gambar fallback bawaan saat ditinjau & didistribusikan')}</li>
						</ul>
					</div>
				{/if}
			</div>

			<!-- Toolbar -->
			<div class="flex flex-wrap items-center gap-1 rounded-lg border p-1" role="toolbar" aria-label={tr('Format teks')}>
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
				<button class="min-h-8 min-w-8 rounded px-2 hover:bg-secondary" type="button" title={articleBusy ? tr('Mengunggah…') : tr('Unggah gambar')} disabled={articleBusy} onclick={() => articleImageInput?.click()}>
					{#if articleBusy}<LoaderCircle size={15} class="animate-spin" />{:else}<Image size={15} />{/if}
				</button>
				<button
					class="min-h-8 min-w-8 rounded px-2 hover:bg-secondary"
					style="color: {linkActive ? 'var(--primary)' : 'inherit'};"
					type="button"
					onmousedown={(e: MouseEvent) => e.preventDefault()}
					title={linkActive ? tr('Ubah / hapus tautan') : tr('Sisipkan tautan')}
					aria-pressed={linkActive}
					onclick={openLinkDialog}
				><Link size={15} /></button>
				{#if allowEmbed}
					<button class="min-h-8 min-w-8 rounded px-2 hover:bg-secondary" type="button" onmousedown={(e: MouseEvent) => e.preventDefault()} title={tr("Sisipkan video atau peta")} onclick={openEmbedDialog}><Video size={15} /></button>
				{/if}
			</div>

			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<div
				bind:this={bodyEl}
				class="prose prose-sm max-h-[46vh] min-h-[200px] overflow-y-auto rounded-lg border p-3 text-sm"
				contenteditable={!saving}
				role="textbox"
				tabindex="0"
				aria-label={tr('Isi artikel')}
				oninput={sync}
				onclick={handleBodyClick}
			></div>

			{#if saveError}<p class="text-sm" style="color: var(--destructive);">{saveError}</p>{/if}
			{#if linkOverLimit}
				<p class="text-sm" style="color: var(--destructive);"><TriangleAlert size={13} class="inline" /> {tr('Terlalu banyak link aktif. Maks')} {linkLimit} {tr("tautan per media.")}</p>
			{/if}
		</div>

		{#if extraFields}
			<div class="mt-4 space-y-3 border-t pt-3">{@render extraFields()}</div>
		{/if}

		<div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t pt-3">
			<span class="inline-flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-xs text-muted-foreground">
				{words} {tr('kata')}
				<span style="opacity: .6;">·</span>
				<Image size={13} />
				<span>{fmtKb(totalImageKb)} ({tr('Cover')} {fmtKb(coverBytes)} · {tr('Artikel')} {fmtKb(articleBytes)})</span>
				{#if enforceLinkLimit}
					<span style="opacity: .6;">·</span>
					<Link size={13} />
					<span style="color: {linkOverLimit ? 'var(--destructive)' : 'var(--muted-foreground)'}; font-weight: {linkOverLimit ? 700 : 400};">{tr('Link')} {linkCount}/{linkLimit}</span>
				{/if}
			</span>
			<div class="flex flex-wrap items-center gap-2">
				{#if onRegenerate}
					<button class="app-btn app-btn-soft" type="button" disabled={saving || regenDisabled} title={regenDisabled ? regenTitle : undefined} onclick={onRegenerate}>
						<RotateCcw size={14} /> {regenLabel}
					</button>
				{/if}
				<button class="app-btn app-btn-soft" type="button" disabled={saving} onclick={onClose}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={saving} onclick={handleSave}>
					{#if saving}<LoaderCircle size={14} class="animate-spin" />{/if}
					{saving ? tr('Menyimpan…') : saveLabel}
				</button>
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

{#if linkDialog.open}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="font-bold">{linkDialog.isEdit ? tr('Ubah tautan') : tr('Sisipkan tautan')}</p>
			<p class="mt-1 text-sm text-muted-foreground">
				{linkDialog.isEdit ? tr('Ubah URL di bawah, atau kosongkan untuk melepas tautan.') : tr('Masukkan URL untuk teks yang dipilih.')}
			</p>
			<input
				class="app-input mt-3 w-full"
				aria-label={tr('URL tautan')}
				placeholder={tr("https://contoh.com/halaman")}
				inputmode="url"
				bind:value={linkDialog.url}
				onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); applyLinkDialog(); } }}
			/>
			<div class="mt-4 flex justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (linkDialog = { ...linkDialog, open: false })}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" onclick={applyLinkDialog}>{linkDialog.url.trim() ? tr('Terapkan') : tr('Lepas tautan')}</button>
			</div>
		</div>
	</div>
{/if}

{#if embedDialog.open}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.5);" role="presentation">
		<div class="app-card w-full max-w-lg p-5">
			<p class="font-bold">{tr("Sisipkan video atau peta")}</p>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Tempel tautan video (YouTube/Vimeo) atau tautan embed peta dari Google Maps.")}</p>
			<input
				class="app-input mt-3 w-full"
				aria-label="URL embed"
				placeholder={tr("https://www.youtube.com/watch?v=... atau https://www.google.com/maps/...&pb=...")}
				bind:value={embedDialog.url}
				oninput={() => (embedDialog = { ...embedDialog, error: '' })}
				onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); applyEmbedDialog(); } }}
			/>
			{#if embedDialog.error}<p class="mt-2 text-xs" style="color: var(--destructive);">{embedDialog.error}</p>{/if}
			<p class="mt-2 text-xs text-muted-foreground">{tr("Embed hanya bisa disisipkan pada media yang mengizinkannya, dan hanya dari penyedia yang didukung. Setelah disimpan, embed ikut terkirim saat artikel terbit di situs mitra.")}</p>
			<div class="mt-4 flex justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (embedDialog = { ...embedDialog, open: false })}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" onclick={applyEmbedDialog}>{tr('Sisipkan')}</button>
			</div>
		</div>
	</div>
{/if}
