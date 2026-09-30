<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Audit log platform — port app/admin/audit-log/page.tsx.
	// Jejak tindakan platform & brand: filter server-side (search/brand/action) + paginasi offset.
	import { onMount } from 'svelte';
	import { Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type Audit = {
		id: string;
		brand_id?: string;
		brand_name?: string;
		actor_email?: string;
		action: string;
		target_type: string;
		target_id?: string;
		created_at: string;
	};
	type Page = { total: number; items: Audit[] };
	type BrandOption = { id: string; name: string };

	const PAGE_SIZE_OPTIONS = [10, 20, 50];
	const ACTION_OPTIONS = [
		{ value: 'membership.', label: 'Member' },
		{ value: 'brand.', label: 'Brand' },
		{ value: 'content.', label: 'Konten' },
		{ value: 'platform.', label: 'Platform' }
	];

	let loading = $state(true);
	let error = $state('');
	let data = $state<Page>({ total: 0, items: [] });
	let brands = $state<BrandOption[]>([]);
	let offset = $state(0);
	let pageSize = $state(20);
	let searchInput = $state('');
	let search = $state('');
	let brand = $state('');
	let action = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(0, 20, '', '', '');
			const res = await apiRequest('/v1/platform/operations/brands');
			if (res.ok) brands = (await res.json()) ?? [];
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(nextOffset: number, nextLimit: number, q: string, brandId: string, act: string) {
		loading = true;
		error = '';
		const params = new URLSearchParams({ offset: String(nextOffset), limit: String(nextLimit) });
		if (q) params.set('search', q);
		if (brandId) params.set('brand_id', brandId);
		if (act) params.set('action', act);
		try {
			const res = await apiRequest(`/v1/platform/operations/audit-log?${params.toString()}`);
			if (!res.ok) {
				const detail = await readApiError(res, 'Permintaan platform gagal.');
				error = detail === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : detail;
				return;
			}
			data = await res.json();
			offset = nextOffset;
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			loading = false;
		}
	}

	function applyFilters() {
		// Nilai input yang diketik harus dipindahkan ke state `search` yang dikirim
		// ke API; tanpa ini kotak pencarian tidak memfilter apa pun.
		search = searchInput.trim();
		void load(0, pageSize, search, brand, action);
	}

	function resetFilters() {
		searchInput = '';
		search = '';
		brand = '';
		action = '';
		void load(0, pageSize, '', '', '');
	}

	function changePageSize(value: number) {
		pageSize = value;
		void load(0, value, search, brand, action);
	}

	const hasActiveFilter = $derived(Boolean(search || brand || action));
</script>

<section>
	<PageHeader title="Audit Log" description={tr("Jejak tindakan platform dan brand yang tersimpan di server.")}>
		<Badge tone="accent" label="{data.total.toLocaleString('id-ID')} events" />
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat audit log…" />

	{#if error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke audit log platform.")}</p>
		</div>
	{:else}
		<form class="app-card mt-4 flex flex-wrap items-center gap-3 p-3" onsubmit={(e) => { e.preventDefault(); applyFilters(); }}>
			<div class="relative min-w-[240px] flex-1">
				<Search size={14} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
				<input
					class="app-input w-full pl-9"
					placeholder={tr("Cari action, target, atau actor...")}
					aria-label={tr("Cari audit log")}
					bind:value={searchInput}
				/>
			</div>
			<Select
				ariaLabel="Filter brand"
				minWidth={180}
				value={brand}
				onchange={(v) => {
					brand = v;
					void load(0, pageSize, search, brand, action);
				}}
				options={[{ value: '', label: tr('Semua brand') }, ...brands.map((b) => ({ value: b.id, label: b.name }))]}
			/>
			<Select
				ariaLabel="Filter jenis aksi"
				minWidth={160}
				value={action}
				onchange={(v) => {
					action = v;
					void load(0, pageSize, search, brand, action);
				}}
				options={[{ value: '', label: tr('Semua aksi') }, ...ACTION_OPTIONS.map((o) => ({ value: o.value, label: o.label }))]}
			/>
			<button class="app-btn app-btn-soft" type="submit" disabled={loading}>Terapkan filter</button>
			{#if hasActiveFilter}
				<button class="app-btn app-btn-soft" type="button" disabled={loading} onclick={resetFilters}>Reset</button>
			{/if}
		</form>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 860px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">Waktu</th>
						<th class="px-4 py-2.5 text-left font-medium">Action</th>
						<th class="px-4 py-2.5 text-left font-medium">Actor</th>
						<th class="px-4 py-2.5 text-left font-medium">Brand</th>
						<th class="px-4 py-2.5 text-left font-medium">Target</th>
					</tr>
				</thead>
				<tbody>
					{#each data.items as event (event.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">{fmtDateTime(event.created_at)}</td>
							<td class="px-4 py-2.5"><Badge tone="neutral" label={event.action} /></td>
							<td class="px-4 py-2.5">{event.actor_email || 'System'}</td>
							<td class="px-4 py-2.5">{event.brand_name || event.brand_id || 'Platform'}</td>
							<td class="px-4 py-2.5">
								<span>{event.target_type}</span>
								{#if event.target_id}
									<span class="block text-xs text-muted-foreground">{event.target_id}</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && data.items.length === 0}
				<p class="px-4 py-12 text-center text-sm text-muted-foreground">{tr("Tidak ada audit event yang cocok.")}</p>
			{/if}
		</div>

		<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
			<div class="flex items-center gap-3">
				<p class="text-sm text-muted-foreground">
					{data.total ? `${offset + 1}–${Math.min(offset + pageSize, data.total)} dari ${data.total} events` : '0 events'}
				</p>
				<Select
					ariaLabel="Baris per halaman"
					minWidth={140}
					value={String(pageSize)}
					onchange={(v) => changePageSize(Number(v))}
					options={PAGE_SIZE_OPTIONS.map((size) => ({ value: String(size), label: `${size} / halaman` }))}
				/>
			</div>
			<Pagination total={data.total} limit={pageSize} {offset} onChange={(next) => void load(next, pageSize, search, brand, action)} />
		</div>
	{/if}
</section>
