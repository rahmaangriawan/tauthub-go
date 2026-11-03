<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Knowledge Base — port knowledge-view.tsx: collections dengan statistik
	// sumber, buka/tutup daftar sumber (paginasi 50), salin konten, upload file,
	// crawl/re-crawl sitemap dengan polling status, hapus collection/source.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Check, ChevronLeft, ChevronRight, Copy, FileText, List, Plus, Trash2, Upload, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Collection = { id: string; name: string; description?: string | null; created_at: string; source_count?: number };
	type SourceItem = {
		id: string; collection_id: string; title: string; source_type: string;
		status: 'processing' | 'active' | 'failed' | 'deprecated' | 'archived';
		error_message?: string | null; created_at: string; char_length?: number; token_estimate?: number;
	};

	let collections = $state<Collection[]>([]);
	let loading = $state(true);
	let error = $state('');
	let openCollection = $state<string | null>(null);
	let sources = $state<Record<string, SourceItem[]>>({});
	let sourcePage = $state<Record<string, number>>({});
	let sourceTotal = $state<Record<string, number>>({});

	// Modal: collection baru
	let showDialog = $state(false);
	let newName = $state('');
	let newDesc = $state('');
	let creating = $state(false);

	// Modal: upload
	let showUpload = $state(false);
	let uploadTitle = $state('');
	let uploadFile = $state<File | null>(null);
	let uploadTargetCol = $state('');
	let uploading = $state(false);

	// Modal: crawl sitemap
	let showSitemap = $state(false);
	let sitemapUrl = $state('');
	let sitemapCol = $state('');
	let sitemapLoading = $state(false);
	let crawlRunId = $state<string | null>(null);
	let crawlStatus = $state<'idle' | 'pending' | 'running' | 'completed' | 'failed'>('idle');
	let crawlDetail = $state<{ total: number | null; processed: number | null; failed: string[]; error: string | null }>({
		total: null, processed: null, failed: [], error: null
	});

	let copied = $state<string | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadCollections();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Polling status crawl sitemap tiap 2 dtk sampai selesai/gagal.
	$effect(() => {
		if (!crawlRunId) return;
		const id = window.setInterval(async () => {
			try {
				const res = await apiRequest(`/v1/brands/${brandId}/knowledge/sitemap?run_id=${encodeURIComponent(crawlRunId!)}`);
				if (!res.ok) return;
				const data = await res.json();
				crawlStatus = data.status;
				crawlDetail = {
					total: data.total_urls ?? null,
					processed: data.processed ?? null,
					failed: data.failed_urls ?? [],
					error: data.error ?? null
				};
				if (data.status === 'completed' || data.status === 'failed') window.clearInterval(id);
			} catch {
				/* kesalahan polling sesaat diabaikan */
			}
		}, 2000);
		return () => window.clearInterval(id);
	});

	async function loadCollections() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections?limit=200`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat collections')));
			const data = (await res.json()) as Collection[];
			collections = data;
			void Promise.allSettled(data.map((c) => fetchSources(c.id)));
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat data knowledge');
		} finally {
			loading = false;
		}
	}

	async function fetchSources(collectionId: string, pageIdx = 0) {
		const limit = 50;
		const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections/${collectionId}/sources?limit=${limit}&offset=${pageIdx * limit}`);
		if (res.ok) {
			const data = (await res.json()) as { sources: SourceItem[]; total: number };
			sources = { ...sources, [collectionId]: data.sources };
			sourcePage = { ...sourcePage, [collectionId]: pageIdx };
			sourceTotal = { ...sourceTotal, [collectionId]: data.total };
		}
	}

	async function createCollection() {
		if (!newName.trim()) return;
		creating = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections`, {
				method: 'POST',
				body: JSON.stringify({ name: newName.trim(), description: newDesc.trim() || null })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat collection')));
			showDialog = false;
			newName = '';
			newDesc = '';
			void loadCollections();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal membuat collection');
		} finally {
			creating = false;
		}
	}

	async function deleteCollection(id: string) {
		const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections/${id}`, { method: 'DELETE' });
		if (res.ok) {
			collections = collections.filter((c) => c.id !== id);
			const nextSources = { ...sources };
			delete nextSources[id];
			sources = nextSources;
			if (openCollection === id) openCollection = null;
		} else {
			error = await readApiError(res, tr('Gagal menghapus collection'));
		}
	}

	async function deleteSource(collectionId: string, sourceId: string) {
		const res = await apiRequest(`/v1/brands/${brandId}/knowledge/sources/${sourceId}`, { method: 'DELETE' });
		if (res.ok) {
			sources = { ...sources, [collectionId]: (sources[collectionId] ?? []).filter((s) => s.id !== sourceId) };
		} else {
			error = await readApiError(res, tr('Gagal menghapus source'));
		}
	}

	async function uploadManual() {
		if (!uploadTitle.trim() || !uploadFile) return;
		uploading = true;
		try {
			const content = await uploadFile.text();
			const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections/${uploadTargetCol}/sources`, {
				method: 'POST',
				body: JSON.stringify({
					title: uploadTitle.trim(), content,
					source_type: uploadFile.type || 'manual', original_filename: uploadFile.name
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Upload gagal'));
			const target = uploadTargetCol;
			showUpload = false;
			uploadTitle = '';
			uploadFile = null;
			uploadTargetCol = '';
			void fetchSources(target);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Upload gagal';
		} finally {
			uploading = false;
		}
	}

	async function submitSitemap() {
		if (!sitemapUrl.trim() || !sitemapCol) return;
		sitemapLoading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/knowledge/sitemap`, {
				method: 'POST',
				body: JSON.stringify({ collection_id: sitemapCol, sitemap_url: sitemapUrl.trim(), max_pages: 50 })
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Crawl gagal'));
			const { run_id } = (await res.json()) as { run_id: string };
			crawlRunId = run_id;
			crawlStatus = 'pending';
			crawlDetail = { total: null, processed: null, failed: [], error: null };
			sitemapUrl = '';
			sitemapCol = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Crawl gagal';
		} finally {
			sitemapLoading = false;
		}
	}

	async function copyMarkdown(source: SourceItem) {
		const res = await apiRequest(`/v1/brands/${brandId}/knowledge/sources/${source.id}/content`);
		if (!res.ok) {
			error = tr('Gagal ambil konten');
			return;
		}
		const data = (await res.json()) as { content?: string };
		await navigator.clipboard.writeText(data.content ?? '');
		copied = source.id;
		window.setTimeout(() => (copied = null), 1800);
	}

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';

	const statusStyle = (status: string) =>
		status === 'active'
			? 'background-color: var(--success-soft); color: var(--success);'
			: status === 'processing'
				? 'background-color: var(--warning-soft); color: var(--warning);'
				: 'background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);';
</script>

<PageTitle title="Knowledge Base" />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="page-title">Knowledge Base</h1>
			<p class="page-desc">{tr("Kelola konten, artikel lama, dan sumber eksternal yang dipakai Tauthub saat menjawab atau menulis.")}</p>
		</div>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (showDialog = true)}><Plus size={14} /> Collection</button>
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<div class="mt-4 grid gap-3 md:grid-cols-2">
			{#each [0, 1, 2, 3] as i (i)}
				<div class="app-card p-4">
					<div class="h-2.5 w-1/3 animate-pulse rounded" style="background-color: var(--muted);"></div>
					<div class="mt-2 h-3.5 w-2/3 animate-pulse rounded" style="background-color: var(--muted);"></div>
					<div class="mt-2 h-2.5 w-1/4 animate-pulse rounded" style="background-color: var(--muted);"></div>
				</div>
			{/each}
		</div>
	{:else}
		<div class="mt-4 grid gap-4 lg:grid-cols-3">
			<div class="grid content-start gap-3 lg:col-span-2">
				{#each collections as col (col.id)}
					{@const colSources = sources[col.id] ?? []}
					{@const totalChars = colSources.reduce((acc, s) => acc + (s.char_length ?? 0), 0)}
					{@const estTokens = colSources.reduce((acc, s) => acc + (s.token_estimate ?? 0), 0)}
					<section class="app-card p-4">
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0 flex-1">
								<h2 class="truncate font-semibold">{col.name}</h2>
								<p class="mt-0.5 text-xs text-muted-foreground">{col.description ?? 'Tanpa deskripsi'} · {fmtDate(col.created_at)}</p>
								<p class="mt-1 text-xs font-medium">{sourceTotal[col.id] ?? colSources.length} sumber</p>
								{#if estTokens > 0}
									<p class="mt-0.5 text-[11px] text-muted-foreground">
										Estimasi token: {estTokens.toLocaleString('id-ID')} · Karakter: {totalChars.toLocaleString('id-ID')}
									</p>
								{/if}
							</div>
							<div class="flex shrink-0 gap-1">
								<button class="rounded-lg p-1.5 transition-colors hover:bg-secondary" title="Toggle sumber" aria-label="Toggle sumber" onclick={() => (openCollection = openCollection === col.id ? null : col.id)}>
									<FileText size={17} />
								</button>
								<button class="rounded-lg p-1.5 transition-colors hover:bg-secondary" title="Upload file / paste teks" aria-label="Upload sumber" onclick={() => { uploadTargetCol = col.id; showUpload = true; }}>
									<Upload size={16} />
								</button>
								<button class="rounded-lg p-1.5 transition-colors hover:bg-secondary" title="Crawl sitemap" aria-label="Crawl sitemap" onclick={() => { sitemapCol = col.id; showSitemap = true; }}>
									<List size={16} />
								</button>
								<button class="rounded-lg p-1.5 transition-colors hover:bg-secondary" title={tr("Hapus collection")} aria-label={tr("Hapus collection")} onclick={() => void deleteCollection(col.id)}>
									<Trash2 size={15} style="color: var(--destructive);" />
								</button>
							</div>
						</div>

						{#if openCollection === col.id}
							<div class="mt-3.5">
								{#if colSources.length === 0}
									<p class="text-xs text-muted-foreground">{tr("Belum ada sumber di collection ini.")}</p>
								{:else}
									<ul class="grid gap-2">
										{#each colSources as s (s.id)}
											<li>
												<div class="flex items-start justify-between gap-3 rounded-lg border px-3 py-2">
													<div class="min-w-0 flex-1">
														<p class="truncate text-sm font-medium">{s.title}</p>
														<p class="mt-0.5 flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
															<span>Type: {s.source_type}{s.char_length != null ? ` · ${s.char_length.toLocaleString('id-ID')} karakter` : ''}{s.token_estimate != null ? ` · ${s.token_estimate.toLocaleString('id-ID')} token` : ''}</span>
															<span class="rounded-full px-2 py-0.5 font-semibold" style={statusStyle(s.status)}>{s.status}</span>
														</p>
													</div>
													<div class="flex shrink-0 gap-1">
														<button class="rounded-lg p-1.5 transition-colors hover:bg-secondary" title="Salin konten teks" aria-label="Salin konten" type="button" onclick={() => void copyMarkdown(s)}>
															{#if copied === s.id}<Check size={15} />{:else}<Copy size={15} />{/if}
														</button>
														<button class="rounded-lg p-1.5 transition-colors hover:bg-secondary" title={tr("Hapus source")} aria-label={tr("Hapus sumber")} type="button" onclick={() => void deleteSource(col.id, s.id)}>
															<Trash2 size={14} style="color: var(--destructive);" />
														</button>
													</div>
												</div>
											</li>
										{/each}
									</ul>
								{/if}
								{#if colSources.length > 0}
									<div class="mt-3.5 flex items-center justify-center gap-3">
										<button
											class="app-btn app-btn-soft !min-h-8 !text-xs"
											type="button"
											disabled={!(sourcePage[col.id] ?? 0)}
											onclick={() => void fetchSources(col.id, (sourcePage[col.id] ?? 0) - 1)}
										><ChevronLeft size={13} /> Prev</button>
										<span class="text-xs text-muted-foreground">
											{Math.min((sourcePage[col.id] ?? 0) * 50 + 1, sourceTotal[col.id] ?? 0)}–{Math.min(((sourcePage[col.id] ?? 0) + 1) * 50, sourceTotal[col.id] ?? 0)} {tr("dari")} {sourceTotal[col.id] ?? 0}
										</span>
										<button
											class="app-btn app-btn-soft !min-h-8 !text-xs"
											type="button"
											disabled={colSources.length < 50 || ((sourcePage[col.id] ?? 0) + 1) * 50 >= (sourceTotal[col.id] ?? 0)}
											onclick={() => void fetchSources(col.id, (sourcePage[col.id] ?? 0) + 1)}
										>Next <ChevronRight size={13} /></button>
									</div>
								{/if}
							</div>
						{/if}
					</section>
				{/each}

				{#if collections.length === 0}
					<div class="app-card flex flex-col items-center px-6 py-10 text-center">
						<List size={28} class="text-muted-foreground" />
						<p class="mt-2 text-sm text-muted-foreground">{tr("Belum ada collection. Bikin yang pertama di atas.")}</p>
					</div>
				{/if}
			</div>

			<aside class="grid content-start gap-3">
				<section class="app-card p-4">
					<h2 class="font-semibold">Quick actions</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">Bantu mulai dengan cepat.</p>
					<div class="mt-3.5 grid gap-2.5">
						<button class="app-btn app-btn-soft !justify-start" type="button" onclick={() => (showDialog = true)}><Plus size={16} /> Collection baru</button>
						<button
							class="app-btn app-btn-soft !justify-start"
							type="button"
							disabled={collections.length === 0}
							onclick={() => { showUpload = true; uploadTargetCol = ''; }}
						><Upload size={16} /> Upload teks</button>
						{#if collections.length === 0}
							<p class="text-xs text-muted-foreground">{tr("Buat collection dulu untuk mulai upload atau crawl.")}</p>
						{/if}
					</div>
				</section>
			</aside>
		</div>
	{/if}
</section>

<!-- Modal: Collection baru -->
{#if showDialog}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showDialog = false; }}>
		<div class="app-card w-full max-w-md p-5" role="dialog" aria-modal="true" aria-label="New Collection">
			<h3 class="font-semibold">New Collection</h3>
			<input class="app-input mt-3 w-full" placeholder={tr("Nama collection")} bind:value={newName} />
			<textarea class="app-input mt-2 w-full" placeholder={tr("Deskripsi (opsional)")} rows={3} bind:value={newDesc}></textarea>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (showDialog = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={creating || !newName.trim()} onclick={() => void createCollection()}>
					{creating ? 'Membuat…' : 'Buat'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Modal: Upload teks -->
{#if showUpload}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showUpload = false; }}>
		<div class="app-card w-full max-w-md p-5" role="dialog" aria-modal="true" aria-label="Upload konten">
			<h3 class="font-semibold">Upload konten</h3>
			<label class="mt-3 block text-xs">
				<span class="text-muted-foreground">Collection {uploadTargetCol ? `(pilih "${collections.find((c) => c.id === uploadTargetCol)?.name ?? uploadTargetCol}")` : 'semua collections'}</span>
				{#if !uploadTargetCol}
					<div class="mt-1">
						<Select
							bind:value={uploadTargetCol}
							ariaLabel="Collection"
							placeholder={tr("Pilih collection")}
							minWidth={260}
							options={[
								{ value: '', label: tr('Pilih collection') },
								...collections.map((c) => ({ value: c.id, label: c.name }))
							]}
						/>
					</div>
				{/if}
			</label>
			<input class="app-input mt-2 w-full" placeholder={tr("Judul")} bind:value={uploadTitle} />
			<div class="mt-2">
				<input
					id="kb-file-upload"
					type="file"
					accept=".txt,.md,.html,.json"
					class="hidden"
					onchange={(e) => (uploadFile = (e.currentTarget as HTMLInputElement).files?.[0] ?? null)}
				/>
				<label for="kb-file-upload" class="flex cursor-pointer flex-col items-center gap-1 rounded-lg border border-dashed px-4 py-5 text-center text-xs text-muted-foreground">
					{#if uploadFile}
						<span class="flex items-center gap-2">
							<span class="font-medium text-foreground">{uploadFile.name}</span>
							<span>{(uploadFile.size / 1024).toFixed(1)} KB</span>
							<button class="rounded p-0.5 hover:bg-secondary" type="button" title={tr("Hapus file")} onclick={(e) => { e.preventDefault(); uploadFile = null; }}><X size={14} /></button>
						</span>
					{:else}
						<Upload size={16} />
						<span>{tr("Pilih file atau drag & drop")}</span>
						<small>.txt .md .html .json</small>
					{/if}
				</label>
			</div>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => { showUpload = false; uploadTitle = ''; uploadFile = null; uploadTargetCol = ''; }}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={uploading || !uploadTitle.trim() || !uploadFile} onclick={() => void uploadManual()}>
					{uploading ? 'Mengunggah…' : 'Upload'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Modal: Crawl sitemap -->
{#if showSitemap}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget && crawlStatus === 'idle') showSitemap = false; }}>
		<div class="app-card w-full max-w-md p-5" role="dialog" aria-modal="true" aria-label="Crawl sitemap">
			<div class="flex items-center justify-between gap-3">
				<h3 class="font-semibold">Crawl / Re-crawl sitemap</h3>
				<button
					class="rounded-lg p-1 transition-colors hover:bg-secondary"
					type="button"
					aria-label={tr("Tutup")}
					onclick={() => { showSitemap = false; crawlStatus = 'idle'; crawlRunId = null; void loadCollections(); }}
				><X size={18} /></button>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">{tr("* Re-crawl akan otomatis menandai sumber lama yang sudah tidak ada di sitemap ini jadi deprecated dan menambahkan yang baru.")}</p>

			{#if crawlStatus === 'idle'}
				<div class="mt-3">
					<Select
						bind:value={sitemapCol}
						ariaLabel="Collection"
						placeholder={tr("Pilih collection")}
						minWidth={260}
						options={[
							{ value: '', label: tr('Pilih collection') },
							...collections.map((c) => ({ value: c.id, label: c.name }))
						]}
					/>
				</div>
				<input class="app-input mt-2 w-full" placeholder="https://example.com/sitemap.xml" bind:value={sitemapUrl} />
				<div class="mt-4 flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => { showSitemap = false; sitemapUrl = ''; sitemapCol = ''; }}>{tr("Batal")}</button>
					<button class="app-btn app-btn-primary" type="button" disabled={sitemapLoading || !sitemapUrl.trim() || !sitemapCol} onclick={() => void submitSitemap()}>
						{sitemapLoading ? 'Menyiapkan…' : 'Crawl'}
					</button>
				</div>
			{:else}
				<div class="mt-3 grid gap-3">
					<p class="text-sm">
						Status: <strong>
							{crawlStatus === 'pending' ? tr('Menunggu queue...') : crawlStatus === 'running' ? 'Sedang mengekstrak sitemap...' : crawlStatus === 'completed' ? 'Selesai!' : tr('Gagal')}
						</strong>
					</p>
					{#if crawlStatus === 'running' || crawlStatus === 'completed'}
						<p class="text-xs text-muted-foreground">Progress: {crawlDetail.processed ?? 0} / {crawlDetail.total ?? '?'} URL diproses</p>
					{/if}
					{#if crawlStatus === 'failed' && crawlDetail.error}
						<p class="text-xs" style="color: var(--destructive);">Error: {crawlDetail.error}</p>
					{/if}
					{#if crawlDetail.failed.length > 0}
						<div>
							<p class="text-xs font-semibold" style="color: var(--warning);">{crawlDetail.failed.length} URL gagal diproses:</p>
							<ul class="mt-1 max-h-24 overflow-y-auto pl-4 text-[11px] text-muted-foreground">
								{#each crawlDetail.failed as u (u)}<li>{u}</li>{/each}
							</ul>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}
