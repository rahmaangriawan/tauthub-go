<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Review Verifikasi Dewan Pers — setujui/tolak pengajuan nomor sertifikat dari Mitra.
	// Port apps/web/src/app/admin/dewan-pers-reviews/page.tsx.
	import { onMount } from 'svelte';
	import { Check, X } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type ReviewRow = {
		id: string;
		media_id: string;
		media_name: string | null;
		media_domain: string | null;
		partner_id: string;
		partner_name: string | null;
		partner_public_id: string | null;
		certificate_no: string;
		status: 'pending' | 'verified' | 'rejected' | 'conflict' | 'superseded';
		rejection_reason: string | null;
		conflict_with: string | null;
		conflict_media_name: string | null;
		submitted_at: string | null;
		reviewed_at: string | null;
	};

	const STATUS_META: Record<
		string,
		{ label: string; tone: 'neutral' | 'accent' | 'success' | 'warning' | 'danger' }
	> = $derived({
		pending: { label: tr('Menunggu'), tone: 'warning' },
		verified: { label: 'Terverifikasi', tone: 'success' },
		rejected: { label: tr('Ditolak'), tone: 'danger' },
		conflict: { label: 'Konflik', tone: 'warning' },
		superseded: { label: 'Diganti', tone: 'neutral' }
	});

	let statusFilter = $state('all');
	let rows = $state<ReviewRow[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let page = $state(1);

	let reviewTarget = $state<ReviewRow | null>(null);
	let rejectMode = $state(false);
	let reason = $state('');
	let busy = $state(false);
	let dialogError = $state('');

	const tabs = ['all', 'pending', 'verified', 'rejected', 'conflict'];

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
		try {
			const filter = statusFilter === 'all' ? '' : `&status=${statusFilter}`;
			const res = await apiRequest(
				`/v1/platform/media/dewan-pers-reviews?limit=${PAGE_SIZE}&offset=${(page - 1) * PAGE_SIZE}${filter}`
			);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			rows = data.reviews ?? [];
			total = data.total ?? 0;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function openApprove(row: ReviewRow) {
		reviewTarget = row;
		rejectMode = false;
		reason = '';
		dialogError = '';
	}

	function openReject(row: ReviewRow) {
		reviewTarget = row;
		rejectMode = true;
		reason = '';
		dialogError = '';
	}

	async function submitReview(event: SubmitEvent) {
		event.preventDefault();
		if (!reviewTarget) return;
		busy = true;
		dialogError = '';
		try {
			const body: Record<string, unknown> = { decision: rejectMode ? 'reject' : 'approve' };
			if (rejectMode) {
				if (!reason.trim()) {
					dialogError = 'Alasan penolakan wajib diisi.';
					return;
				}
				body.reason = reason.trim();
			}
			const res = await apiRequest(`/v1/platform/media/dewan-pers-reviews/${reviewTarget.id}`, {
				method: 'POST',
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				dialogError = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			reviewTarget = null;
			notice = rejectMode ? 'Verifikasi ditolak.' : 'Verifikasi disetujui.';
			await load();
		} finally {
			busy = false;
		}
	}
</script>

{#snippet mediaCell(row: ReviewRow)}
	<div class="flex flex-col gap-1">
		<span class="font-medium">{row.media_name ?? row.media_id}</span>
		{#if row.media_domain}<span class="text-xs text-muted-foreground">{row.media_domain}</span>{/if}
	</div>
{/snippet}

{#snippet partnerCell(row: ReviewRow)}
	<span>{row.partner_name ?? row.partner_id.slice(0, 8)}</span>
{/snippet}

{#snippet certCell(row: ReviewRow)}
	<code class="text-xs">{row.certificate_no}</code>
{/snippet}

{#snippet reviewStatusCell(row: ReviewRow)}
	{@const meta = STATUS_META[row.status] ?? STATUS_META.pending}
	<div class="flex flex-col items-start gap-1">
		<Badge tone={meta.tone} label={meta.label} />
		{#if row.status === 'rejected' && row.rejection_reason}
			<p class="max-w-[220px] text-xs" style="color: var(--destructive);">{row.rejection_reason}</p>
		{/if}
		{#if row.status === 'conflict' && row.conflict_media_name}
			<p class="text-xs" style="color: var(--warning);">Konflik dgn: {row.conflict_media_name}</p>
		{/if}
	</div>
{/snippet}

{#snippet submittedCell(row: ReviewRow)}
	<span class="text-xs text-muted-foreground">{fmtDateTime(row.submitted_at)}</span>
{/snippet}

{#snippet reviewedCell(row: ReviewRow)}
	<span class="text-xs text-muted-foreground">{fmtDateTime(row.reviewed_at)}</span>
{/snippet}

{#snippet reviewActions(row: ReviewRow)}
	{#if row.status === 'pending' || row.status === 'conflict'}
		<div class="flex justify-end gap-2">
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				onclick={() => openApprove(row)}><Check size={13} />Setujui</button
			>
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				onclick={() => openReject(row)}><X size={13} />Tolak</button
			>
		</div>
	{/if}
{/snippet}

<section>
	<PageHeader
		title="Review Verifikasi Dewan Pers"
		description={tr("Setujui atau tolak pengajuan nomor sertifikat Dewan Pers dari Mitra. Status hanya berubah setelah review Admin — format valid bukan berarti otomatis terverifikasi.")}
	>
		<Badge tone="accent" value={`${total} pengajuan`} />
	</PageHeader>

	<Feedback {loading} {error} {notice} />

	{#if rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke review Dewan Pers.")}</p>
		</div>
	{:else}
		<div class="mt-3 flex flex-wrap gap-2">
			{#each tabs as t (t)}
				<button
					type="button"
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={statusFilter === t}
					class:app-btn-soft={statusFilter !== t}
					onclick={() => {
						statusFilter = t;
						page = 1;
						void load();
					}}
				>
					{t === 'all'
						? tr('Semua')
						: (STATUS_META[t]?.label ?? t)}
				</button>
			{/each}
		</div>

		<DataTable
			columns={[
				{ key: 'media', label: tr('Media'), render: mediaCell },
				{ key: 'partner', label: tr('Mitra'), render: partnerCell },
				{ key: 'cert', label: 'Nomor Sertifikat', render: certCell },
				{ key: 'status', label: 'Status', render: reviewStatusCell },
				{ key: 'submitted', label: 'Pengajuan', render: submittedCell },
				{ key: 'reviewed', label: 'Review', render: reviewedCell },
				{ key: 'actions', label: 'Aksi', align: 'right' as const, render: reviewActions }
			]}
			rows={rows}
			rowKey={(row: ReviewRow) => row.id}
			{loading}
			empty={tr("Tidak ada pengajuan verifikasi Dewan Pers.")}
			minWidth="1100px"
		/>

		<Pagination
			total={total}
			limit={PAGE_SIZE}
			offset={(page - 1) * PAGE_SIZE}
			onChange={(next) => {
				page = next / PAGE_SIZE + 1;
				void load();
			}}
		/>
	{/if}
</section>

{#if reviewTarget}
	<Modal
		title={rejectMode ? 'Tolak Verifikasi Dewan Pers' : 'Setujui Verifikasi Dewan Pers'}
		maxWidth="md"
		onClose={() => (reviewTarget = null)}
	>
		<p class="text-sm text-muted-foreground">
			{reviewTarget.media_name ?? reviewTarget.media_id} — {reviewTarget.certificate_no}
		</p>
		<form class="mt-3 space-y-3" onsubmit={submitReview}>
			{#if rejectMode}
				<label class="block">
					<span class="text-xs font-bold text-muted-foreground">Alasan Penolakan</span>
					<textarea
						class="app-input mt-1 w-full"
						rows="4"
						placeholder={tr("Alasan penolakan...")}
						bind:value={reason}
					></textarea>
					<span class="mt-1 block text-xs text-muted-foreground">{tr("Wajib diisi — alasan ini terlihat oleh Mitra.")}</span>
				</label>
			{:else}
				<div
					class="flex items-start gap-2 rounded-lg px-3 py-2 text-sm"
					style="background-color: var(--success-soft); color: var(--success);"
				>
					<Check size={16} class="mt-0.5 shrink-0" />{tr("Media akan ditandai Terverifikasi Dewan Pers dan Mitra mendapat notifikasi.")}</div>
			{/if}
			{#if dialogError}
				<p class="text-sm" style="color: var(--destructive);">{dialogError}</p>
			{/if}
			<div class="flex justify-end gap-2 border-t pt-4">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (reviewTarget = null)}>{tr("Batal")}</button>
				<button
					type="submit"
					class="app-btn app-btn-primary"
					style={rejectMode ? 'background-color: var(--destructive);' : undefined}
					disabled={busy}
					>{busy ? 'Menyimpan…' : rejectMode ? 'Tolak Verifikasi' : 'Setujui Verifikasi'}</button
				>
			</div>
		</form>
	</Modal>
{/if}
