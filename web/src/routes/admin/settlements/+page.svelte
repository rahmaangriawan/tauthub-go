<script lang="ts">
	// Salin ke clipboard dengan umpan balik singkat pada ikon.
	let copiedKey = $state<string | null>(null);
	let copiedTimer: ReturnType<typeof setTimeout> | undefined;
	async function copyValue(key: string, value: string) {
		if (!value) return;
		try {
			await navigator.clipboard.writeText(value);
			copiedKey = key;
			clearTimeout(copiedTimer);
			copiedTimer = setTimeout(() => (copiedKey = null), 1500);
		} catch {
			copiedKey = null;
		}
	}
	import { tr } from '$lib/i18n/phrases';

	// Settlement — port app/admin/settlements/page.tsx: payout ke partner.
	// Create meng-agregasi earnings pending dalam periode dan menguncinya (Phase 6).
	// Review withdraw: tolak wajib alasan, bayar + nomor referensi & bukti transfer,
	// void. Plus pengaturan saldo minimal penarikan. Total kolom memakai NET
	// (setelah potongan transfer) agar sama dengan yang diterima mitra; detail
	// memuat rekening mitra untuk transfer manual.
	import { onMount } from 'svelte';
	import { Check, Copy, Landmark, TriangleAlert } from '@lucide/svelte';
	import SettlementIcon from '$lib/components/icons/settlement-icon.svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from '../_platform';

	type Settlement = {
		id: string; partner_id: string; partner_name: string | null; period_start: string;
		period_end: string; status: string; total_amount: number; notes: string | null;
		payout_method: string | null; transfer_fee_idr: number | null; net_amount_idr: number | null;
		invoice_number: string | null;
		bank_name: string | null; bank_account_number: string | null; bank_account_name: string | null;
		receipt_url: string | null; payment_reference: string | null;
		rejection_reason: string | null; paid_at: string | null; created_at: string;
	};
	type Partner = { id: string; name: string | null; email: string; status: string };

	const PAGE_SIZE = 20;
	const STATUS_TONE: Record<string, 'success' | 'neutral' | 'warning' | 'accent' | 'danger'> = {
		pending: 'warning', approved: 'accent', processing: 'neutral', paid: 'success', failed: 'danger'
	};

	// Jumlah yang harus ditransfer = net (setelah potongan transfer). Baris
	// backend selalu mengisi kedua nilai (baris lama dihitung runtime di API).
	function payoutOf(row: Settlement): number {
		return row.net_amount_idr ?? row.total_amount;
	}
	function feeOf(row: Settlement): number {
		return row.transfer_fee_idr ?? 0;
	}

	let rows = $state<Settlement[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let error = $state('');
	let loading = $state(true);
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({ partner_id: '', period_start: '', period_end: '', notes: '' });
	let pageNum = $state(1);

	// Tolak withdrawal
	let rejectTarget = $state<Settlement | null>(null);
	let rejectReason = $state('');
	let rejectBusy = $state(false);
	// Bayar + bukti
	let payTarget = $state<Settlement | null>(null);
	let payForm = $state({ payment_reference: '', receipt_url: '' });
	let payReceiptFile = $state<File | null>(null);
	let payBusy = $state(false);
	// Pengaturan penarikan
	let minimumWithdrawIdr = $state<number>(10000);
	let configBusy = $state(false);
	let configNotice = $state('');
	// Pencarian
	let search = $state('');
	let debouncedSearch = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	// Detail: rekening mitra + rincian potongan, untuk transfer manual.
	let detailTarget = $state<Settlement | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
			await loadConfig();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		const offset = (pageNum - 1) * PAGE_SIZE;
		const q = debouncedSearch ? `&q=${encodeURIComponent(debouncedSearch)}` : '';
		const [settleRes, partnerRes] = await Promise.all([
			apiRequest(`/v1/platform/settlements?limit=${PAGE_SIZE}&offset=${offset}${q}`),
			apiRequest('/v1/platform/partners?limit=500')
		]);
		if (settleRes.ok) {
			const data = await settleRes.json();
			rows = data.settlements ?? [];
			total = data.total ?? 0;
		} else {
			error = await platformError(settleRes);
		}
		if (partnerRes.ok) {
			const data = await partnerRes.json();
			partners = (data.partners ?? []).filter((p: Partner) => p.status === 'active');
		}
		loading = false;
	}

	async function loadConfig() {
		const res = await apiRequest('/v1/platform/settlements/config');
		if (res.ok) {
			const data = await res.json();
			minimumWithdrawIdr = data.minimum_withdraw_idr;
		}
		// config opsional — fallback ke default
	}

	function onSearch(value: string) {
		search = value;
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			debouncedSearch = value.trim();
			pageNum = 1;
			void load();
		}, 300);
	}

	function clearSearch() {
		search = '';
		debouncedSearch = '';
		pageNum = 1;
		void load();
	}

	async function saveConfig() {
		configBusy = true;
		configNotice = '';
		error = '';
		const value = Number(minimumWithdrawIdr);
		if (!Number.isFinite(value) || value < 1) {
			error = tr('Saldo minimal harus angka positif.');
			configBusy = false;
			return;
		}
		const res = await apiRequest('/v1/platform/settlements/config', {
			method: 'PUT',
			body: JSON.stringify({ minimum_withdraw_idr: Math.round(value) })
		});
		if (!res.ok) {
			error = await platformError(res);
			configBusy = false;
			return;
		}
		const data = await res.json();
		minimumWithdrawIdr = data.minimum_withdraw_idr;
		configNotice = tr('Pengaturan penarikan dana berhasil disimpan.');
		configBusy = false;
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const res = await apiRequest('/v1/platform/settlements', {
			method: 'POST',
			body: JSON.stringify({
				partner_id: form.partner_id,
				period_start: new Date(`${form.period_start}T00:00:00.000Z`).toISOString(),
				period_end: new Date(`${form.period_end}T23:59:59.999Z`).toISOString(),
				notes: form.notes.trim() || null
			})
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		createOpen = false;
		busy = false;
		await load();
	}

	/** Setujui pengajuan lalu langsung buka popup kirim bukti pembayaran. */
	async function approveAndPay(settlement: Settlement) {
		error = '';
		const res = await apiRequest(`/v1/platform/settlements/${settlement.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status: 'approved' })
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		payTarget = { ...settlement, status: 'approved' };
		payForm = { payment_reference: settlement.payment_reference ?? '', receipt_url: settlement.receipt_url ?? '' };
		payReceiptFile = null;
		await load();
	}


	async function rejectSettlement() {
		if (!rejectTarget) return;
		const reason = rejectReason.trim();
		if (!reason) {
			error = 'Alasan penolakan wajib diisi.';
			return;
		}
		rejectBusy = true;
		error = '';
		const res = await apiRequest(`/v1/platform/settlements/${rejectTarget.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status: 'failed', rejection_reason: reason })
		});
		if (!res.ok) {
			error = await platformError(res);
			rejectBusy = false;
			return;
		}
		rejectTarget = null;
		rejectReason = '';
		rejectBusy = false;
		await load();
	}

	async function markPaid() {
		if (!payTarget) return;
		payBusy = true;
		error = '';
		try {
			let receiptUrl: string | null = payForm.receipt_url.trim() || null;
			// Upload bukti dulu kalau admin memilih file.
			if (payReceiptFile) {
				const fd = new FormData();
				fd.append('file', payReceiptFile);
				const upRes = await apiRequest(`/v1/platform/settlements/${payTarget.id}/receipt`, { method: 'POST', body: fd });
				if (!upRes.ok) {
					error = await platformError(upRes);
					return;
				}
				const uploaded = await upRes.json();
				receiptUrl = uploaded.receipt_url ?? null;
			}
			const res = await apiRequest(`/v1/platform/settlements/${payTarget.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					status: 'paid',
					payment_reference: payForm.payment_reference.trim() || null,
					receipt_url: receiptUrl
				})
			});
			if (!res.ok) {
				error = await platformError(res);
				return;
			}
			payTarget = null;
			payForm = { payment_reference: '', receipt_url: '' };
			payReceiptFile = null;
			await load();
		} finally {
			payBusy = false;
		}
	}

	async function voidSettlement(settlement: Settlement) {
		error = '';
		const res = await apiRequest(`/v1/platform/settlements/${settlement.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}

	const denied = $derived(Boolean(!loading && rows.length === 0 && isAccessDenied(error)));
</script>

<section>
	<PageHeader
		title="Settlements"
		description="Payout ke partner: create meng-agregasi earnings pending dalam periode dan menguncinya (Phase 6)."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} settlement</span>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><SettlementIcon size={16} />{tr("Buat settlement")}</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke settlement.")}</p>
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-2">
			<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5" style="max-width: 380px;">
				<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari nama partner…")} aria-label={tr("Cari settlement")} value={search} oninput={(e) => onSearch((e.currentTarget as HTMLInputElement).value)} />
			</label>
			{#if debouncedSearch}
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={clearSearch}>Bersihkan pencarian</button>
			{/if}
		</div>

		<Feedback {loading} {error} notice={configNotice} loadingText="Memuat settlement…" />

		<div class="app-card mt-4 p-5">
			<div class="flex flex-wrap items-start justify-between gap-3">
				<div class="min-w-[220px] flex-1">
					<h2 class="text-sm font-semibold">{tr("Pengaturan Penarikan Dana")}</h2>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Saldo minimal yang harus dimiliki Mitra untuk dapat mengajukan penarikan. Nilai ini ditampilkan di dashboard Mitra dan divalidasi di server.")}</p>
				</div>
				<div class="flex flex-wrap items-end gap-2">
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Saldo Minimal Penarikan (IDR)")}</span>
						<input
							class="app-input !w-[200px]"
							type="number"
							min="1"
							inputmode="numeric"
							value={minimumWithdrawIdr}
							oninput={(e) => (minimumWithdrawIdr = Number((e.currentTarget as HTMLInputElement).value))}
						/>
					</label>
					<button class="app-btn app-btn-primary" type="button" disabled={configBusy} onclick={() => void saveConfig()}>
						{configBusy ? 'Menyimpan…' : tr('Simpan Pengaturan')}
					</button>
				</div>
			</div>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 940px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">{tr('Mitra')}</th>
						<th class="px-4 py-2.5 font-medium">Periode</th>
					<th class="px-4 py-2.5 font-medium">Total (neto)</th>
					<th class="px-4 py-2.5 font-medium">Status</th>						<th class="px-4 py-2.5 font-medium">Dibayar</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<SettlementIcon size={16} class="shrink-0 text-muted-foreground" />
									<div>
										<p class="font-medium">{row.partner_name ?? row.partner_id.slice(0, 8)}</p>
										<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5">{fmtDate(row.period_start)} — {fmtDate(row.period_end)}</td>
							<td class="px-4 py-2.5 tabular-nums">
								<span class="font-semibold">{fmtRp(payoutOf(row))}</span>
								{#if feeOf(row) > 0}
									<span class="mt-0.5 block text-[11px] text-muted-foreground">Bruto {fmtRp(row.total_amount)} − potongan {fmtRp(feeOf(row))}</span>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								<Badge label={row.status} tone={STATUS_TONE[row.status] ?? 'neutral'} />
								{#if row.status === 'failed' && row.rejection_reason}
									<p class="mt-1 text-xs" style="color: var(--destructive);">Alasan: {row.rejection_reason}</p>
								{/if}
								{#if row.payment_reference}
									<p class="mt-1 text-xs text-muted-foreground">Ref: {row.payment_reference}</p>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{row.paid_at ? fmtDate(row.paid_at) : '—'}</td>
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-1.5">
									<button
										class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
										type="button"
										onclick={() => { detailTarget = row; error = ''; }}
									>Detail</button>
									{#if row.status === 'pending'}
										<!-- Setujui langsung membuka popup kirim bukti: admin tidak
										     perlu mampir ke state 'processing' dulu. -->
										<button
											class="app-btn !min-h-7 !px-2.5 !text-xs"
											style="background-color: var(--success-soft); color: var(--success);"
											type="button"
											onclick={() => void approveAndPay(row)}
										>{tr('Setujui')}</button>
									{/if}
									{#if row.status === 'approved' || row.status === 'processing'}
										<button
											class="app-btn !min-h-7 !px-2.5 !text-xs"
											style="background-color: var(--success-soft); color: var(--success);"
											type="button"
											onclick={() => { payTarget = row; payForm = { payment_reference: row.payment_reference ?? '', receipt_url: row.receipt_url ?? '' }; payReceiptFile = null; error = ''; }}
										>Bayar + Bukti</button>
									{/if}
									{#if ['pending', 'approved'].includes(row.status)}
										<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);" type="button" onclick={() => { rejectTarget = row; rejectReason = ''; error = ''; }}>Tolak</button>
									{/if}
									{#if ['pending', 'approved'].includes(row.status)}
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void voidSettlement(row)}>Void</button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<SettlementIcon size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr("Belum ada settlement.")}</p>
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

{#if createOpen}
	<Modal title={tr("Buat settlement")} label={tr("Buat settlement")} maxWidth="lg" onClose={() => (createOpen = false)}>
		<form class="flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr('Mitra')}</span>
				<Select
					ariaLabel={tr('Mitra')}
					minWidth={260}
					value={form.partner_id}
					onchange={(v) => (form.partner_id = v)}
					options={[
						{ value: '', label: tr('Pilih partner') },
						...partners.map((p) => ({ value: p.id, label: p.name ?? p.email }))
					]}
				/>
			</label>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Periode mulai</span>
					<input class="app-input w-full" type="date" required bind:value={form.period_start} />
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Periode selesai</span>
					<input class="app-input w-full" type="date" required bind:value={form.period_end} />
				</label>
			</div>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Catatan (opsional)</span>
				<input class="app-input w-full" bind:value={form.notes} />
			</label>
			{#if error}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{error}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Buat settlement')}</button>
			</div>
		</form>
	</Modal>
{/if}

{#if rejectTarget}
	<Modal title="Tolak Withdrawal" label="Tolak withdrawal" maxWidth="md" onClose={() => (rejectTarget = null)}>		<p class="text-sm text-muted-foreground">
			Tolak penarikan <strong>{fmtRp(rejectTarget.total_amount)}</strong>{tr("? Saldo earnings akan dikembalikan ke mitra dan alasan ini terlihat olehnya.")}</p>
		<label class="mt-3 block">
			<span class="mb-1 block text-xs font-semibold text-muted-foreground">Alasan Penolakan *</span>
			<textarea
				class="app-input w-full"
				rows={4}
				placeholder={tr("Contoh: Nominal tidak sesuai dengan saldo earnings, mohon ajukan ulang.")}
				bind:value={rejectReason}
			></textarea>
		</label>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={rejectBusy} onclick={() => (rejectTarget = null)}>Batalkan</button>
			<button class="app-btn app-btn-primary" type="button" disabled={rejectBusy} onclick={() => void rejectSettlement()}>
				{rejectBusy ? 'Menyimpan…' : 'Tolak Withdrawal'}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if payTarget}
	<Modal title="Konfirmasi Pembayaran" label="Konfirmasi pembayaran" maxWidth="md" onClose={() => (payTarget = null)}>
		<p class="text-sm text-muted-foreground">
			Tandai penarikan <strong>{fmtRp(payoutOf(payTarget))}</strong>{tr("sebagai PAID setelah transfer manual selesai ke rekening di Detail.")}
			{#if feeOf(payTarget) > 0}<span class="block mt-1 text-xs">Bruto {fmtRp(payTarget.total_amount)} − potongan {fmtRp(feeOf(payTarget))} = neto {fmtRp(payoutOf(payTarget))}.</span>{/if}</p>
		<div class="mt-3 flex flex-col gap-3">
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Nomor Referensi Transfer (opsional)</span>
				<input class="app-input w-full" placeholder={tr("Contoh: BRIVA/1234567890")} bind:value={payForm.payment_reference} />
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Bukti Pembayaran (opsional)</span>
				<input
					class="app-input w-full"
					type="file"
					accept="image/png,image/jpeg,application/pdf"
					onchange={(e) => (payReceiptFile = (e.currentTarget as HTMLInputElement).files?.[0] ?? null)}
				/>
				{#if payReceiptFile}
					<span class="mt-1 block text-xs text-muted-foreground">{payReceiptFile.name} ({Math.round(payReceiptFile.size / 1024)} KB)</span>
				{:else if payForm.receipt_url}
					<span class="mt-1 block text-xs text-muted-foreground">Bukti existing: {payForm.receipt_url}</span>
				{/if}
				<span class="mt-1 block text-xs text-muted-foreground">PDF/JPG/PNG maks 10 MB — disimpan ke penyimpanan Tauthub.</span>
			</label>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={payBusy} onclick={() => (payTarget = null)}>Batalkan</button>
			<button class="app-btn app-btn-primary" type="button" disabled={payBusy} onclick={() => void markPaid()}>
				{payBusy ? 'Menyimpan…' : 'Konfirmasi Dibayar'}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if detailTarget}
	{@const d = detailTarget}
	{@const hasRek = Boolean(d.bank_account_number)}
	<Modal title="Detail Settlement" label="Detail settlement" maxWidth="md" onClose={() => (detailTarget = null)}>
		<div class="flex flex-col gap-4 text-sm">
			<div class="grid gap-x-4 gap-y-2 sm:grid-cols-2">
				<div>
					<p class="text-xs font-semibold text-muted-foreground">Mitra</p>
					<p class="font-medium">{d.partner_name ?? d.partner_id.slice(0, 8)}</p>
				</div>
				<div>
					<p class="text-xs font-semibold text-muted-foreground">Periode</p>
					<p>{fmtDate(d.period_start)} — {fmtDate(d.period_end)}</p>
				</div>
				<div>
					<p class="text-xs font-semibold text-muted-foreground">Status</p>
					<p><Badge label={d.status} tone={STATUS_TONE[d.status] ?? 'neutral'} /></p>
				</div>
				<div>
					<p class="text-xs font-semibold text-muted-foreground">Nomor Invoice</p>
					<p class="tabular-nums" style="color: var(--muted-foreground);">{d.invoice_number ?? '—'}</p>
				</div>
			</div>

			<div class="rounded-xl border p-4" style="border-color: var(--border);">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<p class="text-xs font-semibold text-muted-foreground">Pembayaran</p>
					<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">
						{d.payout_method === 'ewallet' ? 'E-Wallet' : 'Transfer Bank'}
					</span>
				</div>
				<p class="mt-1 text-2xl font-semibold tabular-nums">{fmtRp(payoutOf(d))}</p>
				<dl class="mt-3 grid gap-1.5 border-t pt-3 text-xs" style="border-color: var(--border);">
					<div class="flex justify-between gap-4">
						<dt class="text-muted-foreground">Bruto</dt>
						<dd class="tabular-nums">{fmtRp(d.total_amount)}</dd>
					</div>
					{#if feeOf(d) > 0}
						<div class="flex justify-between gap-4">
							<dt class="text-muted-foreground">Potongan transfer</dt>
							<dd class="tabular-nums">− {fmtRp(feeOf(d))}</dd>
						</div>
					{/if}
					<div class="flex justify-between gap-4 font-semibold">
						<dt>Neto</dt>
						<dd class="tabular-nums">{fmtRp(payoutOf(d))}</dd>
					</div>
				</dl>
				{#if d.payment_reference}<p class="mt-3 text-xs">Ref: {d.payment_reference}</p>{/if}
				{#if d.receipt_url}<p class="mt-1 text-xs"><a class="underline" href={d.receipt_url} target="_blank" rel="noreferrer">Bukti transfer</a></p>{/if}
				{#if d.status === 'failed' && d.rejection_reason}<p class="mt-1 text-xs" style="color: var(--destructive);">Alasan tolak: {d.rejection_reason}</p>{/if}
			</div>

			<div class="rounded-xl border p-4" style="border-color: {hasRek ? 'var(--border)' : 'var(--warning, #d97706)'}; background: {hasRek ? 'transparent' : 'var(--warning-soft, #fff8e6)'};">
				<p class="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
					<Landmark size={13} />Rekening Mitra
				</p>
				{#if hasRek}
					<div class="mt-3 grid gap-x-6 gap-y-2 sm:grid-cols-[auto_1fr]">
						<p class="text-xs font-medium text-muted-foreground">Bank</p>
						<p class="font-medium">{d.bank_name ?? '—'}</p>
						<p class="text-xs font-medium text-muted-foreground">Nomor</p>
						<div class="flex items-center gap-2">
							<p class="font-bold tabular-nums text-base">{d.bank_account_number}</p>
							<button
								type="button"
								class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
								aria-label="Salin nomor rekening"
								title={copiedKey === 'account' ? 'Tersalin' : 'Salin nomor rekening'}
								onclick={() => void copyValue('account', String(d.bank_account_number ?? ''))}
							>
								{#if copiedKey === 'account'}<Check size={14} />{:else}<Copy size={14} />{/if}
							</button>
						</div>
						<p class="text-xs font-medium text-muted-foreground">Nama Pemilik</p>
						<p class="font-medium">{d.bank_account_name ?? '—'}</p>
					</div>
					<p class="mt-2 text-xs text-muted-foreground">Transfer manual ke rekening ini dengan jumlah neto di atas.</p>
				{:else}
					<p class="mt-1.5 text-xs" style="color: var(--destructive);">
						Mitra belum melengkapi data rekening ({d.payout_method === 'ewallet' ? 'e-wallet' : 'bank'}).
						Minta mitra mengisi di profilnya sebelum dibayar.
					</p>
				{/if}
			</div>

			{#if d.notes}
				<div>
					<p class="text-xs font-semibold text-muted-foreground">Catatan</p>
					<p class="whitespace-pre-wrap text-xs">{d.notes}</p>
				</div>
			{/if}
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (detailTarget = null)}>Tutup</button>
		{/snippet}
	</Modal>
{/if}
