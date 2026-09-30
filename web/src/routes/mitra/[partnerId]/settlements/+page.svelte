<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Invoice / Settlements mitra — port section "Invoice Penarikan": daftar
	// invoice penarikan dana dengan paginasi offset (limit/offset/total dari
	// API), kolom tanggal selesai, modal detail + bukti transfer, dan panduan
	// saldo minimal pada state kosong.
	import { onMount } from 'svelte';
	import { ExternalLink, Receipt, Wallet, X } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const PAGE_SIZE = 20;

	type Settlement = {
		id: string;
		period_start: string | null;
		period_end: string | null;
		status: string;
		total_amount: number;
		payout_method: string;
		transfer_fee_idr: number;
		net_amount_idr: number;
		invoice_number: string | null;
		notes: string | null;
		receipt_url: string | null;
		payment_reference: string | null;
		rejection_reason: string | null;
		paid_at: string | null;
		created_at: string | null;
		updated_at: string | null;
	};

	let loading = $state(true);
	let data = $state<{ settlements: Settlement[]; total: number; minimum_withdraw_idr?: number } | null>(null);
	let statusFilter = $state('');
	let offset = $state(0);
	let error = $state('');
	let detail = $state<Settlement | null>(null);

	const total = $derived(data?.total ?? 0);
	const settlements = $derived(data?.settlements ?? []);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
			const onExpired = () => (window.location.href = '/login');
			const onKeydown = (event: KeyboardEvent) => {
				if (event.key === 'Escape' && detail) detail = null;
			};
			window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.addEventListener('keydown', onKeydown);
			return () => {
				window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
				window.removeEventListener('keydown', onKeydown);
			};
	});

	async function load() {
		error = '';
		const params = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) });
		if (statusFilter) params.set('status', statusFilter);
		const res = await apiRequest(`/v1/partner/settlements?${params}`);
		if (res.ok) {
			data = await res.json();
		} else {
			error = await readApiError(res, tr('Gagal memuat daftar penarikan.'));
			data = null;
		}
	}

	function changeFilter(value: string) {
		statusFilter = value;
		offset = 0;
		void load();
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const statusLabel: Record<string, string> = $derived({
		pending: tr('Menunggu review'),
		approved: tr('Disetujui'),
		processing: tr('Sedang Ditransfer'),
		paid: tr('Selesai / Terbayar'),
		failed: tr('Ditolak / Dibatalkan')
	});
	function statusColor(status: string) {
		if (status === 'paid') return 'var(--success)';
		if (status === 'failed') return 'var(--destructive)';
		if (status === 'pending' || status === 'approved') return 'var(--warning, #a15c00)';
		return 'var(--muted-foreground)';
	}
</script>

<PageTitle title={tr('Invoice & Penarikan')} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr('Invoice & Penarikan')}</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				{tr("Daftar invoice penarikan dana, status pencairan, dan bukti transfer dari Tauthub.")}
				{tr("Minimal penarikan")} {rp(data?.minimum_withdraw_idr ?? 0)}.
			</p>
		</div>
		<Select
			value={statusFilter}
			ariaLabel={tr('Status penarikan')}
			placeholder={tr("Semua status")}
			align="end"
			onchange={(v) => changeFilter(v)}
			options={[
				{ value: '', label: tr('Semua status') },
				{ value: 'pending', label: tr('Menunggu review') },
				{ value: 'approved', label: tr('Disetujui') },
				{ value: 'processing', label: tr('Sedang Ditransfer') },
				{ value: 'paid', label: tr('Selesai / Terbayar') },
				{ value: 'failed', label: tr('Ditolak / Dibatalkan') }
			]}
		/>
	</header>

	{#if error}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
	{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat penarikan…")}</p>
	{:else if !error && (data?.settlements ?? []).length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<Receipt size={28} class="mx-auto text-muted-foreground" />
			<p class="mt-3 text-sm text-muted-foreground">
				{tr("Belum ada invoice penarikan dana. Ajukan penarikan dari menu Penghasilan saat saldo mencapai minimal")} {rp(data?.minimum_withdraw_idr ?? 0)}.
			</p>
		</div>
	{:else if data}
			<div class="app-card mt-4 overflow-x-auto">
				<table class="w-full min-w-[760px] text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">{tr("No. Invoice / ID")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Tanggal Pengajuan")}</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr("Nominal Penarikan")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Status Pembayaran")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Tanggal Selesai")}</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr("Aksi")}</th>
					</tr>
				</thead>
				<tbody>
					{#each settlements as s (s.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<Wallet size={15} class="shrink-0 text-muted-foreground" />
									<div class="min-w-0">
										<p class="truncate font-mono text-xs font-semibold">{s.invoice_number ?? `INV-${s.id.slice(0, 8).toUpperCase()}`}</p>
										<p class="truncate text-[11px] text-muted-foreground">{s.notes ? s.notes.slice(0, 30) : 'Penarikan Mitra'}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{dt(s.created_at || s.period_start)}</td>
							<td class="px-4 py-2.5 text-right font-medium tabular-nums">{rp(s.total_amount)}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {statusColor(s.status)};">
									{statusLabel[s.status] ?? s.status}
								</span>
								{#if s.status === 'failed' && s.rejection_reason}
									<p class="mt-0.5 text-[11px]" style="color: var(--destructive);">{tr('Alasan')}: {s.rejection_reason}</p>
								{/if}
								{#if s.payment_reference}
									<p class="mt-0.5 text-[11px] text-muted-foreground">Ref: {s.payment_reference}</p>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{s.paid_at ? dt(s.paid_at) : '—'}</td>
							<td class="px-4 py-2.5 text-right">
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => (detail = s)}>
									{tr("Detail & Bukti")}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if total > PAGE_SIZE}
				<div class="flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3">
					<p class="text-xs text-muted-foreground">
						{tr("Menampilkan")} {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} {tr("dari")} {total}
					</p>
					<div class="flex items-center gap-2">
						<button
							class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
							disabled={offset === 0 || loading}
							onclick={() => { offset = Math.max(0, offset - PAGE_SIZE); void load(); }}
						>‹ {tr("Sebelumnya")}</button>
						<p class="text-xs text-muted-foreground">{tr("Halaman")} {Math.floor(offset / PAGE_SIZE) + 1} {tr("dari")} {Math.max(1, Math.ceil(total / PAGE_SIZE))}</p>
						<button
							class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
							disabled={offset + PAGE_SIZE >= total || loading}
							onclick={() => { offset = offset + PAGE_SIZE; void load(); }}
						>{tr("Berikutnya")} ›</button>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</section>

{#if detail}
		<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={(event) => event.currentTarget === event.target && (detail = null)}>
			<dialog open class="app-card my-auto w-full max-w-lg p-5" aria-label={tr("Detail Invoice Penarikan")}>
				<div class="flex items-start justify-between gap-4">
					<h2 class="text-lg font-semibold">{tr("Detail Invoice Penarikan")}</h2>
					<button class="app-btn app-btn-soft !min-h-11 !w-11 !p-0" aria-label={tr("Tutup")} onclick={() => (detail = null)}><X size={17} /></button>
				</div>
			<p class="mt-1 text-xs text-muted-foreground">{tr("Informasi tagihan penarikan dan bukti transfer dari Tauthub.")}</p>

			<div class="mt-4 space-y-3">
				<div class="rounded-lg border p-3" style="background-color: {detail.status === 'paid' ? 'var(--success-soft, rgba(22,121,75,.08))' : 'var(--warning-soft, rgba(161,92,0,.08))'}; border-color: {detail.status === 'paid' ? 'var(--success)' : 'var(--warning, #a15c00)'};">
					<div class="flex items-center justify-between">
						<p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr("STATUS PENCAIRAN")}</p>
						<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {statusColor(detail.status)};">
							{statusLabel[detail.status] ?? detail.status}
						</span>
					</div>
					<p class="mt-1 text-2xl font-bold tabular-nums" style="color: {detail.status === 'paid' ? 'var(--success)' : 'var(--warning, #a15c00)'};">
						{rp(detail.total_amount)}
					</p>
				</div>

				<div class="grid grid-cols-2 gap-3">
					<div class="rounded-lg border p-2.5" style="background-color: var(--muted);">
						<p class="text-[11px] text-muted-foreground">{tr("Nomor Invoice")}</p>
						<p class="text-sm font-semibold">{detail.invoice_number ?? `INV-${detail.id.slice(0, 8).toUpperCase()}`}</p>
					</div>
					<div class="rounded-lg border p-2.5" style="background-color: var(--muted);">
						<p class="text-[11px] text-muted-foreground">{tr("Tanggal Pengajuan")}</p>
						<p class="text-sm font-semibold">{dt(detail.created_at || detail.period_start)}</p>
					</div>
					<div class="rounded-lg border p-2.5" style="background-color: var(--muted);">
						<p class="text-[11px] text-muted-foreground">{tr("Tanggal Selesai Transfer")}</p>
						<p class="text-sm font-semibold">{detail.paid_at ? dt(detail.paid_at) : tr('Menunggu Transfer')}</p>
					</div>
					<div class="rounded-lg border p-2.5" style="background-color: var(--muted);">
						<p class="text-[11px] text-muted-foreground">{tr("Rekening Tujuan")}</p>
						<p class="text-sm font-semibold">{detail.notes || tr('Rekening terdaftar')}</p>
						{#if detail.payout_method}
							<p class="text-[11px] text-muted-foreground">{detail.payout_method === 'ewallet' ? tr('E-Wallet') : tr('Transfer Bank')}</p>
						{/if}
					</div>
				</div>

				{#if detail.status === 'failed'}
					<div class="rounded-lg border p-3" style="background-color: rgba(197,43,54,.06); border-color: var(--destructive);">
						<p class="text-sm font-semibold" style="color: var(--destructive);">{tr("Penarikan Ditolak Admin")}</p>
						<p class="mt-1 text-sm">{detail.rejection_reason || tr('Tidak ada alasan tercatat.')}</p>
						<p class="mt-1 text-xs text-muted-foreground">{tr("Saldo earnings telah dikembalikan dan dapat diajukan kembali.")}</p>
					</div>
				{/if}

				{#if detail.payment_reference}
					<div class="rounded-lg border p-2.5" style="background-color: var(--muted);">
						<p class="text-[11px] text-muted-foreground">{tr("Nomor Referensi Transfer")}</p>
						<p class="text-sm font-semibold">{detail.payment_reference}</p>
					</div>
				{/if}

				<div>
					<p class="text-sm font-semibold">{tr("Bukti Pembayaran / Transfer Bank")}</p>
					{#if detail.receipt_url}
						<div class="mt-2 rounded-lg border p-3 text-center" style="border-color: var(--border); background-color: var(--muted);">
							<img src={detail.receipt_url} alt={tr("Bukti Transfer")} class="mx-auto max-h-60 rounded-md object-contain" />
							<a class="mt-2 inline-block text-xs" style="color: var(--primary);" href={detail.receipt_url} target="_blank" rel="noreferrer">
								{tr("Buka gambar ukuran penuh")} <ExternalLink size={11} class="inline" />
							</a>
						</div>
					{:else if detail.status === 'paid'}
						<div class="mt-2 rounded-lg border p-3" style="background-color: var(--success-soft, rgba(22,121,75,.08)); border-color: var(--success);">
							<p class="text-sm font-semibold" style="color: var(--success);">{tr("Pembayaran Selesai")}</p>
							<p class="mt-0.5 text-xs text-muted-foreground">
								{tr("Transfer dana sebesar")} {rp(detail.total_amount)} {tr("telah berhasil dikirimkan oleh tim Tauthub ke rekening Anda.")}
							</p>
						</div>
					{:else}
						<div class="mt-2 rounded-lg border border-dashed p-3 text-center" style="border-color: var(--border); background-color: var(--muted);">
							<p class="text-sm text-muted-foreground">
								{tr("Bukti transfer atau konfirmasi pembayaran akan otomatis tersedia di sini setelah admin Tauthub menyelesaikan transfer.")}
							</p>
						</div>
					{/if}
				</div>
			</div>

			<div class="mt-5 flex justify-end">
				<button class="app-btn app-btn-soft" onclick={() => (detail = null)}>{tr("Tutup")}</button>
			</div>
		</dialog>
	</div>
{/if}
