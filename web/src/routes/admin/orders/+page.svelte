<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Monitoring Order — port app/admin/orders/page.tsx: satu baris = satu order
	// distribusi (satu media). Nomor order per media (ORD-) dan nomor kampanye
	// induk (CAM-) dipakai melacak masalah; detail teknis ada di publikasi.
	// Tab "Per Kampanye": satu baris = satu order kampanye (CAM-) sebagai
	// entitas — media terorder, terbit, rencana vs terpakai rupiah.
	import { onMount } from 'svelte';
	import { Receipt, Search, TriangleAlert } from '@lucide/svelte';
	import { page as routePage } from '$app/state';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime, fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from '../_platform';

	type Order = {
		publication_id: string; order_code: string | null; campaign_order_code: string | null;
		campaign_name: string | null; status: string; publish_status: string | null;
		client_price: number; scheduled_at: string | null; published_at: string | null;
		published_url: string | null; created_at: string; brand_name: string | null;
		brand_public_id: string | null; media_name: string | null;
		media_integration_type: string | null; document_title: string | null;
		error_code: string | null; error_message: string | null;
	};

	const STATUS_TONE: Record<string, 'success' | 'neutral' | 'warning' | 'accent' | 'danger'> = {
		published: 'success', pending: 'warning', waiting_partner: 'warning',
		partner_approved: 'accent', partner_rejected: 'danger', queued: 'accent',
		processing: 'accent', failed: 'danger', retrying: 'warning', cancelled: 'neutral'
	};
	const STATUS_LABEL: Record<string, string> = $derived({
		pending: tr('Menunggu'), waiting_partner: tr('Menunggu mitra'), partner_approved: tr('Disetujui mitra'),
		partner_rejected: tr('Ditolak mitra'), queued: 'Dalam antrean', processing: 'Diproses',
		published: tr('Terbit'), failed: tr('Gagal'), retrying: 'Dicoba ulang', cancelled: tr('Dibatalkan')
	});
	const PROBLEM_STATUSES = ['failed', 'retrying', 'partner_rejected', 'cancelled'];
	const PAGE_SIZE = 20;

	let orders = $state<Order[]>([]);
	let status = $state('');
	let problemOnly = $state(false);
	let query = $state('');
	let error = $state('');
	let loading = $state(true);
	let page = $state(1);
	let total = $state(0);
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
			// Langsung dibuka dengan ?tab=kampanye (link/refresh): tab sudah
			// 'kampanye' tanpa lewat setTab, jadi datanya harus dimuat di sini.
			if (tab === 'kampanye') void loadCampaigns();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		const params = new URLSearchParams();
		if (status) params.set('status', status);
		if (problemOnly) params.set('problem', '1');
		if (query.trim()) params.set('q', query.trim());
		params.set('limit', String(PAGE_SIZE));
		params.set('offset', String((page - 1) * PAGE_SIZE));
		const res = await apiRequest(`/v1/platform/orders?${params.toString()}`);
		if (res.ok) {
			const data = await res.json();
			orders = data.orders ?? [];
			total = data.total ?? 0;
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	function setStatus(value: string) {
		status = value;
		page = 1;
		void load();
	}

	function toggleProblem() {
		problemOnly = !problemOnly;
		page = 1;
		void load();
	}

	function onQuery(value: string) {
		query = value;
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			page = 1;
			void load();
		}, 300);
	}

	function changePage(next: number) {
		page = next;
		void load();
	}

	const filters = ['', ...PROBLEM_STATUSES, 'waiting_partner', 'queued', 'processing', 'published'];
	const totalPages = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	const denied = $derived(Boolean(!loading && isAccessDenied(error)));

	// ---------- tab Per Kampanye ----------
	type CampaignRow = {
		campaign_id: string; order_code: string | null; name: string;
		brand_name: string | null; brand_public_id: string | null;
		taut_status: string | null; campaign_status: string | null; budget: number;
		media_count: number; publication_count: number; published_count: number;
		planned_cost: number; realized_cost: number; created_at: string;
	};

	const CAMP_STATUS_TONE: Record<string, 'success' | 'neutral' | 'warning' | 'accent' | 'danger'> = {
		submitted: 'success', scheduled: 'success', distributed: 'success',
		review: 'accent', placement: 'accent', production: 'accent', submitted_review: 'accent',
		draft: 'neutral', budget: 'neutral', materials: 'neutral', active: 'warning'
	};
	const CAMP_STATUS_LABEL: Record<string, string> = $derived({
		draft: 'Draf', materials: 'Materi', budget: 'Budget', profiled: 'Profil selesai',
		production: 'Produksi', review: 'Review', placement: 'Media terkunci',
		submitted: 'Disubmit', scheduled: 'Terjadwal', distributed: 'Tersebar',
		active: 'Aktif', paused: 'Dijeda'
	});

	let tab = $state(routePage.url.searchParams.get('tab') === 'kampanye' ? 'kampanye' : 'artikel');
	let campaigns = $state<CampaignRow[]>([]);
	let campQuery = $state('');
	let campError = $state('');
	let campLoading = $state(true);
	let campPage = $state(1);
	let campTotal = $state(0);
	const CAMP_PAGE_SIZE = 25;
	let campTimer: ReturnType<typeof setTimeout> | null = null;
	const campTotalPages = $derived(Math.max(1, Math.ceil(campTotal / CAMP_PAGE_SIZE)));

	function setTab(next: 'artikel' | 'kampanye') {
		tab = next;
		history.replaceState({}, '', `/admin/orders${next === 'kampanye' ? '?tab=kampanye' : ''}`);
		if (next === 'kampanye' && campaigns.length === 0) void loadCampaigns();
	}

	async function loadCampaigns() {
		campLoading = true;
		campError = '';
		const params = new URLSearchParams();
		if (campQuery.trim()) params.set('q', campQuery.trim());
		params.set('limit', String(CAMP_PAGE_SIZE));
		params.set('offset', String((campPage - 1) * CAMP_PAGE_SIZE));
		const res = await apiRequest(`/v1/platform/orders/campaigns?${params.toString()}`);
		if (res.ok) {
			const data = await res.json();
			campaigns = data.campaigns ?? [];
			campTotal = data.total ?? 0;
		} else {
			campError = await platformError(res);
		}
		campLoading = false;
	}

	function onCampQuery(value: string) {
		campQuery = value;
		if (campTimer) clearTimeout(campTimer);
		campTimer = setTimeout(() => {
			campPage = 1;
			void loadCampaigns();
		}, 300);
	}

	function changeCampPage(next: number) {
		campPage = next;
		void loadCampaigns();
	}

	function campStatusLabel(row: CampaignRow): string {
		const st = row.taut_status ?? row.campaign_status ?? '';
		return CAMP_STATUS_LABEL[st] ?? st;
	}
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Publikasi'), href: '/admin/publications' }]} crumb={tr('Monitoring Order')} path='/admin/orders'>
	<PageHeader
		title="Monitoring Order"
		description={tr("Satu baris = satu order distribusi (satu media). Nomor order per media dan nomor kampanye induknya dipakai untuk melacak masalah; detail teknis (attempts, job, retry) ada di halaman publikasi.")}
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} order</span>
	</PageHeader>

	<div class="mt-4 flex items-center gap-1 rounded-full p-1" style="width: fit-content; background-color: var(--secondary);">
		<button
			class="rounded-full px-4 py-1.5 text-xs font-medium transition-colors"
			style="background-color: {tab === 'artikel' ? 'var(--primary)' : 'transparent'}; color: {tab === 'artikel' ? 'var(--primary-foreground)' : 'var(--muted-foreground)'};"
			type="button"
			onclick={() => setTab('artikel')}
		>Per Artikel</button>
		<button
			class="rounded-full px-4 py-1.5 text-xs font-medium transition-colors"
			style="background-color: {tab === 'kampanye' ? 'var(--primary)' : 'transparent'}; color: {tab === 'kampanye' ? 'var(--primary-foreground)' : 'var(--muted-foreground)'};"
			type="button"
			onclick={() => setTab('kampanye')}
		>Per Kampanye</button>
	</div>

	{#if tab === 'kampanye'}
		{#if denied}
			<div class="app-card mt-6 px-6 py-10 text-center">
				<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke monitoring order.")}</p>
			</div>
		{:else}
			<Feedback loading={campLoading} error={campError} loadingText="Memuat kampanye…" />

			<div class="mt-4 flex flex-wrap items-center gap-2">
				<label class="app-input flex min-w-[260px] flex-1 items-center gap-2 !py-1.5" style="max-width: 400px;">
					<Search size={16} class="text-muted-foreground" />
					<input
						class="w-full bg-transparent text-sm outline-none"
						placeholder="Cari no. kampanye (CAM-), nama kampanye, klien…"
						value={campQuery}
						oninput={(e) => onCampQuery((e.currentTarget as HTMLInputElement).value)}
					/>
				</label>
				<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(campTotal)} kampanye</span>
			</div>

			<div class="app-card mt-3 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 1180px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">No. Kampanye</th>
							<th class="px-4 py-2.5 font-medium">Nama</th>
							<th class="px-4 py-2.5 font-medium">Klien</th>
							<th class="px-4 py-2.5 font-medium">Status</th>
							<th class="px-4 py-2.5 font-medium">Media</th>
							<th class="px-4 py-2.5 font-medium">Terbit</th>
							<th class="px-4 py-2.5 font-medium">Rencana</th>
							<th class="px-4 py-2.5 font-medium">Terpakai</th>
							<th class="px-4 py-2.5 font-medium">Waktu</th>
						</tr>
					</thead>
					<tbody>
						{#each campaigns as row (row.campaign_id)}
							<tr class="border-t">
								<td class="px-4 py-2.5">
									<a class="font-semibold" style="color: var(--primary);" href={`/admin/orders/kampanye/${row.campaign_id}`}>
										{row.order_code ?? row.campaign_id.slice(0, 8)}
									</a>
								</td>
								<td class="max-w-[240px] px-4 py-2.5">{row.name ?? '—'}</td>
								<td class="px-4 py-2.5">{row.brand_name ?? '—'}</td>
								<td class="px-4 py-2.5">
									<Badge label={campStatusLabel(row)} tone={CAMP_STATUS_TONE[row.taut_status ?? row.campaign_status ?? ''] ?? 'neutral'} />
								</td>
								<td class="px-4 py-2.5 tabular-nums">{fmtNum(row.media_count)}</td>
								<td class="px-4 py-2.5 tabular-nums">{fmtNum(row.published_count)}</td>
								<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.planned_cost)}</td>
								<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.realized_cost)}</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">{fmtDateTime(row.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if !campLoading && campaigns.length === 0}
					<div class="flex flex-col items-center px-4 py-12 text-center">
						<Receipt size={26} class="text-muted-foreground" />
						<p class="mt-2 text-sm text-muted-foreground">Tidak ada kampanye untuk pencarian ini.</p>
					</div>
				{/if}
			</div>

			<div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
				<span>{fmtNum(campTotal)} kampanye · halaman {campPage} dari {campTotalPages}</span>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={campPage <= 1 || campLoading} onclick={() => changeCampPage(Math.max(1, campPage - 1))}>‹ Sebelumnya</button>
					<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={campPage >= campTotalPages || campLoading} onclick={() => changeCampPage(campPage + 1)}>Berikutnya ›</button>
				</div>
			</div>
		{/if}
	{:else}
	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke monitoring order.")}</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat order…" />

		<div class="mt-4 flex flex-wrap items-center gap-2">
			<label class="app-input flex min-w-[260px] flex-1 items-center gap-2 !py-1.5" style="max-width: 400px;">
				<Search size={16} class="text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					placeholder={tr("Cari no. order (ORD-/CAM-), artikel, brand, media…")}
					value={query}
					oninput={(e) => onQuery((e.currentTarget as HTMLInputElement).value)}
				/>
			</label>
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-soft={!problemOnly}
				type="button"
				style={problemOnly ? 'background-color: var(--destructive-soft, #fdecee); color: var(--destructive);' : undefined}
				onclick={toggleProblem}
			>
				<TriangleAlert size={16} />{problemOnly ? 'Hanya bermasalah' : tr('Tampilkan yang bermasalah')}
			</button>
		</div>

		<div class="mt-3 flex flex-wrap gap-2">
			{#each filters as f (f || 'all')}
				<button
					class="rounded-full px-3 py-1 text-xs transition-colors"
					style="background-color: {status === f ? 'var(--primary)' : 'var(--secondary)'}; color: {status === f ? 'var(--primary-foreground)' : 'var(--muted-foreground)'};"
					type="button"
					onclick={() => setStatus(f)}
				>{f === '' ? tr('Semua') : STATUS_LABEL[f] ?? f}</button>
			{/each}
		</div>

		<div class="app-card mt-3 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1240px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">No. Order</th>
						<th class="px-4 py-2.5 font-medium">{tr("Kampanye")}</th>
						<th class="px-4 py-2.5 font-medium">Artikel</th>
						<th class="px-4 py-2.5 font-medium">{tr("Klien")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Harga")}</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Waktu</th>
					</tr>
				</thead>
				<tbody>
					{#each orders as row (row.publication_id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<a class="font-semibold" style="color: var(--primary);" href={`/admin/publications/${row.publication_id}`}>
									{row.order_code ?? row.publication_id.slice(0, 8)}
								</a>
							</td>
							<td class="px-4 py-2.5">
								{#if row.campaign_order_code}
									<p class="tabular-nums">{row.campaign_order_code}</p>
								{:else}
									<p class="text-muted-foreground">Tanpa kampanye</p>
								{/if}
								{#if row.campaign_name}<p class="text-xs text-muted-foreground">{row.campaign_name}</p>{/if}
							</td>
							<td class="max-w-[260px] px-4 py-2.5">{row.document_title ?? '—'}</td>
							<td class="px-4 py-2.5">{row.brand_name ?? '—'}</td>
							<td class="px-4 py-2.5">
								<p>{row.media_name ?? '—'}</p>
								{#if row.media_integration_type}<p class="text-xs text-muted-foreground">{row.media_integration_type}</p>{/if}
							</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.client_price)}</td>
							<td class="px-4 py-2.5">
								<Badge label={STATUS_LABEL[row.status] ?? row.status} tone={STATUS_TONE[row.status] ?? 'neutral'} />
								{#if row.error_code}<p class="mt-1 text-xs" style="color: var(--destructive);">{row.error_code}</p>{/if}
							</td>
							<td class="px-4 py-2.5">
								<p class="text-xs text-muted-foreground">{fmtDateTime(row.created_at)}</p>
								{#if row.published_at}
									<p class="text-xs" style="color: var(--success);">{tr("Terbit")} {fmtDateTime(row.published_at)}</p>
								{:else if row.scheduled_at}
									<p class="text-xs text-muted-foreground">{tr("Jadwal")} {fmtDateTime(row.scheduled_at)}</p>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && orders.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Receipt size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{problemOnly ? tr('Tidak ada order bermasalah pada filter ini.') : tr('Tidak ada order untuk filter ini.')}</p>
				</div>
			{/if}
		</div>

		<div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
			<span>{fmtNum(total)} order · halaman {page} {tr("dari")} {totalPages}</span>
			<div class="flex gap-2">
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={page <= 1 || loading} onclick={() => changePage(Math.max(1, page - 1))}>‹ Sebelumnya</button>
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={page >= totalPages || loading} onclick={() => changePage(page + 1)}>Berikutnya ›</button>
			</div>
		</div>
		{/if}
	{/if}
	</ExplorerFrame>
</section>

