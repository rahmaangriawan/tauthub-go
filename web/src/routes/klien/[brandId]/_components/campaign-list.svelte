<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { getI18n } from '$lib/i18n/i18n.svelte';

	// Daftar & manajemen kampanye Taut Pintar klien.
	// Port app/klien/[brandId]/taut-pintar/{page,client}.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Archive, ArrowRight, Check, Clock, FileText, LoaderCircle, Plus, RotateCcw, Search, Share2, Trash, X, Zap } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { campaignUrlId } from '$lib/campaign-url';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';
	import ProfilerGateModal from '$lib/components/profiler-gate-modal.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { STAGE_LABEL, STAGE_TONE, STATUS_OPTIONS, statusAccent } from '../taut-pintar/_shared';
	import { tcToRp } from '$lib/credits/rupiah';

	const brandId = $derived(page.params.brandId as string);
	const i18n = $derived(getI18n());
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Campaign = {
		id: string;
		public_id: string | null;
		name: string;
		taut_status: string | null;
		entry_mode: 'ai' | 'manual' | 'kilat' | null;
		media_type: string | null;
		budget_strategy: string | null;
		starts_at: string | null;
		created_at: string | null;
		archived?: boolean;
		/** Nomor putaran aktif: kampanye yang diulang menaikkan angka ini. */
		taut_round?: number | null;
		stats: { articles: number; published_media: number; tc_used: number };
	};

	const PER_PAGE = 9;

	let list = $state<Campaign[]>([]);
	let total = $state(0);
	let rateIdr = $state(1000);
	let pageNum = $state(1);
	let loading = $state(true);
	let busy = $state(false);
	// Popup gerbang Taut Brief — dipilih dulu sebelum masuk workspace.
	let profilerGateOpen = $state(false);
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
	let createMode = $state<'kilat' | 'manual' | null>('kilat');

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
			rateIdr = Number(data.rate_idr) || 1000;
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

	// Buka putaran baru di kampanye yang sudah selesai: status kembali ke
	// penempatan media, lalu katalog dibuka dalam mode tambah.
	async function addRound(c: Campaign) {
		if (rowBusyId) return;
		rowBusyId = c.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${c.id}/rounds`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menambah putaran baru.')));
			await goto(`/klien/${brandUrl}/media?campaign=${campaignUrlId(c)}`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : String(reason);
		} finally {
			rowBusyId = '';
		}
	}

	async function archiveCampaign(id: string) {
		if (!window.confirm(tr('Hapus kampanye ini? Kampanye dipindahkan ke Arsip dan masih bisa dikembalikan.'))) return;
		rowBusyId = id;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus kampanye.')));
			await loadList();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : String(reason);
		} finally {
			rowBusyId = '';
		}
	}

	async function archiveAction(id: string, action: 'restore' | 'purge') {
		if (action === 'purge' && !window.confirm(tr('Hapus permanen? Kampanye hilang selamanya dan tidak bisa dikembalikan.'))) return;
		rowBusyId = id;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${id}?action=${action}`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memproses kampanye.')));
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
			if (!res.ok) throw new Error(j.detail ?? tr('Gagal membuat tautan share.'));
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
		copied = false;
	}

	const shareUrl = $derived(shareToken ? `${typeof window !== 'undefined' ? window.location.origin : ''}/berita-acara/${shareToken}` : '');

	let copied = $state(false);

	async function copyShare() {
		if (!shareUrl) return;
		try {
			await navigator.clipboard?.writeText(shareUrl);
			copied = true;
			window.setTimeout(() => (copied = false), 2000);
		} catch {
			/* clipboard tidak tersedia */
		}
	}

	function closeCreate() {
		if (busy) return;
		createOpen = false;
		createMode = null;
	}

	async function createCampaign(mode: 'manual' | 'kilat') {
		if (busy) {
			error = tr('Mohon tunggu, kampanye sedang dibuat…');
			return;
		}
		const campaignName = createName.trim();
		if (campaignName.length < 2) {
			error = tr('Nama kampanye minimal 2 karakter.');
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
					// 'kilat' dicatat di kampanye supaya router tahu halaman ini milik
					// jalur Kilat, bukan The Profiler (lihat migrasi 169).
					entry_mode: mode
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat kampanye.')));
			const data = await res.json();
			// URL memakai public_id 6 karakter (migrasi 170); API menerima keduanya.
			const target = campaignUrlId(data);
			// Mode menentukan step awal: Kilat → halaman ringkasnya sendiri;
			// Manual → Matchmaker tanpa AI. (Mode AI Pintar/The Profiler sudah
			// tidak ditawarkan untuk kampanye baru.)
			if (mode === 'kilat') {
				await goto(`/klien/${brandUrl}/taut-pintar/${target}/kilat`);
			} else {
				await goto(`/klien/${brandUrl}/taut-pintar/${target}/matchmaker?mode=manual`);
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
		void goto(`/klien/${brandUrl}/taut-pintar/${campaignUrlId(c)}`);
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

<section>
	<header class="flex flex-wrap items-center justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr("Manajemen Kampanye")}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Kelola semua kampanye distribusi konten Anda di satu tempat.")}</p>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-2">
			<!-- Tombol Profiler: buka popup gerbang (Chat Brief / Instan Brief)
			     tanpa meninggalkan halaman kampanye dulu. -->
			<button class="app-btn app-btn-soft profilerBtn" type="button" disabled={busy} title={tr('Buka Taut Brief')} onclick={() => (profilerGateOpen = true)}>
				{tr('Profiler')}
				<span class="profilerAiBadge">AI</span>
			</button>
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={toggleArchiveView}>
				{#if showArchive}<RotateCcw size={16} />{:else}<Archive size={16} />{/if}
				{showArchive ? tr('Kembali ke Daftar') : tr('Arsip')}
			</button>
			<!-- SEMENTARA: popup create ditimpa redirect ke katalog media; kembalikan ke (createOpen = true) saat fitur siap.
			     Parameter ?from=new-campaign dipakai katalog untuk menampilkan banner konteks. -->
			<button class="app-btn app-btn-primary" type="button" disabled={busy || showArchive} onclick={() => void goto(`/klien/${brandUrl}/media?from=new-campaign`)}>
				{#if busy}<LoaderCircle size={16} class="animate-spin" />{/if}
				{busy ? 'Membuat…' : tr('Kampanye Baru')}
			</button>
		</div>
	</header>

	<div class="app-card mt-4 flex flex-wrap items-center gap-2 p-3">
		<Select
			bind:value={statusFilter}
			ariaLabel="Status kampanye"
			minWidth={160}
			onchange={() => (pageNum = 1)}
			options={STATUS_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
		/>
		<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr("Cari nama kampanye…")}
				aria-label={tr("Cari kampanye")}
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
		<p class="mt-3 text-sm text-muted-foreground">{tr("Kampanye yang dihapus disimpan di sini. Kampanye bisa dikembalikan, atau dihapus permanen.")}</p>
	{/if}

	{#if filteredList.length === 0}
		<div class="app-card mt-4 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{showArchive
					? tr('Arsip kosong.')
					: query || statusFilter !== 'all'
						? tr('Tidak ada kampanye yang cocok.')
						: 'Belum ada kampanye. Klik tr("Kampanye Baru") untuk memulai.'}
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
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badgeStyle('accent')}>{c.entry_mode === 'kilat' ? 'KILAT' : 'DIBANTU AI'}</span>
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badgeStyle(c.stats.published_media > 0 ? 'success' : (STAGE_TONE[c.taut_status ?? 'draft'] ?? 'muted'))}>
								{(STAGE_LABEL[c.taut_status ?? 'draft'] ?? c.taut_status ?? 'Draft').toUpperCase()}
							</span>
							{#if (c.taut_round ?? 1) > 1}
								<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badgeStyle('muted')}>{tr('Putaran')} {c.taut_round}</span>
							{/if}
						</div>
						<div class="flex items-center gap-2">
							{#if c.created_at}
								<span class="text-[11px] text-muted-foreground">{tr('Dibuat:')} {new Date(c.created_at).toLocaleDateString(i18n.locale === 'en' ? 'en-GB' : 'id-ID')}</span>
							{/if}
							{#if !showArchive}
								<div class="flex gap-1">
										{#if c.taut_status === 'submitted' || c.taut_status === 'scheduled'}
											<Tooltip content={tr('Tambah Putaran')}>
												<button
													class="rounded-lg p-1.5 hover:bg-secondary"
													type="button"
													aria-label={tr('Tambah Putaran')}
													disabled={rowBusyId === c.id}
													onclick={() => void addRound(c)}
												>
													{#if rowBusyId === c.id}<LoaderCircle size={14} class="animate-spin" />{:else}<Plus size={14} />{/if}
												</button>
											</Tooltip>
										{/if}
										<Tooltip content={tr("Berita acara & share")}>
											<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Berita acara & share")} onclick={() => void openShare(c.id)}>
												<Share2 size={14} />
											</button>
										</Tooltip>
										<Tooltip content={tr("Hapus ke arsip")}>
											<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Hapus ke arsip")} disabled={rowBusyId === c.id} onclick={() => void archiveCampaign(c.id)}>
												<Archive size={14} />
											</button>
										</Tooltip>
								</div>
							{/if}
						</div>
					</div>

					<p class="mt-3 truncate font-semibold">{c.name}</p>
					<p class="mt-1 text-xs text-muted-foreground">
						{c.media_type
							? `Tipe media: ${c.media_type} · Strategi anggaran: ${c.budget_strategy ?? '-'} · Mulai ${c.starts_at ? new Date(c.starts_at).toLocaleDateString('id-ID') : '-'}`
							: tr('Kampanye distribusi konten PR otomatis.')}
					</p>

					<div class="mt-3 flex flex-wrap items-end justify-between gap-3 border-t pt-3">
						<div>
							<p class="text-[10px] font-semibold tracking-wide text-muted-foreground">ANGGARAN TERSERAP</p>
							<p class="text-sm font-semibold tabular-nums">{tcToRp(c.stats.tc_used, rateIdr)}</p>
						</div>
						<div>
							<p class="text-[10px] font-semibold tracking-wide text-muted-foreground">PROGRES MEDIA</p>
							<p class="text-sm font-semibold tabular-nums">
								<span style="color: {c.stats.published_media > 0 ? 'var(--success)' : 'inherit'};">{c.stats.published_media} {tr("Terbit")}</span>
								{' / '}{totalMedia} {tr("Total")} </p>
						</div>
						<button class="ml-auto inline-flex items-center gap-1 text-sm font-medium" type="button" style="color: var(--primary);" onclick={() => handleContinue(c)}>{tr("Buka Kampanye")}<ArrowRight size={14} />
						</button>
					</div>

					{#if showArchive}
						<div class="mt-3 flex gap-2 border-t pt-3">
							<button class="app-btn app-btn-primary flex-1" type="button" disabled={rowBusyId === c.id} onclick={() => void archiveAction(c.id, 'restore')}>
								<Clock size={14} /> Pulihkan
							</button>
							<button class="app-btn app-btn-soft" type="button" aria-label={tr("Hapus permanen")} disabled={rowBusyId === c.id} onclick={() => void archiveAction(c.id, 'purge')}>
								<Trash size={14} />
							</button>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		{#if totalPages > 1 || loading}
			<div class="mt-4 flex flex-wrap items-center justify-between gap-2">
				<p class="text-xs text-muted-foreground">{loading ? tr('Memuat…') : `${total} kampanye`}</p>
				<div class="flex items-center gap-2">
					<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label={tr("Halaman sebelumnya")} disabled={pageNum <= 1 || loading} onclick={() => { pageNum = Math.max(1, pageNum - 1); void loadList(); }}>
						‹
					</button>
					<span class="text-xs text-muted-foreground">{tr("Halaman")} {pageNum} {tr("dari")} {totalPages}</span>
					<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label={tr("Halaman berikutnya")} disabled={pageNum >= totalPages || loading} onclick={() => { pageNum = Math.min(totalPages, pageNum + 1); void loadList(); }}>
						›
					</button>
				</div>
			</div>
		{/if}
	{/if}
</section>

{#if shareFor}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
		style="background-color: rgba(15, 23, 42, 0.45);"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) closeShare(); }}
	>
		<dialog open class="shareModal app-card" aria-label={tr("Berita Acara Kampanye")}>
			<div class="flex items-start justify-between gap-4">
				<div>
					<h2 class="shareTitle">{tr("Berita Acara Kampanye")}</h2>
					<p class="shareSubtitle">{tr("Detail kampanye & tautan share publik untuk klien.")}</p>
				</div>
				<button class="shareClose" type="button" aria-label={tr("Tutup")} onclick={closeShare}><X size={18} /></button>
			</div>

			<div class="shareDivider"></div>

			<div class="shareNameRow">
				<div class="shareNameCell">
					<p class="shareLabel">{tr("Nama kampanye")}</p>
					<p class="shareName">{shareFor.name}</p>
				</div>
				<div class="shareStatusCell">
					<p class="shareLabel">Status</p>
					<span class="shareBadge" style={badgeStyle('accent')}>
						{STAGE_LABEL[shareFor.taut_status ?? 'draft'] ?? shareFor.taut_status}
					</span>
				</div>
			</div>

			<div class="shareMeta">
				<div>
					<p class="shareLabel">{tr("Jenis media")}</p>
					<p class="shareMetaValue">{shareFor.media_type ?? tr('Belum dipilih')}</p>
				</div>
				<div>
					<p class="shareLabel">Mulai</p>
					<p class="shareMetaValue">{shareFor.starts_at ? new Date(shareFor.starts_at).toLocaleDateString('id-ID') : '-'}</p>
				</div>
			</div>

			<div class="shareStats">
				<div>
					<span class="shareStatsLabel">Artikel</span>
					<span class="shareStatsValue">{shareFor.stats.articles ?? 0}</span>
				</div>
				<div>
					<span class="shareStatsLabel">{tr("Media terbit")}</span>
					<span class="shareStatsValue">{shareFor.stats.published_media ?? 0}</span>
				</div>
				<div>
					<span class="shareStatsLabel">Terpakai</span>
					<span class="shareStatsValue">{tcToRp(shareFor.stats.tc_used ?? 0, rateIdr)}</span>
				</div>
			</div>

			{#if shareError}<p class="shareSection text-xs" style="color: var(--destructive);">{shareError}</p>{/if}

			<div class="shareSection">
				<p class="shareLabel shareLinkLabel">Tautan share publik</p>
				{#if shareToken}
					<div class="mt-2 flex items-center gap-2">
						<input class="app-input shareLinkInput" readonly value={shareUrl} onclick={(e) => e.currentTarget.select()} />
						<button class="app-btn app-btn-primary shareCopyBtn" type="button" onclick={() => void copyShare()}>
							{#if copied}<Check size={15} />{/if}
							{copied ? tr('Tersalin') : tr('Salin')}
						</button>
					</div>
				{:else}
					<button class="app-btn app-btn-soft mt-2" type="button" disabled={shareLoading} onclick={() => void openShare(shareFor!.id)}>
						{#if shareLoading}<LoaderCircle size={14} class="animate-spin" />{/if} {tr("Buat tautan share publik")}
					</button>
				{/if}
			</div>
		</dialog>
	</div>
{/if}

{#if createOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-3xl p-5" aria-label={tr("Buat Kampanye")}>
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-lg font-semibold">{tr("Buat Kampanye")}</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup")} disabled={busy} onclick={closeCreate}><X size={16} /></button>
			</div>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Pilih cara kamu membuat kampanye distribusi konten.")}</p>

			<label class="mt-3 block">
				<span class="text-sm font-medium">{tr("Nama Kampanye")}<span style="color: var(--destructive);">*</span></span>
				<input
					class="app-input mt-1 w-full"
					placeholder={tr("Contoh: Launching Produk Q3 2026")}
					value={createName}
					oninput={(e) => { createName = (e.currentTarget as HTMLInputElement).value; if (error) error = ''; }}
				/>
			</label>

			{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

			<div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
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
						The Writer tanpa AI: siapkan media & unggah artikel tulisanmu sendiri.
					</span>
				</button>
				<button
					type="button"
					class="rounded-xl border p-3 text-left transition-colors"
					style="border-color: {createMode === 'kilat' ? 'var(--primary)' : 'var(--border)'}; background-color: {createMode === 'kilat' ? 'var(--accent)' : 'transparent'};"
					disabled={busy}
					onclick={() => (createMode = 'kilat')}
				>
					<span class="flex items-center gap-2">
						<Zap size={16} style="color: var(--primary);" />
						<span class="font-bold">Kampanye Kilat</span>
					</span>
					<span class="mt-1 block text-xs text-muted-foreground">{tr("Cukup URL website + anggaran. AI menyusun ringkasan & memilih medianya, lalu kamu langsung ke produksi artikel.")}</span>
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

<ProfilerGateModal
	open={profilerGateOpen}
	brandId={brandId}
	{brandUrl}
	onClose={() => (profilerGateOpen = false)}
/>

<style>
	/* Tombol Profiler: badge AI saja (tanpa ikon garis generik). */
	.profilerAiBadge {
		display: inline-flex;
		align-items: center;
		background: var(--primary, #2b67ec);
		color: #fff;
		font-size: 10.5px;
		font-weight: 700;
		letter-spacing: 0.02em;
		padding: 2px 8px;
		border-radius: 999px;
		margin-left: 2px;
	}

	/* Dialog "Berita Acara": modal putih bersih di atas overlay gelap transparan
	   (dashboard tetap terlihat di belakang). Shadow tunggal — dialog adalah
	   satu-satunya elemen yang terangkat di atas halaman. */
	.shareModal {
		width: 100%;
		max-width: 640px;
		max-height: 90vh;
		overflow-y: auto;
		padding: 24px;
		border-radius: 16px;
		box-shadow: 0 24px 64px rgba(15, 23, 42, 0.28);
		animation: shareModalIn 0.16s cubic-bezier(0.16, 1, 0.3, 1);
	}
	@keyframes shareModalIn {
		from { opacity: 0; transform: translateY(6px) scale(0.985); }
		to { opacity: 1; transform: none; }
	}
	@media (prefers-reduced-motion: reduce) {
		.shareModal { animation: none; }
	}
	.shareTitle {
		font-size: 19px;
		font-weight: 750;
		letter-spacing: -0.01em;
		color: var(--foreground);
	}
	.shareSubtitle {
		margin-top: 2px;
		font-size: 13.5px;
		color: var(--muted-foreground);
	}
	.shareClose {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 36px;
		height: 36px;
		border: none;
		border-radius: 10px;
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background-color 140ms ease, color 140ms ease;
	}
	.shareClose:hover {
		background: var(--secondary);
		color: var(--foreground);
	}
	.shareDivider {
		height: 1px;
		margin: 18px 0;
		background: var(--border);
	}
	.shareSection {
		margin-top: 18px;
	}
	.shareLabel {
		font-size: 12.5px;
		color: var(--muted-foreground);
	}
	/* Nama kampanye + status: pola sama dengan baris Jenis media | Mulai di
	   bawahnya — dua sel berlabel dengan pemisah vertikal tipis.
	   Kedua baris memakai grid 2 kolom identik (1fr 1fr) supaya label Status
	   sejajar persis dengan label Mulai, dan nama kampanye sejajar dengan
	   Jenis media. Sebelumnya flex + flex:0 0 auto membuat Status menempel
	   setelah nama sehingga kolomnya melenceng. */
	.shareNameRow {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}
	.shareNameCell {
		min-width: 0;
		padding-right: 18px;
	}
	.shareStatusCell {
		min-width: 0;
		border-left: 1px solid var(--border);
		padding-left: 18px;
	}
	.shareName {
		margin-top: 3px;
		font-size: 15.5px;
		line-height: 24px;
		font-weight: 650;
		color: var(--foreground);
		word-break: break-word;
	}
	/* Badge setinggi kotak teks nama (24px) supaya garis nilainya persis
	   sejajar dengan "tes12233" di sel kiri dan nilai di baris bawah. */
	.shareBadge {
		display: inline-flex;
		align-items: center;
		margin-top: 3px;
		height: 24px;
		border-radius: 999px;
		padding: 0 12px;
		font-size: 11.5px;
		font-weight: 650;
		line-height: 1;
		white-space: nowrap;
	}
	.shareMeta {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		margin-top: 18px;
	}
	.shareMeta > div {
		min-width: 0;
	}
	.shareMeta > div:first-child {
		padding-right: 18px;
	}
	.shareMeta > div + div {
		border-left: 1px solid var(--border);
		padding-left: 18px;
	}
	.shareMetaValue {
		margin-top: 3px;
		font-size: 15px;
		line-height: 24px;
		font-weight: 600;
		color: var(--foreground);
		word-break: break-word;
	}
	/* Panel statistik: satu permukaan biru lembut, tiga kolom berpemisah garis
	   — angka dari data laporan yang sama seperti sebelumnya. */
	.shareStats {
		display: flex;
		margin-top: 20px;
		padding: 18px 20px;
		background: #f1f6fd;
		border-radius: 12px;
	}
	.shareStats > div {
		flex: 1;
		min-width: 0;
	}
	.shareStats > div:first-child {
		padding-right: 18px;
	}
	.shareStats > div + div {
		border-left: 1px solid color-mix(in srgb, var(--primary) 14%, transparent);
		padding-left: 18px;
	}
	.shareStatsLabel {
		display: block;
		font-size: 12.5px;
		color: var(--muted-foreground);
	}
	.shareStatsValue {
		display: block;
		margin-top: 4px;
		font-size: 24px;
		font-weight: 650;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.shareLinkLabel {
		font-weight: 600;
	}
	.shareLinkInput {
		flex: 1;
		min-width: 0;
		height: 44px;
	}
	.shareCopyBtn {
		flex: 0 0 auto;
		height: 44px;
		min-width: 96px;
	}
	@media (max-width: 640px) {
		.shareModal {
			padding: 18px 16px;
		}
		.shareNameRow {
			grid-template-columns: minmax(0, 1fr);
			gap: 14px;
		}
		.shareNameCell {
			padding-right: 0;
		}
		.shareStatusCell {
			border-left: none;
			border-top: 1px solid var(--border);
			padding-left: 0;
			padding-top: 12px;
		}
		.shareMeta {
			grid-template-columns: minmax(0, 1fr);
			gap: 14px;
		}
		.shareMeta > div:first-child {
			padding-right: 0;
		}
		.shareMeta > div + div {
			border-left: none;
			border-top: 1px solid var(--border);
			padding-left: 0;
			padding-top: 12px;
		}
	}
	@media (max-width: 480px) {
		.shareStats {
			flex-direction: column;
			gap: 12px;
			padding: 16px;
		}
		.shareStats > div:first-child {
			padding-right: 0;
		}
		.shareStats > div + div {
			border-left: none;
			border-top: 1px solid color-mix(in srgb, var(--primary) 14%, transparent);
			padding-left: 0;
			padding-top: 12px;
		}
		.shareStatsValue {
			font-size: 20px;
		}
	}
</style>
