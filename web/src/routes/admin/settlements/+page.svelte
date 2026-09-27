<script lang="ts">
	// Settlement — port app/admin/settlements/page.tsx: payout ke partner.
	// Create meng-agregasi earnings pending dalam periode. Review withdraw:
	// tolak wajib alasan, bayar + nomor referensi & bukti transfer, void.
	// Plus pengaturan saldo minimal penarikan.
	import { onMount } from 'svelte';
	import { TriangleAlert, Wallet } from '@lucide/svelte';
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
		receipt_url: string | null; payment_reference: string | null;
		rejection_reason: string | null; paid_at: string | null; created_at: string;
	};
	type Partner = { id: string; name: string | null; email: string; status: string };

	const PAGE_SIZE = 20;
	const NEXT_STATUS: Record<string, string> = { pending: 'approved', approved: 'processing', processing: 'paid' };
	const STATUS_TONE: Record<string, 'success' | 'neutral' | 'warning' | 'accent' | 'danger'> = {
		pending: 'warning', approved: 'accent', processing: 'neutral', paid: 'success', failed: 'danger'
	};

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
			error = 'Saldo minimal harus angka positif.';
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
		configNotice = 'Pengaturan penarikan dana berhasil disimpan.';
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

	async function transition(settlement: Settlement, status: string) {
		error = '';
		const res = await apiRequest(`/v1/platform/settlements/${settlement.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status })
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
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
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><Wallet size={16} /> Buat settlement</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke settlement.</p>
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-2">
			<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5" style="max-width: 380px;">
				<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari nama partner…" aria-label="Cari settlement" value={search} oninput={(e) => onSearch((e.currentTarget as HTMLInputElement).value)} />
			</label>
			{#if debouncedSearch}
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={clearSearch}>Bersihkan pencarian</button>
			{/if}
		</div>

		<Feedback {loading} {error} notice={configNotice} loadingText="Memuat settlement…" />

		<div class="app-card mt-4 p-5">
			<div class="flex flex-wrap items-start justify-between gap-3">
				<div class="min-w-[220px] flex-1">
					<h2 class="text-sm font-semibold">Pengaturan Penarikan Dana</h2>
					<p class="mt-1 text-sm text-muted-foreground">
						Saldo minimal yang harus dimiliki Mitra untuk dapat mengajukan penarikan. Nilai ini ditampilkan di dashboard Mitra dan divalidasi di server.
					</p>
				</div>
				<div class="flex flex-wrap items-end gap-2">
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Saldo Minimal Penarikan (IDR)</span>
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
						{configBusy ? 'Menyimpan…' : 'Simpan Pengaturan'}
					</button>
				</div>
			</div>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 940px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Partner</th>
						<th class="px-4 py-2.5 font-medium">Periode</th>
						<th class="px-4 py-2.5 font-medium">Total</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Dibayar</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<Wallet size={16} class="shrink-0 text-muted-foreground" />
									<div>
										<p class="font-medium">{row.partner_name ?? row.partner_id.slice(0, 8)}</p>
										<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5">{fmtDate(row.period_start)} — {fmtDate(row.period_end)}</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.total_amount)}</td>
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
								<div class="flex gap-2">
									{#if row.status === 'processing'}
										<button
											class="app-btn !min-h-7 !px-2.5 !text-xs"
											style="background-color: var(--success-soft); color: var(--success);"
											type="button"
											onclick={() => { payTarget = row; payForm = { payment_reference: row.payment_reference ?? '', receipt_url: row.receipt_url ?? '' }; payReceiptFile = null; error = ''; }}
										>Bayar + Bukti</button>
									{/if}
									{#if NEXT_STATUS[row.status] && row.status !== 'processing'}
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void transition(row, NEXT_STATUS[row.status])}>{NEXT_STATUS[row.status]}</button>
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
					<Wallet size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">Belum ada settlement.</p>
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
	<Modal title="Buat settlement" label="Buat settlement" maxWidth="lg" onClose={() => (createOpen = false)}>
		<form class="flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Partner</span>
				<Select
					ariaLabel="Partner"
					minWidth={260}
					value={form.partner_id}
					onchange={(v) => (form.partner_id = v)}
					options={[
						{ value: '', label: 'Pilih partner' },
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
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat settlement'}</button>
			</div>
		</form>
	</Modal>
{/if}

{#if rejectTarget}
	<Modal title="Tolak Withdrawal" label="Tolak withdrawal" maxWidth="md" onClose={() => (rejectTarget = null)}>
		<p class="text-sm text-muted-foreground">
			Tolak penarikan <strong>{fmtRp(rejectTarget.total_amount)}</strong>? Saldo earnings akan dikembalikan ke mitra dan alasan ini terlihat olehnya.
		</p>
		<label class="mt-3 block">
			<span class="mb-1 block text-xs font-semibold text-muted-foreground">Alasan Penolakan *</span>
			<textarea
				class="app-input w-full"
				rows={4}
				placeholder="Contoh: Nominal tidak sesuai dengan saldo earnings, mohon ajukan ulang."
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
			Tandai penarikan <strong>{fmtRp(payTarget.total_amount)}</strong> sebagai PAID setelah transfer manual selesai.
		</p>
		<div class="mt-3 flex flex-col gap-3">
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Nomor Referensi Transfer (opsional)</span>
				<input class="app-input w-full" placeholder="Contoh: BRIVA/1234567890" bind:value={payForm.payment_reference} />
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
