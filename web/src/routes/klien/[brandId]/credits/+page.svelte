<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kredit Saldo — wallet, konfigurasi TC, ledger, transaksi (port section
	// credits app lama). Termasuk aksi Top Up dan laporan topup gagal.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Coins, Lock, Wallet, Plus, TriangleAlert, X, Flag } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, readApiError, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import TopUpModal from '$lib/components/credit/topup-modal.svelte';
	import { submitTopUp, type TopUpInvoiceData } from '$lib/credits/topup-client';
	import PageTitle from '$lib/components/page-title.svelte';
	import ComingSoon from '$lib/components/coming-soon.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Credits = {
		pending_payment: { id: string; payment_url: string | null; amount_idr: number; tc: number | null; status: string } | null;
		balance: { total: number; used: number; remaining: number; package_count: number };
		wallet: { available_tc: number; reserved_tc: number; captured_tc: number; total_tc: number; rate_idr: number; available_idr: number };
		tc_config: { rate_idr: number; topup_packages: Array<{ tc?: number; idr?: number; price_idr?: number; label?: string }> };
		ledger: Array<{ id: string; ref_type: string; amount_tc: number; balance_after_tc: number; reason: string | null; document_title: string | null; media_name: string | null; created_at: string }>;
		ledger_total: number;
		transactions: Array<{ id: string; amount: number; reason: string; balance_after: number; package_name: string | null; document_title: string | null; created_at: string }>;
		total: number;
	};

	let data = $state<Credits | null>(null);
	let loading = $state(true);

	// Normalisasi paket: backend mengirim {tc, idr}; sebagian konfigurasi lama
	// memakai price_idr. Keduanya diterima agar modal selalu punya nominal.
	const packages = $derived(
		(data?.tc_config.topup_packages ?? [])
			.map((p) => ({ tc: Number(p.tc ?? 0), idr: Number(p.idr ?? p.price_idr ?? 0) }))
			.filter((p) => p.tc > 0 && p.idr > 0)
	);
	const rateIdr = $derived(data?.tc_config.rate_idr ?? data?.wallet.rate_idr ?? 1000);

	let topupOpen = $state(false);
	let topupBusy = $state(false);
	let topupError = $state('');
	let topupPayment = $state<{ payment_url: string | null; amount_idr: number; tc: number } | null>(null);
	// Satu intent = satu idempotency key; dipertahankan saat retry (parity).
	let topupKey = '';

	let reportOpen = $state(false);
	let reportTc = $state('');
	let reportRef = $state('');
	let reportDesc = $state('');
	let reportBusy = $state(false);
	let reportErr = $state('');
	let reportMsg = $state('');

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/credits?limit=50`);
		if (res.ok) data = await res.json();
		loading = false;
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		// Tab checkout gateway menulis kunci ini setelah pembayaran selesai.
		const onStorage = (event: StorageEvent) => {
			if (event.key === 'tauthub:topup-success' || event.key === 'tauthub:topup-cancelled') {
				topupPayment = null;
				topupError = '';
				topupOpen = false;
				topupKey = crypto.randomUUID();
				void load();
			}
		};
		window.addEventListener('storage', onStorage);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.removeEventListener('storage', onStorage);
		};
	});

	function openTopUp() {
		topupPayment = null;
		topupError = '';
		topupOpen = true;
		if (!topupKey) topupKey = crypto.randomUUID();
	}

	async function startTopUp(tc: number, invoice?: TopUpInvoiceData | null) {
		topupBusy = true;
		topupError = '';
		if (!topupKey) topupKey = crypto.randomUUID();
		try {
			const result = await submitTopUp(`/v1/brands/${brandId}`, tc, topupKey, invoice);
			if (result.status === 'paid') {
				topupKey = crypto.randomUUID();
				topupPayment = null;
				topupOpen = false;
				showToast(tr('Top up ini sudah terbayar. Saldo telah diperbarui.'));
				await load();
				return;
			}
			if (['expired', 'cancelled', 'failed'].includes(result.status)) {
				topupKey = crypto.randomUUID();
				topupPayment = null;
				topupError = 'Pembayaran sebelumnya batal atau kedaluwarsa. Silakan coba top up lagi.';
				return;
			}
			topupPayment = { payment_url: result.payment_url, amount_idr: result.amount_idr, tc: result.tc };
		} catch (reason) {
			topupError = reason instanceof Error ? reason.message : tr('Gagal membuat pembayaran.');
		} finally {
			topupBusy = false;
		}
	}

	function resetTopUpPayment() {
		topupPayment = null;
		topupKey = crypto.randomUUID();
		void load();
	}

	function openReport() {
		reportOpen = true;
		reportTc = '';
		reportRef = '';
		reportDesc = '';
		reportErr = '';
		reportMsg = '';
	}

	async function submitReport() {
		reportErr = '';
		reportMsg = '';
		const claimed = Number(reportTc);
		if (!Number.isFinite(claimed) || claimed <= 0) {
			reportErr = tr('Jumlah rupiah harus lebih dari 0.');
			return;
		}
		if (reportDesc.trim().length < 10) {
			reportErr = 'Deskripsi masalah minimal 10 karakter.';
			return;
		}
		reportBusy = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/credit-reports`, {
				method: 'POST',
				body: JSON.stringify({
					claimed_tc: Math.trunc(claimed),
					payment_reference: reportRef.trim() || undefined,
					description: reportDesc.trim()
				})
			});
			if (!res.ok) {
				reportErr = await readApiError(res, tr('Gagal mengirim laporan.'));
				return;
			}
			reportMsg = 'Laporan terkirim. Tim Tauthub akan memverifikasi dan menambahkan saldo setelah disetujui.';
			reportTc = '';
			reportRef = '';
			reportDesc = '';
		} catch (reason) {
			reportErr = reason instanceof Error ? reason.message : tr('Gagal mengirim laporan.');
		} finally {
			reportBusy = false;
		}
	}

	// Catatan: refLabel/fmtDate untuk tabel ledger dihapus karena daftar mutasi
	// kini digantikan panel Coming Soon. Kembalikan saat fitur ledger dikerjakan.
	const fmt = (n: number) => n.toLocaleString('id-ID');
</script>

<PageTitle title={tr("Kredit Saldo")} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr("Saldo")}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Saldo rupiah untuk eksekusi publikasi.")}</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<button class="app-btn app-btn-soft" type="button" onclick={openReport}>
				<Flag size={15} />{tr("Laporkan Topup Gagal")}</button>
			<button class="app-btn app-btn-primary" type="button" onclick={openTopUp}>
				<Plus size={15} />{tr("Top Up Saldo")}</button>
		</div>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat saldo…")}</p>
	{:else if data}
		{#if data.pending_payment}
			<div class="app-card mt-4 flex flex-wrap items-center justify-between gap-3 border-l-4 p-4" style="border-left-color: var(--warning);">
				<div>
					<p class="text-sm font-medium">{tr("Ada pembayaran top up yang belum selesai")}</p>
					<p class="text-xs text-muted-foreground">
						Rp {fmt(data.pending_payment.amount_idr)} · status {data.pending_payment.status}
					</p>
				</div>
				{#if data.pending_payment.payment_url}
					<a class="app-btn app-btn-primary" href={data.pending_payment.payment_url}>Lanjutkan pembayaran</a>
				{/if}
			</div>
		{/if}

		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="app-card flex items-center gap-3 p-4">
				<Wallet size={18} style="color: var(--primary);" />
				<div>
					<p class="text-xs text-muted-foreground">{tr("Saldo tersedia")}</p>
					<p class="text-2xl font-bold tabular-nums">Rp {fmt(data.wallet.available_tc * rateIdr)}</p>
				</div>
			</div>
			<div class="app-card flex items-center gap-3 p-4">
				<Lock size={18} style="color: var(--warning);" />
				<div>
					<p class="text-xs text-muted-foreground">Tertahan</p>
					<p class="text-2xl font-bold tabular-nums">Rp {fmt(data.wallet.reserved_tc * rateIdr)}</p>
				</div>
			</div>
			<div class="app-card flex items-center gap-3 p-4">
				<Coins size={18} style="color: var(--success);" />
				<div>
					<p class="text-xs text-muted-foreground">{tr("Total terpakai")}</p>
					<p class="text-2xl font-bold tabular-nums">Rp {fmt(data.wallet.captured_tc * rateIdr)}</p>
				</div>
			</div>
		</div>


		<div class="app-card mt-4 overflow-x-auto">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<h2 class="text-sm font-semibold">{tr("Riwayat ledger")}</h2>
				<span class="text-xs text-muted-foreground">{fmt(data.ledger_total)} entri</span>
			</div>
			<!-- Fitur belum dirilis — daftar mutasi digantikan panel Coming Soon
			     (parity distribution.tsx Riwayat Transaksi di project lama). -->
			<div class="p-4">
				<ComingSoon message="Riwayat mutasi saldo kredit sedang kami siapkan dan akan segera tersedia di Tauthub." />
			</div>
		</div>
	{/if}
</section>

{#if topupOpen}
	<TopUpModal
		{rateIdr}
		packages={packages}
		busy={topupBusy}
		error={topupError}
		payment={topupPayment}
		onSelectPackage={startTopUp}
		onResetPayment={resetTopUpPayment}
		onClose={() => (topupOpen = false)}
	/>
{/if}

{#if reportOpen}
	<div class="rp-overlay" role="presentation" onclick={() => (reportOpen = false)}>
		<dialog open class="rp-dialog" aria-label={tr("Laporkan Topup Gagal")} onclick={(e) => e.stopPropagation()}>
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">{tr("Laporkan Topup Gagal")}</h2>
					<p class="mt-1 text-xs text-muted-foreground">{tr("Pembayaran sukses tapi saldo belum masuk? Laporan akan diverifikasi admin Tauthub, lalu saldo ditambahkan manual setelah disetujui.")}</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup")} onclick={() => (reportOpen = false)}>
					<X size={18} />
				</button>
			</div>

			<div class="mt-4 flex flex-col gap-3">
				{#if reportMsg}
					<p class="rounded-lg px-3 py-2 text-sm" style="background-color: color-mix(in srgb, var(--success) 10%, var(--card)); color: var(--success);">{reportMsg}</p>
				{/if}
				{#if reportErr}
					<p class="rounded-lg px-3 py-2 text-sm" style="background-color: color-mix(in srgb, var(--destructive) 10%, var(--card)); color: var(--destructive);">{reportErr}</p>
				{/if}

				<label class="flex flex-col gap-1 text-sm">{tr("Jumlah yang dibayar")}<input class="app-input" type="number" min="1" placeholder={tr("cth: 1000")} bind:value={reportTc} />
				</label>
				<label class="flex flex-col gap-1 text-sm">
					Referensi pembayaran (opsional)
					<input class="app-input" placeholder={tr("ID transaksi / nomor invoice dari gateway")} bind:value={reportRef} />
				</label>
				<label class="flex flex-col gap-1 text-sm">
					Deskripsi masalah
					<textarea class="app-input" rows="3" placeholder={tr("cth: Sudah bayar via QRIS tapi saldo tidak bertambah setelah 30 menit")} bind:value={reportDesc}></textarea>
				</label>

				<div class="flex justify-end gap-2 border-t pt-3">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (reportOpen = false)}>{tr("Batal")}</button>
					<button class="app-btn app-btn-primary" type="button" disabled={reportBusy} onclick={() => void submitReport()}>
						{reportBusy ? 'Mengirim…' : tr('Kirim Laporan')}
					</button>
				</div>
			</div>
		</dialog>
	</div>
{/if}

<style>
	.rp-overlay {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		overflow-y: auto;
		padding: 24px 16px;
		background-color: rgba(15, 23, 42, 0.45);
	}
	.rp-dialog {
		position: relative;
		width: 100%;
		max-width: 480px;
		margin: auto;
		border: 1px solid var(--border);
		border-radius: 14px;
		padding: 18px 20px;
		background-color: var(--card);
		color: var(--foreground);
		box-shadow: var(--shadow-md);
	}
</style>