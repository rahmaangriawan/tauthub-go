<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Error monitor platform — port app/admin/errors/page.tsx.
	// Agregasi 5 kategori error (batch variant, publikasi, job distribusi, RSS, queue generic)
	// dengan pencarian server-side debounce 300ms.
	import { onMount } from 'svelte';
	import { TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type PublicationError = {
		id: string;
		status: string;
		brand_name: string | null;
		brand_public_id: string | null;
		media_name: string | null;
		document_title: string | null;
		campaign_name: string | null;
		error_code: string | null;
		error_message: string | null;
		http_status: number | null;
		updated_at: string;
	};
	type JobError = PublicationError & { publication_id: string | null; attempts: number };
	type RssError = {
		id: string;
		name: string;
		install_status: string;
		install_error: string | null;
		partner_name: string | null;
		media_name: string | null;
		updated_at: string;
	};
	type QueueError = {
		id: string;
		kind: string;
		status: string;
		last_error: string | null;
		attempts: number;
		updated_at: string;
	};
	type VariantBatchError = {
		id: string;
		mode: string;
		status: string;
		brand_name: string | null;
		brand_public_id: string | null;
		total: number;
		failed: number;
		failed_media: string | null;
		updated_at: string;
	};
	// Peringatan non-fatal: pengiriman sukses tapi ada yang tidak sempurna,
	// mis. gambar artikel gagal diunggah ke media mitra.
	type WarningRow = {
		id: string;
		publication_id: string | null;
		status: string;
		brand_name: string | null;
		media_name: string | null;
		document_title: string | null;
		warning_message: string | null;
		updated_at: string;
	};

	const ADAPTER_CODE_LABELS: Record<string, string> = $derived({
		adapter_unreachable: 'Adapter tidak terjangkau',
		adapter_rejected: 'Adapter menolak payload',
		adapter_error: 'Error adapter (5xx)',
		adapter_not_accepted: 'Adapter tidak accept',
		partner_not_ready: 'Partner belum siap',
		no_connection: tr('Tidak ada koneksi'),
		media_missing: 'Media tidak ditemukan',
		document_missing: 'Artikel tidak ditemukan',
		version_missing: 'Versi konten hilang',
		connection_secret_missing: 'Koneksi tidak aktif',
		publish_failed: tr('Publikasi gagal')
	});

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let search = $state('');
	let publications = $state<PublicationError[]>([]);
	let jobs = $state<JobError[]>([]);
	let rss = $state<RssError[]>([]);
	let queue = $state<QueueError[]>([]);
	let variantBatches = $state<VariantBatchError[]>([]);
	let warnings = $state<WarningRow[]>([]);
	let offsets = $state({ variant: 0, publication: 0, job: 0, rss: 0, queue: 0, warning: 0 });

	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load('');
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			clearTimeout(debounceTimer);
		};
	});

	function onSearchInput(value: string) {
		search = value;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			offsets = { variant: 0, publication: 0, job: 0, rss: 0, queue: 0, warning: 0 };
			void load(value.trim());
		}, 300);
	}

	async function load(q: string) {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/errors${q ? `?q=${encodeURIComponent(q)}` : ''}`);
			if (!res.ok) {
				const detail = await readApiError(res, 'Permintaan platform gagal.');
				error = detail === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : detail;
				return;
			}
			const body = await res.json();
			publications = body.publications ?? [];
			jobs = body.jobs ?? [];
			rss = body.rss ?? [];
			queue = body.queue ?? [];
			variantBatches = body.variant_batches ?? [];
			warnings = body.warnings ?? [];
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			loading = false;
		}
	}

	function clearSearch() {
		search = '';
		clearTimeout(debounceTimer);
		offsets = { variant: 0, publication: 0, job: 0, rss: 0, queue: 0, warning: 0 };
		void load('');
	}

	function slice<T>(rows: T[], key: keyof typeof offsets): T[] {
		return rows.slice(offsets[key], offsets[key] + PAGE_SIZE);
	}

	function setOffset(key: keyof typeof offsets, value: number) {
		offsets = { ...offsets, [key]: value };
	}

	const total = $derived(
		publications.length + jobs.length + rss.length + queue.length + variantBatches.length
	);
</script>

<section>
	<PageHeader
		title="Error Reports"
		description={tr("Agregasi error distribusi, queue, dan verifikasi RSS. Klien/mitra tidak melihat halaman ini — Tauthub yang bertindak.")}
	>
		<Badge tone={total > 0 ? 'danger' : 'success'} label="{total} error" />
	</PageHeader>

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<input
			class="app-input w-full max-w-md"
			placeholder={tr("Cari brand, media, artikel, atau pesan error…")}
			aria-label={tr("Cari error")}
			value={search}
			oninput={(e) => onSearchInput((e.currentTarget as HTMLInputElement).value)}
		/>
		{#if search.trim()}
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={clearSearch}>Bersihkan pencarian</button>
		{/if}
	</div>

	<Feedback {loading} {error} loadingText="Memuat laporan error…" />

	{#if error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke laporan error.")}</p>
		</div>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-3 lg:grid-cols-6">
			{#each [
				{ label: tr('Publikasi gagal'), value: publications.length },
				{ label: 'Job distribusi gagal', value: jobs.length },
				{ label: 'Batch variant gagal', value: variantBatches.length },
				{ label: 'RSS gagal verifikasi', value: rss.length },
				{ label: 'Queue gagal', value: queue.length },
				{ label: tr('Peringatan pengiriman'), value: warnings.length }
			] as card (card.label)}
				<div class="app-card p-3">
					<p class="text-2xl font-bold" style="color: var(--destructive);">{card.value}</p>
					<p class="text-xs text-muted-foreground">{card.label}</p>
				</div>
			{/each}
		</div>

		<!-- Batch variant gagal / sebagian -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Batch variant gagal / sebagian</h2>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 980px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Brand</th>
							<th class="px-3 py-2 text-left font-medium">Mode</th>
							<th class="px-3 py-2 text-left font-medium">Varian</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Media gagal")}</th>
							<th class="px-3 py-2 text-left font-medium">Status</th>
							<th class="px-3 py-2 text-left font-medium">Waktu</th>
							<th class="px-3 py-2 text-left font-medium"></th>
						</tr>
					</thead>
					<tbody>
						{#each slice(variantBatches, 'variant') as row (row.id)}
							<tr class="border-t">
								<td class="px-3 py-2 font-medium">{row.brand_name ?? '—'}</td>
								<td class="px-3 py-2">
									<Badge tone={row.mode === 'generate' ? 'accent' : 'neutral'} label={row.mode === 'generate' ? 'Dibuatkan' : 'Parafrase'} />
								</td>
								<td class="px-3 py-2">{row.failed}/{row.total} gagal</td>
								<td class="px-3 py-2 text-xs">{row.failed_media ?? '—'}</td>
								<td class="px-3 py-2"><Badge tone={row.status === 'failed' ? 'danger' : 'warning'} label={row.status} /></td>
								<td class="px-3 py-2 text-xs text-muted-foreground">{fmtDateTime(row.updated_at)}</td>
								<td class="px-3 py-2 whitespace-nowrap">
									<a href={`/admin/variants/${row.id}`} class="text-xs font-semibold" style="color: var(--accent-foreground);">{tr("Buka detail")}</a>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !loading && variantBatches.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Tidak ada batch variant gagal.")}</p>
			{/if}
			<Pagination total={variantBatches.length} limit={PAGE_SIZE} offset={offsets.variant} onChange={(v) => setOffset('variant', v)} />
		</div>

		<!-- Publikasi gagal -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">{tr("Publikasi gagal / retry")}</h2>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 1080px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Artikel</th>
							<th class="px-3 py-2 text-left font-medium">Brand</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Media")}</th>
							<th class="px-3 py-2 text-left font-medium">Status</th>
							<th class="px-3 py-2 text-left font-medium">Error</th>
							<th class="px-3 py-2 text-left font-medium">Waktu</th>
							<th class="px-3 py-2 text-left font-medium"></th>
						</tr>
					</thead>
					<tbody>
						{#each slice(publications, 'publication') as row (row.id)}
							<tr class="border-t">
								<td class="px-3 py-2 font-medium">{row.document_title ?? row.id.slice(0, 8)}</td>
								<td class="px-3 py-2">{row.brand_name ?? '—'}</td>
								<td class="px-3 py-2">{row.media_name ?? '—'}</td>
								<td class="px-3 py-2"><Badge tone="danger" label={row.status} /></td>
								<td class="px-3 py-2">
									<div class="flex flex-wrap gap-1">
										{#if row.error_code}<Badge tone="danger" label={row.error_code} />{/if}
										{#if row.http_status}<Badge tone="warning" label="HTTP {row.http_status}" />{/if}
									</div>
									<p class="mt-1 text-xs" style="color: var(--destructive);">{row.error_message ?? '—'}</p>
									{#if row.error_code && ADAPTER_CODE_LABELS[row.error_code]}
										<p class="text-xs text-muted-foreground">{ADAPTER_CODE_LABELS[row.error_code]}</p>
									{/if}
								</td>
								<td class="px-3 py-2 text-xs text-muted-foreground">{fmtDateTime(row.updated_at)}</td>
								<td class="px-3 py-2 whitespace-nowrap">
									<a href={`/admin/publications/${row.id}`} class="text-xs font-semibold" style="color: var(--accent-foreground);">{tr("Buka detail")}</a>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !loading && publications.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Tidak ada publikasi gagal.")}</p>
			{/if}
			<Pagination total={publications.length} limit={PAGE_SIZE} offset={offsets.publication} onChange={(v) => setOffset('publication', v)} />
		</div>

		<!-- Job distribusi gagal -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Job distribusi gagal (queue)</h2>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 900px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Artikel</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Media")}</th>
							<th class="px-3 py-2 text-left font-medium">Attempts</th>
							<th class="px-3 py-2 text-left font-medium">Error</th>
							<th class="px-3 py-2 text-left font-medium">Waktu</th>
						</tr>
					</thead>
					<tbody>
						{#each slice(jobs, 'job') as row (row.id)}
							<tr class="border-t">
								<td class="px-3 py-2 font-medium">{row.document_title ?? row.id.slice(0, 8)}</td>
								<td class="px-3 py-2">{row.media_name ?? '—'}</td>
								<td class="px-3 py-2">{row.attempts}</td>
								<td class="px-3 py-2">
									<div class="flex flex-wrap gap-1">
										{#if row.error_code}<Badge tone="danger" label={row.error_code} />{/if}
										{#if row.http_status}<Badge tone="warning" label="HTTP {row.http_status}" />{/if}
									</div>
									<p class="mt-1 text-xs" style="color: var(--destructive);">{row.error_message ?? '—'}</p>
								</td>
								<td class="px-3 py-2 text-xs text-muted-foreground">{fmtDateTime(row.updated_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !loading && jobs.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Tidak ada job distribusi gagal.")}</p>
			{/if}
			<Pagination total={jobs.length} limit={PAGE_SIZE} offset={offsets.job} onChange={(v) => setOffset('job', v)} />
		</div>

		<!-- Peringatan pengiriman (non-fatal) -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">{tr('Peringatan pengiriman')}</h2>
			<p class="mt-1 text-xs text-muted-foreground">
				{tr('Artikel tetap terbit, tetapi ada bagian yang tidak sempurna — misalnya gambar gagal diunggah ke media mitra. Buka detail publikasi untuk memperbaiki gambarnya.')}
			</p>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 900px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Artikel</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Media")}</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Peringatan")}</th>
							<th class="px-3 py-2 text-left font-medium">Waktu</th>
							<th class="px-3 py-2 text-left font-medium"></th>
						</tr>
					</thead>
					<tbody>
						{#each slice(warnings, 'warning') as row (row.id)}
							<tr class="border-t">
								<td class="px-3 py-2 font-medium">{row.document_title ?? row.id.slice(0, 8)}</td>
								<td class="px-3 py-2">{row.media_name ?? '—'}</td>
								<td class="px-3 py-2 text-xs" style="color: var(--warning);">{row.warning_message ?? '—'}</td>
								<td class="px-3 py-2 text-xs text-muted-foreground">{fmtDateTime(row.updated_at)}</td>
								<td class="px-3 py-2">
									{#if row.publication_id}
										<a
											class="text-xs underline"
											style="color: var(--primary);"
											href="/admin/publications/{row.publication_id}">{tr('Perbaiki gambar')}</a>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !loading && warnings.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr('Tidak ada peringatan pengiriman.')}</p>
			{/if}
			<Pagination total={warnings.length} limit={PAGE_SIZE} offset={offsets.warning} onChange={(v) => setOffset('warning', v)} />
		</div>

		<!-- RSS gagal verifikasi -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">RSS gagal verifikasi</h2>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 900px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Feed</th>
							<th class="px-3 py-2 text-left font-medium">Partner</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Media")}</th>
							<th class="px-3 py-2 text-left font-medium">Error</th>
							<th class="px-3 py-2 text-left font-medium">Waktu</th>
						</tr>
					</thead>
					<tbody>
						{#each slice(rss, 'rss') as row (row.id)}
							<tr class="border-t">
								<td class="px-3 py-2 font-medium">{row.name}</td>
								<td class="px-3 py-2">{row.partner_name ?? '—'}</td>
								<td class="px-3 py-2">{row.media_name ?? '—'}</td>
								<td class="px-3 py-2 text-xs" style="color: var(--destructive);">{row.install_error ?? '—'}</td>
								<td class="px-3 py-2 text-xs text-muted-foreground">{fmtDateTime(row.updated_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !loading && rss.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Tidak ada RSS gagal verifikasi.")}</p>
			{/if}
			<Pagination total={rss.length} limit={PAGE_SIZE} offset={offsets.rss} onChange={(v) => setOffset('rss', v)} />
		</div>

		<!-- Queue generic -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Queue generic gagal</h2>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 760px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Kind</th>
							<th class="px-3 py-2 text-left font-medium">Attempts</th>
							<th class="px-3 py-2 text-left font-medium">Error</th>
							<th class="px-3 py-2 text-left font-medium">Waktu</th>
						</tr>
					</thead>
					<tbody>
						{#each slice(queue, 'queue') as row (row.id)}
							<tr class="border-t">
								<td class="px-3 py-2 font-medium">{row.kind}</td>
								<td class="px-3 py-2">{row.attempts}</td>
								<td class="px-3 py-2 text-xs" style="color: var(--destructive);">{row.last_error ?? '—'}</td>
								<td class="px-3 py-2 text-xs text-muted-foreground">{fmtDateTime(row.updated_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if !loading && queue.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Tidak ada queue gagal.")}</p>
			{/if}
			<Pagination total={queue.length} limit={PAGE_SIZE} offset={offsets.queue} onChange={(v) => setOffset('queue', v)} />
		</div>
	{/if}
</section>
