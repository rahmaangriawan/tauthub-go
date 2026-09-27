<script lang="ts">
	// Laporan topup gagal — port app/admin/credit-reports/page.tsx: klien
	// melaporkan topup gagal (pembayaran sukses tapi saldo TC tidak masuk).
	// Admin review: setujui (tambah TC manual) atau tolak (wajib catatan).
	import { onMount } from 'svelte';
	import { CircleCheck, Coins, TriangleAlert, XCircle } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type CreditReport = {
		id: string; brand_id: string; brand_name: string | null; user_id: string;
		user_email: string | null; user_display_name: string | null; claimed_tc: number;
		amount_idr: number | null; payment_reference: string | null; description: string | null;
		status: 'pending' | 'approved' | 'rejected'; review_note: string | null;
		reviewed_at: string | null; created_at: string; available_tc: number;
	};

	const PAGE_SIZE = 20;
	const STATUS_LABEL: Record<CreditReport['status'], string> = {
		pending: 'Menunggu', approved: 'Disetujui', rejected: 'Ditolak'
	};
	const STATUS_TONE: Record<CreditReport['status'], 'warning' | 'success' | 'danger'> = {
		pending: 'warning', approved: 'success', rejected: 'danger'
	};

	let rows = $state<CreditReport[]>([]);
	let total = $state(0);
	let error = $state('');
	let loading = $state(true);
	let statusFilter = $state('');
	let review = $state<CreditReport | null>(null);
	let reviewNote = $state('');
	let reviewError = $state('');
	let busy = $state(false);
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

	async function load() {
		loading = true;
		error = '';
		const params = new URLSearchParams({
			limit: String(PAGE_SIZE),
			offset: String((pageNum - 1) * PAGE_SIZE)
		});
		if (statusFilter) params.set('status', statusFilter);
		const res = await apiRequest(`/v1/platform/operations/credit-reports?${params.toString()}`);
		if (res.ok) {
			const data = await res.json();
			rows = data.reports ?? [];
			total = data.total ?? 0;
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	function setStatusFilter(value: string) {
		statusFilter = value;
		pageNum = 1;
		void load();
	}

	async function submitReview(action: 'approve' | 'reject') {
		if (!review) return;
		if (action === 'reject' && reviewNote.trim().length < 5) {
			reviewError = 'Alasan penolakan wajib diisi (minimal 5 karakter).';
			return;
		}
		busy = true;
		reviewError = '';
		error = '';
		const res = await apiRequest(`/v1/platform/operations/credit-reports/${review.id}/review`, {
			method: 'POST',
			body: JSON.stringify({ action, note: reviewNote.trim() || null })
		});
		if (!res.ok) {
			reviewError = await platformError(res);
			busy = false;
			return;
		}
		review = null;
		reviewNote = '';
		busy = false;
		await load();
	}

	const denied = $derived(Boolean(!loading && rows.length === 0 && error.includes('Akses platform')));
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Laporan Topup Gagal"
		description="Klien melaporkan topup yang gagal (pembayaran sukses tapi saldo TC tidak masuk). Setujui untuk menambahkan TC manual ke wallet brand."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{total} laporan</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke laporan topup.</p>
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-2">
			<select class="app-input !w-auto" value={statusFilter} onchange={(e) => setStatusFilter((e.currentTarget as HTMLSelectElement).value)}>
				<option value="">Semua status</option>
				<option value="pending">Menunggu</option>
				<option value="approved">Disetujui</option>
				<option value="rejected">Ditolak</option>
			</select>
			{#if statusFilter}
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => setStatusFilter('')}>Bersihkan filter</button>
			{/if}
		</div>

		<Feedback {loading} {error} loadingText="Memuat laporan topup…" />

		<div class="app-card mt-3 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1080px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Klien</th>
						<th class="px-4 py-2.5 font-medium">Brand</th>
						<th class="px-4 py-2.5 font-medium">Klaim TC</th>
						<th class="px-4 py-2.5 font-medium">Referensi</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Dibuat</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p class="font-medium">{row.user_display_name ?? '—'}</p>
								<p class="text-xs text-muted-foreground">{row.user_email ?? ''}</p>
							</td>
							<td class="px-4 py-2.5">{row.brand_name ?? '—'}</td>
							<td class="px-4 py-2.5">
								<p class="font-medium">{fmtNum(row.claimed_tc)} TC</p>
								<p class="text-xs text-muted-foreground">{row.amount_idr != null ? fmtRp(row.amount_idr) : '—'}</p>
							</td>
							<td class="px-4 py-2.5">{row.payment_reference ?? '—'}</td>
							<td class="px-4 py-2.5"><Badge label={STATUS_LABEL[row.status]} tone={STATUS_TONE[row.status]} /></td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(row.created_at)}</td>
							<td class="px-4 py-2.5">
								{#if row.status === 'pending'}
									<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--accent); color: var(--accent-foreground);" type="button" onclick={() => { review = row; reviewNote = ''; reviewError = ''; }}>Review</button>
								{:else}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => { review = row; reviewNote = row.review_note ?? ''; reviewError = ''; }}>Detail</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Coins size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">Belum ada laporan topup gagal.</p>
				</div>
			{/if}
			<div class="px-4 pb-4">
				<Pagination
					total={total}
					limit={PAGE_SIZE}
					offset={(pageNum - 1) * PAGE_SIZE}
					onChange={(next) => { pageNum = Math.floor(next / PAGE_SIZE) + 1; void load(); }}
				/>
			</div>
		</div>
	{/if}
</section>

{#if review}
	<Modal
		title={review.status === 'pending' ? 'Review Laporan Topup' : 'Detail Laporan Topup'}
		label="Review laporan topup"
		maxWidth="lg"
		onClose={() => (review = null)}
	>
		<p class="text-sm text-muted-foreground">{review.user_display_name ?? 'Klien'} — {review.brand_name ?? 'Brand'}</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if review.status === 'pending' && reviewError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{reviewError}</div>
			{/if}
			<div>
				<p class="text-xs font-bold text-muted-foreground">Klaim</p>
				<p class="text-sm">{fmtNum(review.claimed_tc)} TC {review.amount_idr != null ? `(${fmtRp(review.amount_idr)})` : ''}</p>
			</div>
			{#if review.payment_reference}
				<div>
					<p class="text-xs font-bold text-muted-foreground">Referensi pembayaran</p>
					<p class="text-sm">{review.payment_reference}</p>
				</div>
			{/if}
			<div>
				<p class="text-xs font-bold text-muted-foreground">Deskripsi laporan</p>
				<p class="whitespace-pre-wrap text-sm">{review.description ?? '—'}</p>
			</div>
			<div>
				<p class="text-xs font-bold text-muted-foreground">Saldo brand saat ini</p>
				<p class="text-sm">{fmtNum(review.available_tc)} TC tersedia</p>
			</div>
			{#if review.status !== 'pending'}
				<div>
					<p class="text-xs font-bold text-muted-foreground">Status</p>
					<Badge label={STATUS_LABEL[review.status]} tone={STATUS_TONE[review.status]} />
				</div>
				{#if review.review_note}
					<div>
						<p class="text-xs font-bold text-muted-foreground">Catatan admin</p>
						<p class="whitespace-pre-wrap text-sm">{review.review_note}</p>
					</div>
				{/if}
			{:else}
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Catatan (wajib saat menolak)</span>
					<textarea
						class="app-input w-full"
						rows={3}
						placeholder="cth: Terverifikasi — TC ditambahkan. / Data tidak cocok dengan transaksi."
						value={reviewNote}
						oninput={(e) => { reviewNote = (e.currentTarget as HTMLTextAreaElement).value; reviewError = ''; }}
					></textarea>
					<span class="mt-1 block text-xs text-muted-foreground">Catatan ini tampil untuk admin &amp; riwayat laporan.</span>
				</label>
			{/if}
		</div>
		{#snippet footer()}
			{#if review?.status === 'pending'}
				<button class="app-btn app-btn-soft" type="button" onclick={() => (review = null)}>Batal</button>
				<button class="app-btn app-btn-soft" style="color: var(--destructive);" type="button" disabled={busy} onclick={() => void submitReview('reject')}>
					<XCircle size={15} />{busy ? 'Menyimpan…' : 'Tolak'}
				</button>
				<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void submitReview('approve')}>
					<CircleCheck size={15} />{busy ? 'Menyimpan…' : 'Setujui & Tambah TC'}
				</button>
			{/if}
		{/snippet}
	</Modal>
{/if}
