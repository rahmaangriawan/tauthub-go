<script lang="ts">
	// Tab Pending /admin/media — media baru menunggu review Admin (pending_review).
	// Port MediaPendingSection di apps/web/src/app/admin/media/_sections.tsx.
	import { Search } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import {
		errMessage,
		isAccessError,
		idr,
		mountWithSession,
		platformRequest,
		categoryText,
		type Media,
		type Partner
	} from './_shared';

	const PAGE_SIZE = 20;

	let rows = $state<Media[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let error = $state('');
	let loading = $state(true);
	let search = $state('');
	let page = $state(1);
	let reviewTarget = $state<Media | null>(null);
	let rejectReason = $state('');
	let reviewBusy = $state(false);

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
				limit: String(PAGE_SIZE),
				offset: String((page - 1) * PAGE_SIZE),
				status: 'pending_review'
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

	async function approveMedia(media: Media) {
		reviewBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${media.id}/review`, {
				method: 'POST',
				body: JSON.stringify({ decision: 'approve' })
			});
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			reviewBusy = false;
		}
	}

	async function rejectMedia() {
		if (!reviewTarget) return;
		const reason = rejectReason.trim();
		if (!reason) {
			error = 'Alasan penolakan wajib diisi.';
			return;
		}
		reviewBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${reviewTarget.id}/review`, {
				method: 'POST',
				body: JSON.stringify({ decision: 'reject', reason })
			});
			reviewTarget = null;
			rejectReason = '';
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			reviewBusy = false;
		}
	}

	const partnerLabel = (id: string | null) =>
		id ? (partners.find((p) => p.id === id)?.name ?? partners.find((p) => p.id === id)?.email ?? '—') : '—';

	// Daftar perubahan mitra yang menunggu keputusan admin (migrasi 159).
	const pendingChanges = (row: Media) => (Array.isArray(row.pending_changes) ? row.pending_changes : []);
</script>

{#snippet mediaCell(row: Media)}
	<div class="flex items-center gap-2">
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.slug ?? row.id.slice(0, 8)}</p>
		</div>
	</div>
{/snippet}

{#snippet changesCell(row: Media)}
	{#if pendingChanges(row).length === 0}
		<p class="text-xs text-muted-foreground">Pengajuan baru — tanpa perubahan data.</p>
	{:else}
		<div style="min-width: 200px; max-width: 320px;">
			{#each pendingChanges(row) as c (c.field)}
				<p class="mb-1 text-xs leading-snug">
					<span class="font-medium">{c.label}:</span>
					<span class="text-muted-foreground">{c.from ?? '(kosong)'}</span>
					{' → '}
					<span style="color: var(--primary);">{c.to ?? '(kosong)'}</span>
				</p>
			{/each}
		</div>
	{/if}
{/snippet}

{#snippet websiteCell(row: Media)}
	<p class="max-w-[220px] truncate text-xs text-muted-foreground">{row.website_url ?? '—'}</p>
{/snippet}

{#snippet priceCell(row: Media)}
	<span class="tabular-nums">{idr(row.price ?? 0)}</span>
{/snippet}

{#snippet partnerCell(row: Media)}
	<div>
		<p class="text-sm">{partnerLabel(row.partner_id)}</p>
		<div class="mt-1">
			{#if row.referred_by_partner_id}
				<Badge
					tone="accent"
					label={`Referral: ${row.referrer_name ?? row.referred_by_partner_id.slice(0, 8)}`}
				/>
			{:else}
				<Badge tone="neutral" label="Non-referral" />
			{/if}
		</div>
	</div>
{/snippet}

{#snippet updatedCell(row: Media)}
	<span class="text-xs text-muted-foreground">{fmtDateTime(row.updated_at ?? row.created_at)}</span>
{/snippet}

{#snippet categoryCell(row: Media)}
	<span class="text-muted-foreground">{categoryText(row)}</span>
{/snippet}

{#snippet statusCell()}
	<Badge tone="warning" label="🟡 Menunggu Review" />
{/snippet}

{#snippet pendingActions(row: Media)}
	<div class="flex gap-2">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--success);"
			disabled={reviewBusy}
			onclick={() => void approveMedia(row)}>Setujui</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--destructive);"
			disabled={reviewBusy}
			onclick={() => {
				reviewTarget = row;
				rejectReason = '';
				error = '';
			}}>Tolak</button
		>
	</div>
{/snippet}

{#if loading && rows.length === 0 && !error}
	<p class="mt-6 text-sm text-muted-foreground">Memuat media pending…</p>
{:else if rows.length === 0 && isAccessError(error)}
	<div class="app-card mt-6 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke media pending.</p>
	</div>
{:else}
	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder="Cari nama, slug, atau kategori…"
				aria-label="Cari media pending"
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
		<Badge tone="warning" value={`${total} menunggu review`} />
		<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={load}>Muat ulang</button>
	</div>

	{#if error && !isAccessError(error)}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
	{/if}

	<DataTable
		columns={[
			{ key: 'name', label: 'Media', render: mediaCell },
			{ key: 'changes', label: 'Perubahan', render: changesCell },
			{ key: 'category', label: 'Kategori', render: categoryCell },
			{ key: 'website', label: 'Website', render: websiteCell },
			{ key: 'price', label: 'Harga', render: priceCell },
			{ key: 'partner', label: 'Mitra', render: partnerCell },
			{ key: 'updated', label: 'Diperbarui', render: updatedCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'actions', label: '', render: pendingActions }
		]}
		rows={rows}
		rowKey={(row: Media) => row.id}
		{loading}
		empty="Tidak ada media menunggu review."
		minWidth="1200px"
	/>

	<Pagination
		total={total}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>
{/if}

{#if reviewTarget}
	<Modal title={`Review media — ${reviewTarget.name}`} maxWidth="md" onClose={() => (reviewTarget = null)}>
		<p class="text-sm text-muted-foreground">
			Tolak media ini. Alasan akan dikirim ke mitra. (Negosiasi harga dinonaktifkan — harga ditentukan mitra.)
		</p>
		<label class="mt-3 block">
			<span class="text-xs font-medium text-muted-foreground">Alasan / Catatan</span>
			<textarea
				class="app-input mt-1 w-full"
				rows="4"
				bind:value={rejectReason}
				placeholder="Contoh: Kualitas konten belum memenuhi standar..."
			></textarea>
		</label>
		<div class="mt-3 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" disabled={reviewBusy} onclick={() => (reviewTarget = null)}>Batal</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--destructive);"
				disabled={reviewBusy}
				onclick={rejectMedia}>{reviewBusy ? 'Menyimpan…' : 'Tolak Media'}</button
			>
		</div>
	</Modal>
{/if}
