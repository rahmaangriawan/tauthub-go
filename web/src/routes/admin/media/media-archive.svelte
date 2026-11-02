<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Arsip /admin/media — media terarsip / disembunyikan dari portal mitra.
	// Port MediaArchiveSection di apps/web/src/app/admin/media/_sections.tsx.
	import { Archive, EyeOff, RefreshCw, Search, Trash } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import {
		errMessage,
		isAccessError,
		mountWithSession,
		platformRequest,
		categoryText,
		type Partner
	} from './_shared';

	const PAGE_SIZE = 20;

	type ArchiveRow = {
		id: string;
		name: string;
		slug: string | null;
		category: string | null;
		categories?: string[];
		partner_id: string | null;
		status: string;
		integration_type: string;
		created_at: string;
	};

	let rows = $state<ArchiveRow[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let error = $state('');
	let loading = $state(true);
	let busyId = $state<string | null>(null);
	let deleteTarget = $state<ArchiveRow | null>(null);
	let deleteBusy = $state(false);
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
				scope: 'archive',
				limit: String(PAGE_SIZE),
				offset: String((page - 1) * PAGE_SIZE)
			});
			if (debouncedSearch) params.set('search', debouncedSearch);
			const [mediaData, partnerData] = await Promise.all([
				platformRequest<{ media: ArchiveRow[]; total: number }>(`/media?${params}`),
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

	async function changeStatus(row: ArchiveRow, status: string) {
		busyId = row.id;
		error = '';
		try {
			await platformRequest(`/media/${row.id}`, { method: 'PUT', body: JSON.stringify({ status }) });
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busyId = null;
		}
	}

	async function deleteForever() {
		if (!deleteTarget) return;
		deleteBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${deleteTarget.id}`, { method: 'DELETE' });
			deleteTarget = null;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			deleteBusy = false;
		}
	}

	const partnerName = (id: string | null) =>
		id ? (partners.find((p) => p.id === id)?.name ?? partners.find((p) => p.id === id)?.email ?? '—') : '—';
</script>

{#snippet mediaCell(row: ArchiveRow)}
	<div class="flex items-center gap-2">
		<Archive size={16} />
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.slug ?? row.id.slice(0, 8)}</p>
		</div>
	</div>
{/snippet}

{#snippet categoryCell(row: ArchiveRow)}
	<span class="text-muted-foreground">{categoryText(row)}</span>
{/snippet}

{#snippet partnerCell(row: ArchiveRow)}
	<span class="text-muted-foreground">{partnerName(row.partner_id)}</span>
{/snippet}

{#snippet integrationCell(row: ArchiveRow)}
	<Badge tone="neutral" value={row.integration_type} />
{/snippet}

{#snippet statusCell(row: ArchiveRow)}
	{#if row.status === 'hidden'}
		<Badge tone="danger" label={tr("Disembunyikan dari mitra")} />
	{:else}
		<Badge tone="warning" label="Terarsip" />
	{/if}
{/snippet}

{#snippet archiveActions(row: ArchiveRow)}
	<div class="flex flex-wrap items-center justify-end gap-2">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--success);"
			disabled={busyId === row.id}
			onclick={() => void changeStatus(row, 'active')}
		>
			<RefreshCw size={13} /> Pulihkan
		</button>
		{#if row.status === 'archived'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				disabled={busyId === row.id}
				onclick={() => void changeStatus(row, 'hidden')}
			>
				<EyeOff size={13} />{tr("Sembunyikan dari mitra")}</button>
		{/if}
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--destructive);"
			onclick={() => (deleteTarget = row)}
		>
			<Trash size={13} />{tr("Hapus permanen")}</button>
	</div>
{/snippet}

{#if loading && rows.length === 0 && !error}
	<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat arsip media…")}</p>
{:else if rows.length === 0 && isAccessError(error)}
	<div class="app-card mt-6 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke arsip media.")}</p>
	</div>
{:else}
	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr("Cari nama, slug, kategori, atau email mitra…")}
				aria-label={tr("Cari media arsip")}
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
		<Badge tone="neutral" value={`${total} arsip`} />
	</div>

	{#if error && !isAccessError(error)}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
	{/if}

	<DataTable
		columns={[
			{ key: 'name', label: tr('Media'), render: mediaCell },
			{ key: 'category', label: 'Kategori', render: categoryCell },
			{ key: 'partner', label: tr('Mitra'), render: partnerCell },
			{ key: 'integration', label: 'Integrasi', render: integrationCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'actions', label: '', align: 'right' as const, render: archiveActions }
		]}
		rows={rows}
		rowKey={(row: ArchiveRow) => row.id}
		{loading}
		empty={tr("Tidak ada media terarsip atau disembunyikan.")}
		minWidth="900px"
	/>

	<Pagination
		total={total}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>
{/if}

{#if deleteTarget}
	<Modal title={tr("Hapus media permanen")} maxWidth="md" onClose={() => (deleteTarget = null)}>
		<p class="text-sm text-muted-foreground">
			{'Media "' +
				(deleteTarget?.name ?? '') +
				'" akan dihapus permanen dari database. Tindakan ini tidak bisa dibatalkan.'}
		</p>
		<div
			class="mt-3 rounded-lg px-3 py-2 text-xs"
			style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);"
		>{tr("Hanya bisa dihapus bila media tidak punya riwayat publikasi, earning, atau rencana publikasi. Data terkait (standar konten, API key, revenue share, keanggotaan paket media) ikut terhapus.")}</div>
		<div class="mt-4 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" onclick={() => (deleteTarget = null)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--destructive);"
				disabled={deleteBusy}
				onclick={deleteForever}>{deleteBusy ? 'Menghapus…' : tr('Hapus permanen')}</button
			>
		</div>
	</Modal>
{/if}
