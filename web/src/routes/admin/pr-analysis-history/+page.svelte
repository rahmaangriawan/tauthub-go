<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Riwayat analisis PR — port app/admin/pr-analysis-history/page.tsx.
	// Analisis message consistency, sentiment/tone, dan verifikasi URL untuk
	// publikasi published lintas brand/kampanye/media + export CSV.
	import { onMount } from 'svelte';
	import { Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate } from '$lib/admin-format';

	type HistoryRow = {
		publication_id: string;
		status: string;
		published_at: string | null;
		published_url: string | null;
		document_title: string | null;
		media_name: string | null;
		media_tier: string;
		brand_name: string | null;
		brand_public_id: string | null;
		campaign_id: string | null;
		campaign_name: string | null;
		message_score: number | null;
		message_status: string | null;
		message_error: string | null;
		sentiment: string | null;
		sentiment_confidence: number | null;
		compliance_status: string | null;
		compliance_score: number | null;
		verification_error: string | null;
		verification_checked_at: string | null;
	};
	type Filters = {
		brands: Array<{ id: string; name: string; public_id: string | null }>;
		campaigns: Array<{ id: string; name: string; brand_id: string }>;
		media: Array<{ id: string; name: string; partner_id: string | null }>;
		message_statuses: string[];
		compliance_statuses: string[];
	};

	const MESSAGE_LABEL: Record<string, string> = $derived({ completed: tr('Selesai'), failed: tr('Gagal') });
	const COMPLIANCE_LABEL: Record<string, string> = $derived({
		passed: 'URL terverifikasi',
		failed: 'URL bermasalah',
		unavailable: tr('Tidak tersedia')
	});
	const SENTIMENT_LABEL: Record<string, string> = { positive: 'Positif', neutral: 'Netral', negative: 'Negatif' };
	const STATUS_TONE: Record<string, 'success' | 'neutral' | 'warning' | 'accent' | 'danger'> = {
		published: 'success',
		pending: 'warning',
		waiting_partner: 'warning',
		partner_approved: 'accent',
		partner_rejected: 'danger',
		queued: 'accent',
		processing: 'accent',
		failed: 'danger',
		retrying: 'warning',
		cancelled: 'neutral'
	};

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let rows = $state<HistoryRow[]>([]);
	let filters = $state<Filters | null>(null);
	let brandId = $state('');
	let campaignId = $state('');
	let mediaId = $state('');
	let messageStatus = $state('');
	let complianceStatus = $state('');
	let from = $state('');
	let to = $state('');
	let query = $state('');
	let page = $state(1);
	let total = $state(0);
	let exporting = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(1);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function buildParams(extra: Record<string, string> = {}) {
		const params = new URLSearchParams(extra);
		if (brandId) params.set('brand_id', brandId);
		if (campaignId) params.set('campaign_id', campaignId);
		if (mediaId) params.set('media_id', mediaId);
		if (messageStatus) params.set('message_status', messageStatus);
		if (complianceStatus) params.set('compliance_status', complianceStatus);
		if (from) params.set('from', from);
		if (to) params.set('to', to);
		if (query.trim()) params.set('q', query.trim());
		return params;
	}

	// Pencarian live ala app lama: 300 ms setelah berhenti mengetik, reset ke halaman 1.
	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	function onSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void load(1), 300);
	}

	async function load(targetPage: number) {
		loading = true;
		error = '';
		try {
			const params = buildParams({ limit: String(PAGE_SIZE), offset: String((targetPage - 1) * PAGE_SIZE) });
			const res = await apiRequest(`/v1/platform/pr/analysis-history?${params.toString()}`);
			if (!res.ok) {
				const msg = await readApiError(res, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			const data = await res.json();
			rows = data.rows ?? [];
			total = data.total ?? 0;
			if (!filters) filters = data.filters;
			page = targetPage;
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			loading = false;
		}
	}

	function resetFilters() {
		brandId = '';
		campaignId = '';
		mediaId = '';
		messageStatus = '';
		complianceStatus = '';
		from = '';
		to = '';
		query = '';
		void load(1);
	}

	async function exportCsv() {
		exporting = true;
		error = '';
		try {
			const params = buildParams({ format: 'csv' });
			const res = await apiRequest(`/v1/platform/pr/analysis-history?${params.toString()}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const blob = await res.blob();
			const a = document.createElement('a');
			a.href = URL.createObjectURL(blob);
			a.download = `tauthub-pr-analysis-${new Date().toISOString().slice(0, 10)}.csv`;
			a.click();
			URL.revokeObjectURL(a.href);
		} catch {
			error = tr('Tidak dapat mengunduh CSV. Coba lagi.');
		} finally {
			exporting = false;
		}
	}

	function scoreTone(value: number | null): 'success' | 'warning' | 'danger' | 'neutral' {
		if (value == null) return 'neutral';
		if (value >= 70) return 'success';
		if (value >= 40) return 'warning';
		return 'danger';
	}

	const visibleCampaigns = $derived((filters?.campaigns ?? []).filter((c) => !brandId || c.brand_id === brandId));
</script>

<section>
	<PageHeader
		title={tr("Riwayat Analisis PR")}
		description={tr("Analisis message consistency, sentiment/tone, dan verifikasi URL untuk seluruh publikasi published lintas brand/kampanye/media.")}
	>
		<Badge tone="accent" label="{total} publikasi" />
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat riwayat analisis PR…" />

	{#if rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke riwayat analisis PR.")}</p>
		</div>
	{:else}
		<div class="app-card mt-4 p-4">
			<div class="flex flex-wrap items-end gap-3">
				<label class="block min-w-[200px]">
					<span class="text-xs text-muted-foreground">Brand</span>
					<Select
						ariaLabel="Brand"
						minWidth={200}
						value={brandId}
						onchange={(v) => {
							brandId = v;
							campaignId = '';
							void load(1);
						}}
						options={[
							{ value: '', label: tr('Semua brand') },
							...(filters?.brands ?? []).map((b) => ({ value: b.id, label: b.name }))
						]}
					/>
				</label>
				<label class="block min-w-[200px]">
					<span class="text-xs text-muted-foreground">{tr("Kampanye")}</span>
					<div>
						<Select
							ariaLabel={tr("Kampanye")}
							minWidth={200}
							disabled={visibleCampaigns.length === 0}
							value={campaignId}
							onchange={(v) => {
								campaignId = v;
								void load(1);
							}}
							options={[
								{ value: '', label: tr('Semua kampanye') },
								...visibleCampaigns.map((c) => ({ value: c.id, label: c.name }))
							]}
						/>
					</div>
				</label>
				<label class="block min-w-[200px]">
					<span class="text-xs text-muted-foreground">{tr("Media")}</span>
					<Select
						ariaLabel={tr("Media")}
						minWidth={200}
						value={mediaId}
						onchange={(v) => {
							mediaId = v;
							void load(1);
						}}
						options={[
							{ value: '', label: tr('Semua media') },
							...(filters?.media ?? []).map((m) => ({ value: m.id, label: m.name }))
						]}
					/>
				</label>
				<label class="block min-w-[170px]">
					<span class="text-xs text-muted-foreground">Message analysis</span>
					<Select
						ariaLabel="Message analysis"
						minWidth={190}
						value={messageStatus}
						onchange={(v) => {
							messageStatus = v;
							void load(1);
						}}
						options={[
							{ value: '', label: tr('Semua') },
							...(filters?.message_statuses ?? []).map((s) => ({ value: s, label: MESSAGE_LABEL[s] ?? s }))
						]}
					/>
				</label>
				<label class="block min-w-[190px]">
					<span class="text-xs text-muted-foreground">Verifikasi URL</span>
					<Select
						ariaLabel="Verifikasi URL"
						minWidth={190}
						value={complianceStatus}
						onchange={(v) => {
							complianceStatus = v;
							void load(1);
						}}
						options={[
							{ value: '', label: tr('Semua') },
							...(filters?.compliance_statuses ?? []).map((s) => ({ value: s, label: COMPLIANCE_LABEL[s] ?? s }))
						]}
					/>
				</label>
				<label class="block min-w-[170px]">
					<span class="text-xs text-muted-foreground">{tr("Terbit dari")}</span>
					<input class="app-input mt-1 w-full" type="date" bind:value={from} onchange={() => void load(1)} />
				</label>
				<label class="block min-w-[170px]">
					<span class="text-xs text-muted-foreground">{tr("Terbit sampai")}</span>
					<input class="app-input mt-1 w-full" type="date" bind:value={to} onchange={() => void load(1)} />
				</label>
				<label class="block min-w-[220px]">
					<span class="text-xs text-muted-foreground">{tr("Cari (judul / media / brand)")}</span>
					<span class="relative mt-1 block">
						<Search size={14} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
						<input class="app-input w-full pl-9" placeholder={tr("Cari…")} bind:value={query} oninput={onSearchInput} />
					</span>
				</label>
				<button class="app-btn app-btn-soft" type="button" onclick={resetFilters}>Reset</button>
				<button class="app-btn app-btn-soft" type="button" disabled={exporting || loading} title={tr("Unduh CSV dengan filter yang sedang aktif")} onclick={() => void exportCsv()}>
					{exporting ? 'Mengekspor…' : 'Export CSV'}
				</button>
			</div>
		</div>

		<div class="app-card mt-4 p-4">
			<div class="max-h-[70vh] overflow-auto overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 1180px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">{tr("Publikasi")}</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Brand / Kampanye")}</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Media")}</th>
							<th class="px-3 py-2 text-left font-medium">Message</th>
							<th class="px-3 py-2 text-left font-medium">Sentiment</th>
							<th class="px-3 py-2 text-left font-medium">Compliance</th>
							<th class="px-3 py-2 text-left font-medium">{tr("Terbit")}</th>
						</tr>
					</thead>
					<tbody>
						{#each rows as row (row.publication_id)}
							<tr class="border-t align-top">
								<td class="px-3 py-2">
									<p class="font-medium">{row.document_title ?? 'Artikel'}</p>
									<p class="text-xs text-muted-foreground">{row.published_url ?? 'tanpa URL'}</p>
								</td>
								<td class="px-3 py-2">
									<p>{row.brand_name ?? '—'}</p>
									<p class="text-xs text-muted-foreground">{row.campaign_name ?? 'tanpa kampanye'}</p>
								</td>
								<td class="px-3 py-2">
									<p>{row.media_name ?? '—'}</p>
									<Badge tone="neutral" label={row.media_tier} />
								</td>
								<td class="px-3 py-2">
									<Badge tone={scoreTone(row.message_score)} label={row.message_score != null ? `${row.message_score}/100` : '—'} />
									{#if row.message_status}<p class="mt-1 text-xs text-muted-foreground">{MESSAGE_LABEL[row.message_status] ?? row.message_status}</p>{/if}
									{#if row.message_error}<p class="text-xs" style="color: var(--destructive);">{row.message_error}</p>{/if}
								</td>
								<td class="px-3 py-2">
									{#if row.sentiment}
										<Badge
											tone={row.sentiment === 'positive' ? 'success' : row.sentiment === 'negative' ? 'danger' : 'neutral'}
											label={SENTIMENT_LABEL[row.sentiment] ?? row.sentiment}
										/>
									{:else}
										<span class="text-xs text-muted-foreground">—</span>
									{/if}
									{#if row.sentiment_confidence != null}
										<p class="mt-1 text-xs text-muted-foreground">{Math.round(row.sentiment_confidence * 100)}%</p>
									{/if}
								</td>
								<td class="px-3 py-2">
									<Badge
										tone={row.compliance_status === 'passed' ? 'success' : row.compliance_status === 'failed' ? 'danger' : row.compliance_status ? 'warning' : 'neutral'}
										label={row.compliance_score != null
											? `${row.compliance_score}/100`
											: row.compliance_status
												? (COMPLIANCE_LABEL[row.compliance_status] ?? row.compliance_status)
												: '—'}
									/>
									{#if row.verification_error}<p class="text-xs" style="color: var(--warning);">{row.verification_error}</p>{/if}
								</td>
								<td class="px-3 py-2">
									<p>{row.published_at ? fmtDate(row.published_at) : '—'}</p>
									<Badge tone={STATUS_TONE[row.status] ?? 'neutral'} label={row.status} />
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if rows.length === 0}
					<p class="px-4 py-12 text-center text-sm text-muted-foreground">{tr("Tidak ada data untuk filter ini.")}</p>
				{/if}
			</div>
			<Pagination total={total} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={(next) => void load(next / PAGE_SIZE + 1)} />
			<p class="mt-3 text-xs text-muted-foreground">{tr("Kolom “—” berarti analisis belum berjalan atau data tidak tersedia — tidak ada angka palsu.")}</p>
		</div>
	{/if}
</section>
