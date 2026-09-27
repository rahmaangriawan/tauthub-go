<script lang="ts">
	// Monitoring Order — port app/admin/orders/page.tsx: satu baris = satu order
	// distribusi (satu media). Nomor order per media (ORD-) dan nomor kampanye
	// induk (CAM-) dipakai melacak masalah; detail teknis ada di publikasi.
	import { onMount } from 'svelte';
	import { Receipt, Search, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
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
	const STATUS_LABEL: Record<string, string> = {
		pending: 'Menunggu', waiting_partner: 'Menunggu mitra', partner_approved: 'Disetujui mitra',
		partner_rejected: 'Ditolak mitra', queued: 'Dalam antrean', processing: 'Diproses',
		published: 'Terbit', failed: 'Gagal', retrying: 'Dicoba ulang', cancelled: 'Dibatalkan'
	};
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
</script>

<section class="mx-auto max-w-[1400px]">
	<PageHeader
		title="Monitoring Order"
		description="Satu baris = satu order distribusi (satu media). Nomor order per media dan nomor kampanye induknya dipakai untuk melacak masalah; detail teknis (attempts, job, retry) ada di halaman publikasi."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} order</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke monitoring order.</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat order…" />

		<div class="mt-4 flex flex-wrap items-center gap-2">
			<label class="app-input flex min-w-[260px] flex-1 items-center gap-2 !py-1.5" style="max-width: 400px;">
				<Search size={16} class="text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					placeholder="Cari no. order (ORD-/CAM-), artikel, brand, media…"
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
				<TriangleAlert size={16} />{problemOnly ? 'Hanya bermasalah' : 'Tampilkan yang bermasalah'}
			</button>
		</div>

		<div class="mt-3 flex flex-wrap gap-2">
			{#each filters as f (f || 'all')}
				<button
					class="rounded-full px-3 py-1 text-xs transition-colors"
					style="background-color: {status === f ? 'var(--primary)' : 'var(--secondary)'}; color: {status === f ? 'var(--primary-foreground)' : 'var(--muted-foreground)'};"
					type="button"
					onclick={() => setStatus(f)}
				>{f === '' ? 'Semua' : STATUS_LABEL[f] ?? f}</button>
			{/each}
		</div>

		<div class="app-card mt-3 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1240px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">No. Order</th>
						<th class="px-4 py-2.5 font-medium">Kampanye</th>
						<th class="px-4 py-2.5 font-medium">Artikel</th>
						<th class="px-4 py-2.5 font-medium">Klien</th>
						<th class="px-4 py-2.5 font-medium">Media</th>
						<th class="px-4 py-2.5 font-medium">Harga</th>
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
									<p class="text-xs" style="color: var(--success);">Terbit {fmtDateTime(row.published_at)}</p>
								{:else if row.scheduled_at}
									<p class="text-xs text-muted-foreground">Jadwal {fmtDateTime(row.scheduled_at)}</p>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && orders.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Receipt size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{problemOnly ? 'Tidak ada order bermasalah pada filter ini.' : 'Tidak ada order untuk filter ini.'}</p>
				</div>
			{/if}
		</div>

		<div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
			<span>{fmtNum(total)} order · halaman {page} dari {totalPages}</span>
			<div class="flex gap-2">
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={page <= 1 || loading} onclick={() => changePage(Math.max(1, page - 1))}>‹ Sebelumnya</button>
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={page >= totalPages || loading} onclick={() => changePage(page + 1)}>Berikutnya ›</button>
			</div>
		</div>
	{/if}
</section>
