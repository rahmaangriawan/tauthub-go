<script lang="ts">
	// Semua brand — port app/admin/brands/page.tsx: direktori brand platform
	// (owner, plan, jumlah member, status generation) + pencarian server-side.
	import { onMount } from 'svelte';
	import { Building2, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type Brand = {
		id: string; name: string; slug: string; owner_email: string;
		member_count: number; plan_code: string; generation_enabled: boolean; created_at: string;
	};

	const PAGE_SIZE = 20;

	let rows = $state<Brand[]>([]);
	let error = $state('');
	let loading = $state(true);
	let search = $state('');
	let pageNum = $state(1);

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

	async function load(term = '') {
		loading = true;
		error = '';
		const query = term ? `?search=${encodeURIComponent(term)}` : '';
		const res = await apiRequest(`/v1/platform/operations/brands${query}`);
		if (res.ok) rows = await res.json();
		else error = await platformError(res);
		loading = false;
	}

	function submitSearch(e: SubmitEvent) {
		e.preventDefault();
		pageNum = 1;
		void load(search.trim());
	}

	const start = $derived((pageNum - 1) * PAGE_SIZE);
	const visibleRows = $derived(rows.slice(start, start + PAGE_SIZE));
	const denied = $derived(Boolean(!loading && rows.length === 0 && error.includes('Akses platform')));
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader title="Brands" description="Daftar brand, ownership, plan, dan status generation.">
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{rows.length} brand</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke direktori brand platform.</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat direktori brand…" />

		<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" role="search" onsubmit={submitSearch}>
			<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
				<Search size={15} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" name="search" aria-label="Cari brand" placeholder="Cari nama, slug, atau owner..." bind:value={search} />
			</label>
			<button class="app-btn app-btn-soft" type="submit" disabled={loading}>Cari</button>
		</form>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 880px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Brand</th>
						<th class="px-4 py-2.5 font-medium">Owner</th>
						<th class="px-4 py-2.5 font-medium">Plan</th>
						<th class="px-4 py-2.5 font-medium">Members</th>
						<th class="px-4 py-2.5 font-medium">Generation</th>
						<th class="px-4 py-2.5 font-medium">Created</th>
					</tr>
				</thead>
				<tbody>
					{#each visibleRows as brand (brand.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<Building2 size={17} class="shrink-0 text-muted-foreground" />
									<div>
										<a class="font-medium underline" style="color: inherit;" href={`/klien/${brand.id}`}>{brand.name}</a>
										<p class="text-xs text-muted-foreground">{brand.slug}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5">{brand.owner_email}</td>
							<td class="px-4 py-2.5"><Badge label={brand.plan_code} tone="accent" /></td>
							<td class="px-4 py-2.5 tabular-nums">{brand.member_count}</td>
							<td class="px-4 py-2.5">
								<Badge label={brand.generation_enabled ? 'Enabled' : 'Disabled'} tone={brand.generation_enabled ? 'success' : 'danger'} />
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(brand.created_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Building2 size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">Tidak ada brand yang cocok.</p>
				</div>
			{/if}
			<div class="px-4 pb-4">
				<Pagination
					total={rows.length}
					limit={PAGE_SIZE}
					offset={start}
					onChange={(next) => (pageNum = Math.floor(next / PAGE_SIZE) + 1)}
				/>
			</div>
		</div>
	{/if}
</section>
