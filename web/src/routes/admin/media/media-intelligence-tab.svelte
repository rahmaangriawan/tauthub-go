<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Intelligence /admin/media — health score media dari riwayat publikasi.
	// Port MediaIntelligenceSection di apps/web/src/app/admin/media/_sections.tsx.
	import { Activity, Archive, RefreshCw } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import {
		compactNumber,
		errMessage,
		isAccessError,
		mountWithSession,
		platformRequest,
		scoreTone,
		categoryText,
		AUDIT_LABEL,
		auditTone,
		type AuditItem,
		type Breakdown
	} from './_shared';

	const PAGE_SIZE = 20;

	type IntelligenceRow = {
		id: string;
		name: string;
		slug: string | null;
		category: string | null;
		categories?: string[];
		website_url: string | null;
		price: number;
		credit_cost: number;
		status: string;
		integration_type: string;
		partner_id: string | null;
		traffic_estimate: number;
		health_score: number | null;
		health_breakdown: Breakdown | null;
		health_computed_at: string | null;
		warnings: string[];
	};

	type Summary = {
		total: number;
		scored: number;
		unhealthy: number;
		with_warnings: number;
		archived: number;
	} | null;

	type ArchiveImpact = {
		archivable: number;
		pending_publications: number;
		by_status: Record<string, number>;
		blocked: boolean;
		blocking_statuses: string[];
		by_media: { id: string; name: string; total: number; counts: Record<string, number> }[];
	};

	const PUB_STATUS_LABEL: Record<string, string> = $derived({
		pending: 'Pending',
		waiting_partner: tr('Menunggu persetujuan mitra'),
		partner_approved: tr('Disetujui mitra'),
		queued: 'Diantrekan',
		processing: 'Diproses',
		retrying: 'Mencoba ulang'
	});

	let rows = $state<IntelligenceRow[]>([]);
	let summary = $state<Summary>(null);
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let search = $state('');
	let status = $state('');
	let includeArchived = $state(false);
	let page = $state(1);
	let detailTarget = $state<IntelligenceRow | null>(null);
	let detailAudit = $state<AuditItem[] | null>(null);
	let detailLoading = $state(false);
	let restoreTarget = $state<IntelligenceRow | null>(null);
	let restoreBusy = $state(false);
	let restoreAllOpen = $state(false);
	let archiveAllOpen = $state(false);
	let archiveImpact = $state<ArchiveImpact | null>(null);

	async function load(searchValue = '', statusValue = '', withArchived = false) {
		loading = true;
		error = '';
		try {
			const query = new URLSearchParams();
			if (searchValue) query.set('search', searchValue);
			if (statusValue) query.set('status', statusValue);
			if (withArchived) query.set('include_archived', '1');
			const data = await platformRequest<{ media: IntelligenceRow[]; summary: Summary }>(
				`/media/intelligence?${query.toString()}`
			);
			rows = data.media ?? [];
			summary = data.summary ?? null;
			page = 1;
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			loading = false;
		}
	}

	onMount(() => mountWithSession(() => load('', '', false)));

	async function recompute() {
		busy = true;
		error = '';
		try {
			const result = await platformRequest<{ refreshed: number }>('/media/recompute-health', { method: 'POST' });
			error = `Recompute selesai: ${result.refreshed} media diperbarui.`;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}

	async function recomputeOne(id: string) {
		busy = true;
		error = '';
		try {
			await platformRequest(`/media/${id}/recompute-health`, { method: 'POST' });
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}

	async function restoreMedia() {
		if (!restoreTarget) return;
		restoreBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${restoreTarget.id}`, {
				method: 'PUT',
				body: JSON.stringify({ status: 'active' })
			});
			restoreTarget = null;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			restoreBusy = false;
		}
	}

	async function restoreAll() {
		busy = true;
		error = '';
		try {
			const result = await platformRequest<{ restored: number }>('/media/restore-archived', { method: 'POST' });
			restoreAllOpen = false;
			error = `Pulihkan selesai: ${result.restored} media dikembalikan aktif.`;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}

	async function openArchiveAll() {
		busy = true;
		error = '';
		archiveImpact = null;
		try {
			archiveImpact = await platformRequest<ArchiveImpact>('/media/archive-impact');
			archiveAllOpen = true;
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}

	async function archiveAll() {
		busy = true;
		error = '';
		try {
			const result = await platformRequest<{ archived: number; pending_publications: number }>(
				'/media/archive-all',
				{ method: 'POST' }
			);
			archiveAllOpen = false;
			error = `Arsip massal selesai: ${result.archived} media diarsipkan, ${result.pending_publications} publikasi menunggu terdampak.`;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}

	function openDetail(row: IntelligenceRow) {
		detailTarget = row;
		detailAudit = null;
		detailLoading = true;
		void platformRequest<{ items: AuditItem[] }>(`/media/${row.id}/audit`)
			.then((data) => (detailAudit = data.items ?? []))
			.catch(() => (detailAudit = []))
			.finally(() => (detailLoading = false));
	}

	const visibleRows = $derived(rows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
	const isDone = $derived(error.includes('selesai'));
</script>

{#snippet intelligenceMediaCell(row: IntelligenceRow)}
	<div>
		<p class="font-medium">{row.name}</p>
		<p class="text-xs text-muted-foreground">{row.slug ?? row.id.slice(0, 8)}</p>
	</div>
{/snippet}

{#snippet intelligenceCategoryCell(row: IntelligenceRow)}
	<span class="text-muted-foreground">{categoryText(row)}</span>
{/snippet}

{#snippet healthCell(row: IntelligenceRow)}
	<Badge tone={scoreTone(row.health_score)} label={row.health_score != null ? `${row.health_score}/100` : 'belum diskor'} />
{/snippet}

{#snippet successCell(row: IntelligenceRow)}
	{#if row.health_breakdown}
		<span class="text-xs">
			{row.health_breakdown.published ?? 0} / {row.health_breakdown.rejected ?? 0}
		</span>
	{:else}
		<span class="text-xs text-muted-foreground">—</span>
	{/if}
{/snippet}

{#snippet publishTimeCell(row: IntelligenceRow)}
	{#if row.health_breakdown?.avg_publish_hours != null}
		<span class="text-xs">{row.health_breakdown.avg_publish_hours} jam</span>
	{:else}
		<span class="text-xs text-muted-foreground">—</span>
	{/if}
{/snippet}

{#snippet trafficCell(row: IntelligenceRow)}
	<span class="text-xs">{compactNumber(row.traffic_estimate)}/bln</span>
{/snippet}

{#snippet statusCell(row: IntelligenceRow)}
	<div class="flex items-center gap-2">
		<Badge tone={row.status === 'active' ? 'success' : 'neutral'} value={row.status} />
		{#if row.warnings.length > 0}
			<span title={row.warnings.join(', ')}><Badge tone="warning" label={`⚠ ${row.warnings.length}`} /></span>
		{/if}
	</div>
{/snippet}

{#snippet intelligenceActions(row: IntelligenceRow)}
	<div class="flex items-center gap-2">
		{#if row.status === 'archived'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				disabled={busy}
				onclick={() => (restoreTarget = row)}
			>
				<RefreshCw size={13} /> Pulihkan
			</button>
		{:else}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				disabled={busy}
				onclick={() => void recomputeOne(row.id)}>Hitung</button
			>
		{/if}
		<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => openDetail(row)}>{tr("Detail")}</button>
	</div>
{/snippet}

{#if loading && rows.length === 0 && !error}
	<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat data intelligence media…")}</p>
{:else if rows.length === 0 && isAccessError(error)}
	<div class="app-card mt-6 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke data intelligence media.")}</p>
	</div>
{:else}
	<div class="mt-4 flex flex-wrap items-center gap-2">
		{#if summary}<Badge tone="accent" value={`${summary.scored}/${summary.total} diskor`} />{/if}
		<div class="flex-1"></div>
		{#if summary && summary.archived > 0}
			<button
				class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
				style="color: var(--success);"
				disabled={busy}
				onclick={() => (restoreAllOpen = true)}
			>
				<RefreshCw size={15} /> Pulihkan semua ({summary.archived})
			</button>
		{/if}
		<button
			class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
			style="color: var(--destructive);"
			disabled={busy}
			onclick={openArchiveAll}
		>
			<Archive size={15} /> Arsipkan semua
		</button>
		<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" disabled={busy} onclick={recompute}>
			<Activity size={15} /> {busy ? 'Memperbarui…' : 'Recompute massal'}
		</button>
	</div>

	{#if error}
		<div
			class="app-card mt-3 px-4 py-2.5 text-sm"
			style="color: {isDone ? 'var(--success)' : 'var(--destructive)'};">{error}</div
		>
	{/if}

	{#if summary}
		<div class="mt-4 flex flex-wrap gap-3">
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-sm text-muted-foreground">{tr("Total media")}</p>
				<p class="text-2xl font-bold">{summary.total}</p>
			</div>
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-sm text-muted-foreground">Sudah diskor</p>
				<p class="text-2xl font-bold">{summary.scored}</p>
			</div>
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-sm text-muted-foreground">Health rendah (&lt;40)</p>
				<p class="text-2xl font-bold" style="color: {summary.unhealthy > 0 ? 'var(--destructive)' : 'var(--success)'};">
					{summary.unhealthy}
				</p>
			</div>
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-sm text-muted-foreground">Perlu perhatian</p>
				<p class="text-2xl font-bold" style="color: {summary.with_warnings > 0 ? 'var(--warning)' : 'var(--success)'};">
					{summary.with_warnings}
				</p>
			</div>
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-sm text-muted-foreground">Terarsip</p>
				<p class="text-2xl font-bold" style="color: {summary.archived > 0 ? 'var(--muted-foreground)' : 'var(--success)'};">
					{summary.archived}
				</p>
			</div>
		</div>
	{/if}

	<form class="app-card mt-4 flex flex-wrap items-end gap-3 p-3" onsubmit={(e) => e.preventDefault()}>
		<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
			<span class="text-xs font-medium text-muted-foreground">{tr("Cari")}</span>
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr("Nama / kategori media")}
				bind:value={search}
				onkeydown={(e) => {
					if (e.key === 'Enter') void load(search, status, includeArchived);
				}}
			/>
		</label>
		<label class="block w-[200px]">
			<span class="text-xs font-medium text-muted-foreground">Status</span>
			<Select
				ariaLabel="Status"
				minWidth={200}
				align="end"
				value={status}
				onchange={(v) => void load(search, v, includeArchived)}
				options={[
					{ value: '', label: tr('Semua') },
					{ value: 'active', label: 'Active' },
					{ value: 'draft', label: 'Draft' },
					{ value: 'inactive', label: 'Inactive' },
					{ value: 'archived', label: 'Archived' }
				]}
			/>
		</label>
		<label class="flex items-center gap-1 pb-2 text-xs text-muted-foreground">
			<input
				type="checkbox"
				bind:checked={includeArchived}
				onchange={(e) => void load(search, status, e.currentTarget.checked)}
			/>{tr("Sertakan media terarsip")}</label>
		<button
			type="button"
			class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
			onclick={() => void load(search, status, includeArchived)}>Terapkan</button
		>
	</form>

	<DataTable
		columns={[
			{ key: 'name', label: tr('Media'), render: intelligenceMediaCell },
			{ key: 'category', label: 'Kategori', render: intelligenceCategoryCell },
			{ key: 'health', label: 'Health', render: healthCell },
			{ key: 'success', label: 'Success / Tolak', render: successCell },
			{ key: 'publish_time', label: 'Publish time', render: publishTimeCell },
			{ key: 'traffic', label: 'Trafik', render: trafficCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'actions', label: '', render: intelligenceActions }
		]}
		rows={visibleRows}
		rowKey={(row: IntelligenceRow) => row.id}
		{loading}
		empty={tr("Tidak ada media yang cocok.")}
		minWidth="1000px"
	/>

	<Pagination
		total={rows.length}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>
{/if}

{#if detailTarget}
	<Modal title={detailTarget.name} maxWidth="lg" onClose={() => (detailTarget = null)}>
		<p class="text-sm text-muted-foreground">{tr("Breakdown health media —")} {detailTarget.status}</p>
		{#if detailTarget.health_breakdown}
			<div class="mt-3 flex flex-wrap gap-4">
				{#each [['Availability', detailTarget.health_breakdown.availability, 15], ['Success rate', detailTarget.health_breakdown.success_rate, 30], ['Publish time', detailTarget.health_breakdown.publish_time, 35], ['Rejection', detailTarget.health_breakdown.rejection_rate, 20]] as [label, value, max] (label)}
					<div>
						<p class="text-sm text-muted-foreground">{label}</p>
						<p class="text-xl font-bold">{Number(value ?? 0)}/{Number(max)}</p>
					</div>
				{/each}
				<div>
					<p class="text-sm text-muted-foreground">{tr("Terbit / Tolak / Gagal")}</p>
					<p class="text-xl font-bold">
						{detailTarget.health_breakdown.published ?? 0} / {detailTarget.health_breakdown.rejected ?? 0} /
						{detailTarget.health_breakdown.failed ?? 0}
					</p>
				</div>
			</div>
		{:else}
			<p class="mt-3 text-sm text-muted-foreground">{tr("Belum ada breakdown — klik Hitung untuk menghitung health score.")}</p>
		{/if}
		{#if detailTarget.warnings.length > 0}
			<div class="mt-3 flex flex-wrap gap-2">
				{#each detailTarget.warnings as w (w)}<Badge tone="warning" label={w} />{/each}
			</div>
		{/if}
		<div class="mt-4 border-t pt-3">
			<p class="mb-2 text-sm font-medium text-muted-foreground">{tr("Riwayat audit")}</p>
			{#if detailLoading}
				<p class="text-xs text-muted-foreground">{tr("Memuat riwayat audit…")}</p>
			{:else if detailAudit === null}
				<p class="text-xs text-muted-foreground">—</p>
			{:else if detailAudit.length === 0}
				<p class="text-xs text-muted-foreground">{tr("Belum ada aktivitas audit untuk media ini.")}</p>
			{:else}
				<div class="max-h-60 space-y-2 overflow-y-auto pr-1">
					{#each detailAudit as a (a.id)}
						<div class="flex flex-wrap items-center justify-between gap-3">
							<Badge tone={auditTone(a.action)} label={AUDIT_LABEL[a.action] ?? a.action} />
							<span class="text-xs text-muted-foreground">{a.actor_email ?? '—'}</span>
							<span class="text-xs text-muted-foreground">{fmtDateTime(a.created_at)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</Modal>
{/if}

{#if restoreAllOpen}
	<Modal title={tr("Pulihkan semua media terarsip")} maxWidth="md" onClose={() => (restoreAllOpen = false)}>
		<p class="text-sm text-muted-foreground">
			{`${summary?.archived ?? 0} media terarsip akan dikembalikan aktif sekaligus — masing-masing dengan satu catatan audit media.restored.`}
		</p>
		<div
			class="mt-3 rounded-lg px-3 py-2 text-xs"
			style="background-color: var(--warning-soft); color: var(--warning);"
		>{tr("Publikasi yang masih menunggu akan tetap lanjut diproses, tetapi notifikasi persetujuan yang sudah")}<strong>kedaluwarsa tidak kembali aktif otomatis</strong>.
		</div>
		<div class="mt-4 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" onclick={() => (restoreAllOpen = false)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--success);"
				disabled={busy}
				onclick={restoreAll}>{busy ? 'Memulihkan…' : 'Pulihkan semua'}</button
			>
		</div>
	</Modal>
{/if}

{#if archiveAllOpen}
	<Modal title={tr("Arsipkan semua media")} maxWidth="lg" onClose={() => (archiveAllOpen = false)}>
		<p class="text-sm text-muted-foreground">
			{`${archiveImpact?.archivable ?? 0} media non-arsip akan diarsipkan sekaligus — masing-masing dengan satu catatan audit media.archived.`}
		</p>
		{#if archiveImpact?.blocked}
			<div
				class="mt-3 rounded-lg px-3 py-2 text-xs"
				style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);"
			>
				<strong>{tr("Arsip massal ditolak.")}</strong>{tr("Masih ada publikasi yang sedang berjalan:")}<strong
					>{archiveImpact.blocking_statuses
						.map((s) => `${archiveImpact?.by_status[s] ?? 0} ${PUB_STATUS_LABEL[s] ?? s}`)
						.join(', ')}</strong
				>. Tunggu hingga publikasi tersebut selesai, lalu coba lagi.
			</div>
		{:else}
			<div
				class="mt-3 rounded-lg px-3 py-2 text-xs"
				style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);"
			>
				<strong>{archiveImpact?.pending_publications ?? 0} {tr("publikasi yang masih menunggu")}</strong>{tr("akan terdampak: media tidak menerima publikasi baru dan notifikasi persetujuan mitra ditandai")}<strong>kedaluwarsa</strong>{tr(". Riwayat publikasi tetap tersimpan dan media bisa dipulihkan kembali.")}</div>
		{/if}
		{#if archiveImpact && archiveImpact.by_media.length > 0}
			<div class="mt-3 border-t pt-3">
				<p class="mb-2 text-sm font-medium">{tr("Publikasi aktif per media")}</p>
				<div class="max-h-56 space-y-2 overflow-y-auto pr-1">
					{#each archiveImpact.by_media as media (media.id)}
						<div class="flex flex-wrap items-center justify-between gap-2">
							<p class="max-w-[200px] truncate text-sm">{media.name}</p>
							<div class="flex flex-wrap items-center gap-1">
								{#each Object.entries(media.counts) as [st, count] (st)}
									<Badge tone={st === 'waiting_partner' ? 'warning' : 'danger'} label={`${count} ${PUB_STATUS_LABEL[st] ?? st}`} />
								{/each}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}
		<div class="mt-4 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" onclick={() => (archiveAllOpen = false)}>{tr("Batal")}</button>
			{#if !archiveImpact?.blocked}
				<button
					class="app-btn app-btn-primary"
					style="background-color: var(--destructive);"
					disabled={busy || (archiveImpact?.archivable ?? 0) === 0}
					onclick={archiveAll}>{busy ? 'Mengarsipkan…' : `Arsipkan ${archiveImpact?.archivable ?? 0} media`}</button
				>
			{/if}
		</div>
	</Modal>
{/if}

{#if restoreTarget}
	<Modal title={tr("Pulihkan media")} maxWidth="md" onClose={() => (restoreTarget = null)}>
		<p class="text-sm text-muted-foreground">
			{'Media "' + (restoreTarget?.name ?? '') + '" akan dikembalikan aktif dan dapat menerima publikasi baru kembali.'}
		</p>
		<div
			class="mt-3 rounded-lg px-3 py-2 text-xs"
			style="background-color: var(--warning-soft); color: var(--warning);"
		>{tr("Publikasi yang masih menunggu akan tetap lanjut diproses, tetapi notifikasi persetujuan yang sudah")}<strong>kedaluwarsa tidak kembali aktif otomatis</strong>.
		</div>
		<div class="mt-4 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" onclick={() => (restoreTarget = null)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--success);"
				disabled={restoreBusy}
				onclick={restoreMedia}>{restoreBusy ? 'Memulihkan…' : tr('Pulihkan media')}</button
			>
		</div>
	</Modal>
{/if}
