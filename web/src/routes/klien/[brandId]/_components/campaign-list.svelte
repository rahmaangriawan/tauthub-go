<script lang="ts">
	// Daftar & manajemen kampanye Taut Pintar klien.
	// Port app/klien/[brandId]/taut-pintar/{page,client}.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Archive, ArrowRight, Clock, FileText, LoaderCircle, RotateCcw, Search, Share2, Sparkles, Trash, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { STAGE_LABEL, STAGE_TONE, STATUS_OPTIONS, statusAccent, fmtIdr } from '../taut-pintar/_shared';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Campaign = {
		id: string;
		name: string;
		taut_status: string | null;
		media_type: string | null;
		budget_strategy: string | null;
		starts_at: string | null;
		created_at: string | null;
		archived?: boolean;
		stats: { articles: number; published_media: number; tc_used: number };
	};

	const PER_PAGE = 9;

	let list = $state<Campaign[]>([]);
	let total = $state(0);
	let pageNum = $state(1);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let showArchive = $state(false);
	let rowBusyId = $state('');
	let search = $state('');
	let query = $state('');
	let statusFilter = $state('all');

	// Popup share berita acara.
	let shareFor = $state<Campaign | null>(null);
	let shareToken = $state('');
	let shareLoading = $state(false);
	let shareError = $state('');

	// Popup buat kampanye.
	let createOpen = $state(false);
	let createName = $state('');
	let createMode = $state<'ai' | 'manual' | null>(null);

	let filterTimer: number | null = null;

	const totalPages = $derived(Math.max(1, Math.ceil(total / PER_PAGE)));
	// Filter status client-side (parity app lama; API tak menerima status).
	const filteredList = $derived(
		statusFilter === 'all' ? list : list.filter((c) => (c.taut_status ?? 'draft') === statusFilter)
	);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadList();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			if (filterTimer) window.clearTimeout(filterTimer);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	async function loadList() {
		loading = true;
		try {
			const sp = new URLSearchParams();
			if (showArchive) sp.set('archived', '1');
			if (query) sp.set('q', query);
			sp.set('page', String(pageNum));
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns?${sp}`);
			if (!res.ok) return;
			const data = await res.json();
			list = data.campaigns ?? [];
			total = data.total ?? 0;
		} finally {
			loading = false;
		}
	}

	function onSearch(value: string) {
		search = value;
		pageNum = 1;
		if (filterTimer) window.clearTimeout(filterTimer);
		filterTimer = window.setTimeout(() => {
			query = search.trim();
			void loadList();
		}, 300);
	}

	function toggleArchiveView() {
		showArchive = !showArchive;
		pageNum = 1;
		void loadList();
	}

	function resetFilters() {
		statusFilter = 'all';
		search = '';
		query = '';
		pageNum = 1;
		void loadList();
	}

	async function archiveCampaign(id: string) {
		if (!window.confirm('Hapus kampanye ini? Kampanye dipindahkan ke Arsip dan masih bisa dikembalikan.')) return;
		rowBusyId = id;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menghapus kampanye.'));
			await loadList();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : String(reason);
		} finally {
			rowBusyId = '';
		}
	}

	async function archiveAction(id: string, action: 'restore' | 'purge') {
		if (action === 'purge' && !window.confirm('Hapus permanen? Kampanye hilang selamanya dan tidak bisa dikembalikan.')) return;
		rowBusyId = id;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${id}?action=${action}`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memproses kampanye.'));
			await loadList();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : String(reason);
		} finally {
			rowBusyId = '';
		}
	}

	async function openShare(id: string) {
		const c = list.find((x) => x.id === id) ?? null;
		shareFor = c;
		shareToken = '';
		shareError = '';
		shareLoading = Boolean(c);
		if (!c) return;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/campaigns/${c.id}/report/share`, { method: 'POST' });
			const j = await res.json().catch(() => ({}));
			if (!res.ok) throw new Error(j.detail ?? 'Gagal membuat tautan share.');
			shareToken = j.token ?? '';
		} catch (reason) {
			shareError = reason instanceof Error ? reason.message : String(reason);
		} finally {
			shareLoading = false;
		}
	}

	function closeShare() {
		shareFor = null;
		shareToken = '';
		shareError = '';
	}

	const shareUrl = $derived(shareToken ? `${typeof window !== 'undefined' ? window.location.origin : ''}/berita-acara/${shareToken}` : '');

	async function copyShare() {
		if (!shareUrl) return;
		try {
			await navigator.clipboard?.writeText(shareUrl);
		} catch {
			/* clipboard tidak tersedia */
		}
	}

	function closeCreate() {
		if (busy) return;
		createOpen = false;
		createMode = null;
	}

	async function createCampaign(mode: 'ai' | 'manual') {
		if (busy) {
			error = 'Mohon tunggu, kampanye sedang dibuat…';
			return;
		}
		const campaignName = createName.trim();
		if (campaignName.length < 2) {
			error = 'Nama kampanye minimal 2 karakter.';
			return;
		}
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns`, {
				method: 'POST',
				body: JSON.stringify({
					name: campaignName,
					starts_at: new Date().toISOString(),
					entry_mode: mode
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal membuat kampanye.'));
			const data = await res.json();
			// Mode menentukan step awal: AI → Profiler; Manual → Matchmaker tanpa AI.
			if (mode === 'ai') {
				await goto(`/klien/${brandUrl}/taut-pintar/${data.id}/profiler`);
			} else {
				await goto(`/klien/${brandUrl}/taut-pintar/${data.id}/matchmaker?mode=manual`);
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : String(reason);
			busy = false;
		}
	}

	function handleContinue(c: Campaign) {
		// Selalu lewat router root [id]/+page.svelte — biar bukti publikasi +
		// status aktual yang memutuskan tujuan, bukan shortcut client yang bisa
		// membuka /profiler basi untuk kampanye yang sudah selesai.
		void goto(`/klien/${brandUrl}/taut-pintar/${c.id}`);
	}

	const badgeStyle = (tone: string) =>
		tone === 'success'
			? 'background-color: var(--success-soft); color: var(--success);'
			: tone === 'warning'
				? 'background-color: var(--warning-soft); color: var(--warning);'
				: tone === 'accent'
					? 'background-color: var(--accent); color: var(--accent-foreground);'
					: 'background-color: var(--muted); color: var(--muted-foreground);';

	const accentColor = (a: string) =>
		a === 'success' ? 'var(--success)' : a === 'warning' ? 'var(--warning)' : a === 'accent' ? 'var(--primary)' : 'var(--border)';
</script>

<section class="mx-auto max-w-6xl">
	<header class="flex flex-wrap items-center justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Manajemen Kampanye</h1>
			<p class="mt-1 text-sm text-muted-foreground">Kelola semua kampanye distribusi konten Anda di satu tempat.</p>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-2">
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={toggleArchiveView}>
				{#if showArchive}<RotateCcw size={16} />{:else}<Archive size={16} />{/if}
				{showArchive ? 'Kembali ke Daftar' : 'Arsip'}
			</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy || showArchive} onclick={() => (createOpen = true)}>
				{#if busy}<LoaderCircle size={16} class="animate-spin" />{/if}
				{busy ? 'Membuat…' : 'Kampanye Baru'}
			</button>
		</div>
	</header>

	<div class="app-card mt-4 flex flex-wrap items-center gap-2 p-3">
		<select class="app-input !w-auto min-w-[160px]" bind:value={statusFilter} onchange={() => (pageNum = 1)}>
			{#each STATUS_OPTIONS as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
		</select>
		<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder="Cari nama kampanye…"
				aria-label="Cari kampanye"
				value={search}
				oninput={(e) => onSearch((e.currentTarget as HTMLInputElement).value)}
			/>
		</label>
		<button class="app-btn app-btn-soft ml-auto" type="button" onclick={resetFilters}>Reset</button>
	</div>

	{#if error}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);" role="alert">{error}</div>
	{/if}

	{#if showArchive}
		<p class="mt-3 text-sm text-muted-foreground">
			Kampanye yang dihapus disimpan di sini. Kampanye bisa dikembalikan, atau dihapus permanen.
		</p>
	{/if}

	{#if filteredList.length === 0}
		<div class="app-card mt-4 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{showArchive
					? 'Arsip kosong.'
					: query || statusFilter !== 'all'
						? 'Tidak ada kampanye yang cocok.'
						: 'Belum ada kampanye. Klik "Kampanye Baru" untuk memulai.'}
			</p>
		</div>
	{:else}
		<div class="mt-4 grid gap-[14px] sm:grid-cols-2 xl:grid-cols-3">
			{#each filteredList as c (c.id)}
				{@const accent = statusAccent(c.taut_status, c.stats.published_media)}
				{@const totalMedia = Math.max(c.stats.articles, c.stats.published_media)}
				<div class="app-card flex flex-col" style="padding: 18px 20px 14px; border-radius: 14px; border-left: 3px solid {accentColor(accent)};">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<div class="flex flex-wrap items-center gap-2">
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badgeStyle('accent')}>DIBANTU AI</span>
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badgeStyle(c.stats.published_media > 0 ? 'success' : (STAGE_TONE[c.taut_status ?? 'draft'] ?? 'muted'))}>
								{(STAGE_LABEL[c.taut_status ?? 'draft'] ?? c.taut_status ?? 'Draft').toUpperCase()}
							</span>
						</div>
						<div class="flex items-center gap-2">
							{#if c.created_at}
								<span class="text-[11px] text-muted-foreground">Dibuat: {new Date(c.created_at).toLocaleDateString('id-ID')}</span>
							{/if}
							{#if !showArchive}
								<div class="flex gap-1">
									<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Berita acara & share" onclick={() => void openShare(c.id)}>
										<Share2 size={14} />
									</button>
									<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Hapus ke arsip" disabled={rowBusyId === c.id} onclick={() => void archiveCampaign(c.id)}>
										<Archive size={14} />
									</button>
								</div>
							{/if}
						</div>
					</div>

					<p class="mt-3 truncate font-semibold">{c.name}</p>
					<p class="mt-1 text-xs text-muted-foreground">
						{c.media_type
							? `Tipe media: ${c.media_type} · Strategi anggaran: ${c.budget_strategy ?? '-'} · Mulai ${c.starts_at ? new Date(c.starts_at).toLocaleDateString('id-ID') : '-'}`
							: 'Kampanye distribusi konten PR otomatis.'}
					</p>

					<div class="mt-3 flex flex-wrap items-end justify-between gap-3 border-t pt-3">
						<div>
							<p class="text-[10px] font-semibold tracking-wide text-muted-foreground">ANGGARAN TERSERAP</p>
							<p class="text-sm font-semibold tabular-nums">{fmtIdr(c.stats.tc_used)} Kredit</p>
						</div>
						<div>
							<p class="text-[10px] font-semibold tracking-wide text-muted-foreground">PROGRES MEDIA</p>
							<p class="text-sm font-semibold tabular-nums">
								<span style="color: {c.stats.published_media > 0 ? 'var(--success)' : 'inherit'};">{c.stats.published_media} Terbit</span>
								{' / '}{totalMedia} Total
							</p>
						</div>
						<button class="ml-auto inline-flex items-center gap-1 text-sm font-medium" type="button" style="color: var(--primary);" onclick={() => handleContinue(c)}>
							Buka Kampanye <ArrowRight size={14} />
						</button>
					</div>

					{#if showArchive}
						<div class="mt-3 flex gap-2 border-t pt-3">
							<button class="app-btn app-btn-primary flex-1" type="button" disabled={rowBusyId === c.id} onclick={() => void archiveAction(c.id, 'restore')}>
								<Clock size={14} /> Pulihkan
							</button>
							<button class="app-btn app-btn-soft" type="button" aria-label="Hapus permanen" disabled={rowBusyId === c.id} onclick={() => void archiveAction(c.id, 'purge')}>
								<Trash size={14} />
							</button>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		{#if totalPages > 1 || loading}
			<div class="mt-4 flex flex-wrap items-center justify-between gap-2">
				<p class="text-xs text-muted-foreground">{loading ? 'Memuat…' : `${total} kampanye`}</p>
				<div class="flex items-center gap-2">
					<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label="Halaman sebelumnya" disabled={pageNum <= 1 || loading} onclick={() => { pageNum = Math.max(1, pageNum - 1); void loadList(); }}>
						‹
					</button>
					<span class="text-xs text-muted-foreground">Halaman {pageNum} dari {totalPages}</span>
					<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label="Halaman berikutnya" disabled={pageNum >= totalPages || loading} onclick={() => { pageNum = Math.min(totalPages, pageNum + 1); void loadList(); }}>
						›
					</button>
				</div>
			</div>
		{/if}
	{/if}
</section>

{#if shareFor}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-6 w-full max-w-lg p-5" aria-label="Berita Acara Kampanye">
			<div class="flex items-start justify-between gap-4">
				<div>
					<h2 class="font-bold">Berita Acara Kampanye</h2>
					<p class="mt-0.5 text-sm text-muted-foreground">Detail kampanye &amp; tautan share publik untuk klien.</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Tutup" onclick={closeShare}><X size={18} /></button>
			</div>

			<div class="mt-4 space-y-3 text-sm">
				<div>
					<p class="text-xs text-muted-foreground">Nama kampanye</p>
					<p class="font-bold">{shareFor.name}</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Status</p>
					<span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" style={badgeStyle('accent')}>
						{STAGE_LABEL[shareFor.taut_status ?? 'draft'] ?? shareFor.taut_status}
					</span>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Jenis media</p>
					<p>{shareFor.media_type ?? 'Belum dipilih'}</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Mulai</p>
					<p>{shareFor.starts_at ? new Date(shareFor.starts_at).toLocaleDateString('id-ID') : '-'}</p>
				</div>
			</div>

			<div class="mt-4 flex flex-wrap gap-3">
				<div class="min-w-[120px] flex-1">
					<p class="text-xs text-muted-foreground">Artikel</p>
					<p class="text-lg font-bold">{shareFor.stats.articles ?? 0}</p>
				</div>
				<div class="min-w-[120px] flex-1">
					<p class="text-xs text-muted-foreground">Media terbit</p>
					<p class="text-lg font-bold">{shareFor.stats.published_media ?? 0}</p>
				</div>
				<div class="min-w-[120px] flex-1">
					<p class="text-xs text-muted-foreground">TC terpakai</p>
					<p class="text-lg font-bold">{shareFor.stats.tc_used ?? 0}</p>
				</div>
			</div>

			{#if shareError}<p class="mt-2 text-xs" style="color: var(--destructive);">{shareError}</p>{/if}

			<div class="mt-4 space-y-2">
				<p class="text-xs text-muted-foreground">Tautan share publik:</p>
				{#if shareToken}
					<div class="flex items-center gap-2">
						<input class="app-input flex-1" readonly value={shareUrl} />
						<button class="app-btn app-btn-soft" type="button" onclick={copyShare}>Salin</button>
					</div>
				{:else}
					<button class="app-btn app-btn-soft" type="button" disabled={shareLoading} onclick={() => void openShare(shareFor!.id)}>
						{#if shareLoading}<LoaderCircle size={14} class="animate-spin" />{/if}
						Buat tautan share publik
					</button>
				{/if}
			</div>
		</dialog>
	</div>
{/if}

{#if createOpen}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-6 w-full max-w-lg p-5" aria-label="Buat Kampanye">
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-lg font-semibold">Buat Kampanye</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Tutup" disabled={busy} onclick={closeCreate}><X size={16} /></button>
			</div>
			<p class="mt-1 text-sm text-muted-foreground">Pilih cara kamu membuat kampanye distribusi konten.</p>

			<label class="mt-3 block">
				<span class="text-sm font-medium">Nama Kampanye <span style="color: var(--destructive);">*</span></span>
				<input
					class="app-input mt-1 w-full"
					placeholder="Contoh: Launching Produk Q3 2026"
					value={createName}
					oninput={(e) => { createName = (e.currentTarget as HTMLInputElement).value; if (error) error = ''; }}
				/>
			</label>

			{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

			<div class="mt-4 grid grid-cols-2 gap-3">
				<button
					type="button"
					class="rounded-xl border p-3 text-left transition-colors"
					style="border-color: {createMode === 'ai' ? 'var(--primary)' : 'var(--border)'}; background-color: {createMode === 'ai' ? 'var(--accent)' : 'transparent'};"
					disabled={busy}
					onclick={() => (createMode = 'ai')}
				>
					<span class="flex items-center gap-2">
						<Sparkles size={16} style="color: var(--primary);" />
						<span class="font-bold">AI Pintar</span>
					</span>
					<span class="mt-1 block text-xs text-muted-foreground">
						Profiler: AI menyusun strategi &amp; brief, lalu Matchmaker pilih media. Butuh 10 Kredit.
					</span>
				</button>
				<button
					type="button"
					class="rounded-xl border p-3 text-left transition-colors"
					style="border-color: {createMode === 'manual' ? 'var(--primary)' : 'var(--border)'}; background-color: {createMode === 'manual' ? 'var(--accent)' : 'transparent'};"
					disabled={busy}
					onclick={() => (createMode = 'manual')}
				>
					<span class="flex items-center gap-2">
						<FileText size={16} class="text-muted-foreground" />
						<span class="font-bold">Manual</span>
					</span>
					<span class="mt-1 block text-xs text-muted-foreground">
						The Writer tanpa AI: siapkan media &amp; unggah artikel tulisanmu sendiri.
					</span>
				</button>
			</div>

			{#if busy}<div class="mt-3 flex justify-center"><LoaderCircle size={20} class="animate-spin text-muted-foreground" /></div>{/if}

			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-primary" type="button" disabled={busy || !createMode} onclick={() => createMode && void createCampaign(createMode)}>
					{#if busy}<LoaderCircle size={14} class="animate-spin" />{/if}
					Mulai Sekarang
				</button>
			</div>
		</dialog>
	</div>
{/if}
