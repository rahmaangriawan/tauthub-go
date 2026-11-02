<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab "Baru Ditambahkan" /admin/media — media yang baru saja DISETUJUI admin
	// dari tab Pending. Media tampil di sini selama 7 hari sejak disetujui, lalu
	// hilang sendiri (tidak perlu tindakan admin).
	//
	// Penanda waktunya reviewed_at, bukan created_at: mitra bisa menyimpan draf
	// berhari-hari sebelum mengajukan, jadi created_at tidak mencerminkan kapan
	// media benar-benar masuk direktori.
	import { CalendarClock, Search, Sparkles } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import {
		errMessage,
		isAccessError,
		mountWithSession,
		platformRequest,
		categoryText,
		type Media,
		type Partner
	} from './_shared';

	const PAGE_SIZE = 20;
	/** Jendela tampil: 7 hari sejak disetujui (dihitung di server). */
	const RECENT_DAYS = 7;

	let rows = $state<Media[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let error = $state('');
	let loading = $state(true);
	let search = $state('');
	let page = $state(1);

	let debouncedSearch = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const value = search.trim();
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => (debouncedSearch = value), 300);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});

	let ready = $state(false);
	onMount(() => mountWithSession(async () => (ready = true)));

	$effect(() => {
		if (!ready) return;
		void debouncedSearch;
		void page;
		void load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				scope: 'recent',
				recent_days: String(RECENT_DAYS),
				limit: String(PAGE_SIZE),
				offset: String((page - 1) * PAGE_SIZE)
			});
			if (debouncedSearch) params.set('search', debouncedSearch);
			const [mediaData, partnerData] = await Promise.all([
				platformRequest<{ media: Media[]; total: number }>(`/media?${params}`),
				platformRequest<{ partners: Partner[] }>('/partners')
			]);
			rows = mediaData.media ?? [];
			total = mediaData.total ?? 0;
			partners = partnerData.partners ?? [];
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			loading = false;
		}
	}

	const partnerName = (id: string | null) =>
		id
			? (partners.find((p) => p.id === id)?.name ?? partners.find((p) => p.id === id)?.email ?? '—')
			: '—';

	/** Sisa hari sebelum media keluar dari daftar ini. */
	function daysLeft(reviewedAt: string | null | undefined): number | null {
		if (!reviewedAt) return null;
		const approved = new Date(reviewedAt).getTime();
		if (Number.isNaN(approved)) return null;
		const elapsedDays = (Date.now() - approved) / 86_400_000;
		const left = Math.ceil(RECENT_DAYS - elapsedDays);
		return left < 0 ? 0 : left;
	}
</script>

{#snippet mediaCell(row: Media)}
	<div class="flex items-center gap-2">
		<Sparkles size={16} />
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.slug ?? row.id.slice(0, 8)}</p>
		</div>
	</div>
{/snippet}

{#snippet categoryCell(row: Media)}
	<span class="text-muted-foreground">{categoryText(row)}</span>
{/snippet}

{#snippet partnerCell(row: Media)}
	<span class="text-muted-foreground">{partnerName(row.partner_id)}</span>
{/snippet}

{#snippet approvedCell(row: Media)}
	<div class="text-xs">
		<p>{row.reviewed_at ? fmtDateTime(row.reviewed_at) : '—'}</p>
		{#if daysLeft(row.reviewed_at) != null}
			<p class="text-muted-foreground">
				{#if daysLeft(row.reviewed_at) === 0}
					{tr('Keluar dari daftar hari ini')}
				{:else}
					{tr('Sisa')} {daysLeft(row.reviewed_at)} {tr('hari')}
				{/if}
			</p>
		{/if}
	</div>
{/snippet}

{#if loading && rows.length === 0 && !error}
	<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat media baru…')}</p>
{:else if rows.length === 0 && isAccessError(error)}
	<div class="app-card mt-6 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">{tr('Akun ini tidak memiliki akses ke direktori media.')}</p>
	</div>
{:else}
	<div class="app-card mt-4 flex items-start gap-2.5 px-4 py-3">
		<CalendarClock size={16} class="mt-0.5 shrink-0 text-muted-foreground" />
		<p class="text-xs text-muted-foreground">
			{tr(
				'Media yang disetujui dari tab Pending muncul di sini selama 7 hari, lalu keluar otomatis. Tidak perlu tindakan apa pun.'
			)}
		</p>
	</div>

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr('Cari nama, slug, kategori, atau email mitra…')}
				aria-label={tr('Cari media baru')}
				bind:value={search}
				oninput={() => (page = 1)}
			/>
		</label>
		{#if debouncedSearch}
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => (search = '')}
				>Bersihkan pencarian</button
			>
		{/if}
		<div class="flex-1"></div>
		<Badge tone="accent" value={`${total} media baru`} />
	</div>

	{#if error && !isAccessError(error)}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
	{/if}

	<DataTable
		columns={[
			{ key: 'name', label: tr('Media'), render: mediaCell },
			{ key: 'category', label: 'Kategori', render: categoryCell },
			{ key: 'partner', label: tr('Mitra'), render: partnerCell },
			{ key: 'approved', label: tr('Disetujui'), hint: tr('Media yang disetujui dari tab Pending muncul di sini selama 7 hari, lalu keluar otomatis. Tidak perlu tindakan apa pun.'), render: approvedCell }
		]}
		rows={rows}
		rowKey={(row: Media) => row.id}
		{loading}
		empty={tr('Belum ada media yang disetujui dalam 7 hari terakhir.')}
		minWidth="900px"
	/>

	<Pagination
		total={total}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>
{/if}
