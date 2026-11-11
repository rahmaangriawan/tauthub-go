<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Invoice / Settlements mitra — port section "Invoice Penarikan": daftar
	// invoice penarikan dana dengan paginasi offset (limit/offset/total dari
	// API), kolom tanggal selesai, modal detail + bukti transfer, dan panduan
	// saldo minimal pada state kosong.
	import { onMount } from 'svelte';
	import { Calendar, Clock, ExternalLink, FileText, Image, Landmark, Receipt, Wallet, X } from '@lucide/svelte';
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

	const rp = (n: number) => `${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
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
	/** Keterangan satu baris sesuai status aktual (murni teks, bukan data baru). */
	function statusNote(status: string): string {
		switch (status) {
			case 'pending':
				return tr('Pengajuan sedang diperiksa oleh admin Tauthub.');
			case 'approved':
				return tr('Pengajuan disetujui admin dan menunggu proses transfer.');
			case 'processing':
				return tr('Dana sedang dalam proses transfer ke rekening Anda.');
			case 'paid':
				return tr('Dana telah berhasil ditransfer ke rekening Anda.');
			case 'failed':
				return tr('Pengajuan ditolak/dibatalkan admin.');
			default:
				return '';
		}
	}
</script>

<PageTitle title={tr('Invoice & Penarikan')} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="page-title">{tr('Invoice & Penarikan')}</h1>
			<p class="page-desc">
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
	<div class="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:p-4" style="background-color: rgba(15,23,42,.5);" role="presentation">
		<dialog
			open
			class="flex max-h-[92vh] w-full flex-col overflow-hidden rounded-t-2xl border bg-white shadow-2xl sm:max-h-[90vh] sm:max-w-[520px] sm:rounded-2xl"
			style="border-color: var(--border);"
			aria-label={tr("Detail Invoice Penarikan")}
		>
			<!-- Header -->
			<div class="flex items-start justify-between gap-4 px-5 pt-5 sm:px-6">
				<div class="flex items-start gap-3.5">
					<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl" style="background-color: var(--accent); color: var(--primary);">
						<FileText size={20} strokeWidth={2} />
					</span>
					<div class="min-w-0">
						<h2 class="text-xl font-semibold tracking-tight">{tr("Detail Invoice Penarikan")}</h2>
						<p class="mt-0.5 text-[13px] leading-relaxed text-muted-foreground">{tr("Informasi pengajuan dan bukti transfer dari Tauthub.")}</p>
					</div>
				</div>
				<button
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border transition-colors hover:bg-slate-50"
					style="border-color: var(--border); color: var(--muted-foreground);"
					aria-label={tr("Tutup")}
					onclick={() => (detail = null)}
				>
					<X size={16} />
				</button>
			</div>

			<!-- Body -->
			<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-5 pt-4 sm:px-6">
				<!-- Status + nominal -->
				<div
					class="rounded-xl border px-4 py-3.5"
					style="background-color: {detail.status === 'paid'
						? 'var(--success-soft, rgba(22,121,75,.08))'
						: detail.status === 'failed'
							? 'color-mix(in srgb, var(--destructive) 7%, #fff)'
							: 'var(--warning-soft, rgba(161,92,0,.08))'}; border-color: {detail.status === 'paid'
						? 'color-mix(in srgb, var(--success) 35%, var(--border))'
						: detail.status === 'failed'
							? 'color-mix(in srgb, var(--destructive) 35%, var(--border))'
							: 'color-mix(in srgb, var(--warning, #a15c00) 30%, var(--border))'};"
				>
					<div class="flex flex-wrap items-start justify-between gap-2">
						<div class="min-w-0">
							<p class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{tr("STATUS PENCAIRAN")}</p>
							<p class="mt-0.5 text-[26px] font-bold leading-tight tabular-nums" style="color: {statusColor(detail.status)};">
								{rp(detail.total_amount)}
							</p>
						</div>
						<span class="shrink-0 rounded-full bg-white/70 px-2.5 py-1 text-[11.5px] font-medium" style="color: {statusColor(detail.status)};">
							{statusLabel[detail.status] ?? detail.status}
						</span>
					</div>
					{#if statusNote(detail.status)}
						<p class="mt-1 text-[12px] leading-relaxed text-muted-foreground">{statusNote(detail.status)}</p>
					{/if}
				</div>

				<!-- Informasi transaksi -->
				<p class="mt-5 text-sm font-semibold" style="color: var(--foreground);">{tr("Informasi Transaksi")}</p>
				<div class="mt-2 grid grid-cols-1 gap-x-5 sm:grid-cols-2">
					<div class="flex items-start gap-3 py-2.5">
						<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--muted); color: var(--muted-foreground);">
							<Receipt size={15} strokeWidth={2} />
						</span>
						<div class="min-w-0">
							<p class="text-[11.5px] text-muted-foreground">{tr("Nomor Invoice")}</p>
							<p class="break-words text-[13.5px] font-semibold" style="color: var(--foreground);">{detail.invoice_number ?? `INV-${detail.id.slice(0, 8).toUpperCase()}`}</p>
						</div>
					</div>
					<div class="flex items-start gap-3 border-t py-2.5 sm:border-t-0" style="border-color: var(--border);">
						<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--muted); color: var(--muted-foreground);">
							<Calendar size={15} strokeWidth={2} />
						</span>
						<div class="min-w-0">
							<p class="text-[11.5px] text-muted-foreground">{tr("Tanggal Pengajuan")}</p>
							<p class="break-words text-[13.5px] font-semibold" style="color: var(--foreground);">{dt(detail.created_at || detail.period_start)}</p>
						</div>
					</div>
					<div class="flex items-start gap-3 border-t py-2.5" style="border-color: var(--border);">
						<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--muted); color: var(--muted-foreground);">
							<Clock size={15} strokeWidth={2} />
						</span>
						<div class="min-w-0">
							<p class="text-[11.5px] text-muted-foreground">{tr("Status Transfer")}</p>
							<p class="break-words text-[13.5px] font-semibold" style="color: var(--foreground);">{detail.paid_at ? dt(detail.paid_at) : tr('Menunggu Transfer')}</p>
							{#if !detail.paid_at}
								<p class="mt-0.5 text-[11px] text-muted-foreground">{tr("Transfer akan diproses setelah disetujui.")}</p>
							{/if}
						</div>
					</div>
					<div class="flex items-start gap-3 border-t py-2.5" style="border-color: var(--border);">
						<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--muted); color: var(--muted-foreground);">
							<Landmark size={15} strokeWidth={2} />
						</span>
						<div class="min-w-0">
							<p class="text-[11.5px] text-muted-foreground">{tr("Rekening Tujuan")}</p>
							<p class="break-words text-[13.5px] font-semibold" style="color: var(--foreground);">{detail.notes || tr('Rekening terdaftar')}</p>
							{#if detail.payout_method}
								<p class="mt-0.5 text-[11px] text-muted-foreground">{detail.payout_method === 'ewallet' ? tr('E-Wallet') : tr('Transfer Bank')}</p>
							{/if}
						</div>
					</div>
					{#if detail.payment_reference}
						<div class="flex items-start gap-3 border-t py-2.5 sm:col-span-2" style="border-color: var(--border);">
							<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--muted); color: var(--muted-foreground);">
								<Wallet size={15} strokeWidth={2} />
							</span>
							<div class="min-w-0">
								<p class="text-[11.5px] text-muted-foreground">{tr("Nomor Referensi Transfer")}</p>
								<p class="break-words text-[13.5px] font-semibold" style="color: var(--foreground);">{detail.payment_reference}</p>
							</div>
						</div>
					{/if}
				</div>

				{#if detail.status === 'failed'}
					<div class="mt-2 rounded-xl border px-4 py-3" style="background-color: color-mix(in srgb, var(--destructive) 6%, #fff); border-color: color-mix(in srgb, var(--destructive) 30%, var(--border));">
						<p class="text-[13px] font-semibold" style="color: var(--destructive);">{tr("Penarikan Ditolak Admin")}</p>
						<p class="mt-0.5 text-[12.5px] leading-relaxed" style="color: var(--foreground);">{detail.rejection_reason || tr('Tidak ada alasan tercatat.')}</p>
						<p class="mt-0.5 text-[11.5px] text-muted-foreground">{tr("Saldo earnings telah dikembalikan dan dapat diajukan kembali.")}</p>
					</div>
				{/if}

				<!-- Bukti pembayaran -->
				<p class="mt-5 text-sm font-semibold" style="color: var(--foreground);">{tr("Bukti Pembayaran / Transfer Bank")}</p>
				{#if detail.receipt_url}
					<div class="mt-2 rounded-xl border p-3 text-center" style="border-color: var(--border); background-color: var(--muted);">
						<img src={detail.receipt_url} alt={tr("Bukti Transfer")} class="mx-auto max-h-60 rounded-lg object-contain" />
						<a class="mt-2 inline-flex items-center gap-1 text-xs font-medium" style="color: var(--primary);" href={detail.receipt_url} target="_blank" rel="noreferrer">
							{tr("Buka gambar ukuran penuh")} <ExternalLink size={11} />
						</a>
					</div>
				{:else if detail.status === 'paid'}
					<div class="mt-2 rounded-xl border px-4 py-3" style="background-color: var(--success-soft, rgba(22,121,75,.08)); border-color: color-mix(in srgb, var(--success) 30%, var(--border));">
						<p class="text-[13px] font-semibold" style="color: var(--success);">{tr("Pembayaran Selesai")}</p>
						<p class="mt-0.5 text-[12px] leading-relaxed text-muted-foreground">
							{tr("Transfer dana sebesar")} {rp(detail.total_amount)} {tr("telah berhasil dikirimkan oleh tim Tauthub ke rekening Anda.")}
						</p>
					</div>
				{:else}
					<div class="mt-2 rounded-xl border border-dashed px-4 py-6 text-center" style="border-color: var(--border); background-color: var(--muted);">
						<span class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-white/70" style="color: var(--primary);">
							<Image size={20} strokeWidth={1.8} />
						</span>
						<p class="mt-2 text-[13px] font-semibold" style="color: var(--foreground);">{tr("Belum tersedia")}</p>
						<p class="mx-auto mt-0.5 max-w-[360px] text-[12px] leading-relaxed text-muted-foreground">
							{tr("Bukti transfer atau konfirmasi pembayaran akan otomatis tersedia di sini setelah admin Tauthub menyelesaikan transfer.")}
						</p>
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end border-t px-5 py-3.5 sm:px-6" style="border-color: var(--border); background-color: #fff; padding-bottom: calc(0.875rem + env(safe-area-inset-bottom));">
				<button class="app-btn app-btn-soft min-h-11 rounded-[9px] px-5" onclick={() => (detail = null)}>{tr("Tutup")}</button>
			</div>
		</dialog>
	</div>
{/if}
