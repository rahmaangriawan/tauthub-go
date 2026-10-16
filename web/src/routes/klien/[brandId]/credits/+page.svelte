<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kredit Saldo — wallet, konfigurasi TC, ledger, transaksi (port section
	// credits app lama). Termasuk aksi Top Up dan laporan topup gagal.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ChevronLeft, ChevronRight, Clock, Coins, Lock, Wallet, Plus, TriangleAlert, X, Flag } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, readApiError, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import TopUpModal from '$lib/components/credit/topup-modal.svelte';
	import { submitTopUp, type TopUpInvoiceData } from '$lib/credits/topup-client';
	import { tcToRp } from '$lib/credits/rupiah';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type LedgerEntry = {
		id: string;
		ref_type: string;
		ref_id: string | null;
		amount_tc: number;
		balance_after_tc: number;
		rate_idr: number | null;
		amount_idr: number | null;
		reason: string | null;
		document_title: string | null;
		media_name: string | null;
		publication_status: string | null;
		created_at: string;
	};
	type LedgerGroup = {
		campaign_id: string;
		campaign_name: string;
		entries: number;
		spent_tc: number;
		net_tc: number;
		held_tc: number;
		last_at: string;
	};
	type Credits = {
		pending_payment: { id: string; payment_url: string | null; amount_idr: number; tc: number | null; status: string } | null;
		pending_payments: Array<{ id: string; payment_url: string | null; amount_idr: number; tc: number | null; status: string; created_at: string }>;
		balance: { total: number; used: number; remaining: number; package_count: number };
		wallet: { available_tc: number; reserved_tc: number; captured_tc: number; total_tc: number; rate_idr: number; available_idr: number };
		tc_config: { rate_idr: number; topup_packages: Array<{ tc?: number; idr?: number; price_idr?: number; label?: string }> };
		ledger: LedgerEntry[];
		ledger_total: number;
		ledger_groups: LedgerGroup[];
		ledger_groups_total: number;
		transactions: Array<{ id: string; amount: number; reason: string; balance_after: number; package_name: string | null; document_title: string | null; created_at: string }>;
		total: number;
	};

	let data = $state<Credits | null>(null);
	let loading = $state(true);
	// Riwayat dipaginasi dari server PER GRUP KAMPANYE (limit/offset grup).
	const LEDGER_PAGE_SIZE = 50;
	let ledgerOffset = $state(0);
	// Rincian entri per grup dimuat saat grup diklik (accordion satu terbuka).
	let openGroup = $state<string | null>(null);
	let groupDetail = $state<Record<string, { loading: boolean; entries: LedgerEntry[]; total: number }>>({});

	// Normalisasi paket: backend mengirim {tc, idr}; sebagian konfigurasi lama
	// memakai price_idr. Keduanya diterima agar modal selalu punya nominal.
	const packages = $derived(
		(data?.tc_config.topup_packages ?? [])
			.map((p) => ({ tc: Number(p.tc ?? 0), idr: Number(p.idr ?? p.price_idr ?? 0) }))
			.filter((p) => p.tc > 0 && p.idr > 0)
	);
	const rateIdr = $derived(data?.tc_config.rate_idr ?? data?.wallet.rate_idr ?? 1000);
	// Semua tagihan top up yang masih menunggu pembayaran: ditampilkan & dihitung
	// sebagai "menunggu pembayaran", TIDAK ditambahkan ke saldo tersedia (uang
	// baru masuk setelah pembayaran selesai).
	const pendingTopups = $derived(data?.pending_payments ?? (data?.pending_payment ? [data.pending_payment] : []));
	const pendingTotalIdr = $derived(pendingTopups.reduce((sum, p) => sum + (Number(p.amount_idr) || 0), 0));
	const latestPending = $derived(pendingTopups[0] ?? null);

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
		const res = await apiRequest(
			`/v1/brands/${brandId}/credits?group_by=campaign&limit=${LEDGER_PAGE_SIZE}&offset=${ledgerOffset}`
		);
		if (res.ok) data = await res.json();
		loading = false;
	}

	// Buka/tutup rincian grup; entri grup dimuat sekali lalu di-cache sampai
	// halaman di-refresh (topup sukses) — data ledger tidak berubah sendiri.
	async function toggleGroup(g: LedgerGroup) {
		const key = g.campaign_id || 'none';
		if (openGroup === key) {
			openGroup = null;
			return;
		}
		openGroup = key;
		if (groupDetail[key]) return;
		groupDetail = { ...groupDetail, [key]: { loading: true, entries: [], total: 0 } };
		try {
			const res = await apiRequest(
				`/v1/brands/${brandId}/credits?campaign_id=${encodeURIComponent(key)}&limit=500`
			);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat rincian saldo.')));
			const d = await res.json();
			groupDetail = {
				...groupDetail,
				[key]: { loading: false, entries: (d?.ledger ?? []) as LedgerEntry[], total: Number(d?.ledger_total ?? 0) }
			};
		} catch {
			groupDetail = { ...groupDetail, [key]: { loading: false, entries: [], total: 0 } };
		}
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
					// Saldo berubah → cache rincian grup dibuang, muat ulang.
					openGroup = null;
					groupDetail = {};
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

	const fmt = (n: number) => n.toLocaleString('id-ID');

	// Tanggal mutasi ditampilkan WIB (format sama dengan halaman Taut Pintar).
	const jakartaDisplay = (iso: string | null) =>
		iso
			? new Date(iso).toLocaleString('id-ID', {
					timeZone: 'Asia/Jakarta',
					day: 'numeric',
					month: 'short',
					year: 'numeric',
					hour: '2-digit',
					minute: '2-digit'
				})
			: '—';

	// Label human-readable saat `reason` kosong/berisi string mesin.
	const REF_TYPE_LABEL: Record<string, string> = {
		topup: 'Top Up Saldo',
		reserve: 'Dana ditahan untuk publikasi',
		release: 'Dana ditahan dikembalikan',
		capture: 'Kredit publikasi ditagihkan (artikel terbit)',
		adjustment: 'Penyesuaian saldo',
		refund: 'Pengembalian klaim Link Guardian'
	};

	function ledgerLabel(e: LedgerEntry): string {
		const reason = (e.reason ?? '').trim();
		// Baris lama di DB memakai nama produk internal ("Top Up Tulisio
		// Credit — pembayaran X") — dinormalkan ke penamaan baru saat tampil.
		const legacyTopup = /^Top Up Tulisio Credit — pembayaran (.+)$/i.exec(reason);
		if (legacyTopup) return `Top Up Saldo (${legacyTopup[1]}) berhasil`;
		// Reason mesin (huruf kecil bertitik, mis. "ai.image.generate") diganti
		// label manusiawi; reason berbahasa Indonesia dari server dipakai apa adanya.
		if (reason === 'ai.image.generate') return tr('Gambar AI untuk artikel');
		if (reason && !/^[a-z0-9]+([._-][a-z0-9]+)+$/.test(reason)) return reason;
		return tr(REF_TYPE_LABEL[e.ref_type] ?? 'Mutasi saldo');
	}

	type LedgerRow = { tone: 'in' | 'out' | 'hold'; sign: string; amount: string; label: string; sub: string | null };

	// ================== ATURAN ARAH MUTASI — BACA SEBELUM MENGUBAH ==================
	// Warna & tanda mengikuti ARTI uangnya, bukan tanda mentah amount_tc:
	// - `reserve` (−): dana dialokasikan/ditahan → netral (abu), BUKAN merah.
	//   Uangnya belum terpakai; bila publikasi batal akan kembali lewat `release` (+).
	// - `capture` (+) dari dana tertahan: penagihan saat artikel TERBIT → MERAH −.
	//   Tanda mentahnya positif, tapi ini uang KELUAR — kalau ditampilkan hijau +
	//   artinya persis kebalik. Jangan "perbaiki" ini jadi ikut tanda mentah.
	// - `capture` (−) debit langsung (Taut Pintar lama), `adjustment` (−),
	//   `refund` (−) → merah −.
	// - `topup` (+), `adjustment` (+) → hijau + ; `release` (+) → hijau + (dana kembali).
	// ==================================================================================
	function ledgerRow(e: LedgerEntry, fallbackRate: number): LedgerRow {
		const rate = e.rate_idr && e.rate_idr > 0 ? e.rate_idr : fallbackRate;
		const amount = tcToRp(Math.abs(e.amount_tc), rate);
		const sub = [e.document_title, e.media_name].filter(Boolean).join(' · ') || null;
		const label = ledgerLabel(e);
		if (e.ref_type === 'reserve') return { tone: 'hold', sign: '', amount, label, sub };
		const outgoing = e.ref_type === 'capture' || e.ref_type === 'refund' || e.amount_tc < 0;
		return outgoing
			? { tone: 'out', sign: '−', amount, label, sub }
			: { tone: 'in', sign: '+', amount, label, sub };
	}

	// Rentang grup yang sedang tampil untuk teks "Menampilkan X–Y dari N".
	const ledgerFrom = $derived(data && data.ledger_groups_total > 0 ? ledgerOffset + 1 : 0);
	const ledgerTo = $derived(Math.min(ledgerOffset + LEDGER_PAGE_SIZE, data?.ledger_groups_total ?? 0));
	const ledgerHasPrev = $derived(ledgerOffset > 0);
	const ledgerHasNext = $derived(ledgerOffset + LEDGER_PAGE_SIZE < (data?.ledger_groups_total ?? 0));

	function ledgerPage(delta: number) {
		ledgerOffset = Math.max(0, ledgerOffset + delta);
		// Rincian grup ikut direset: komposisi grup bisa bergeser antahalaman.
		openGroup = null;
		groupDetail = {};
		void load();
	}
</script>

<PageTitle title={tr("Saldo")} />

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
		{#if pendingTopups.length > 0}
			<div class="app-card mt-4 flex flex-wrap items-center justify-between gap-3 border-l-4 p-4" style="border-left-color: var(--warning);">
				<div>
					<p class="text-sm font-medium">
						{pendingTopups.length === 1
							? tr("Ada pembayaran top up yang belum selesai")
							: `${pendingTopups.length} ${tr("tagihan top up menunggu pembayaran")}`}
					</p>
					<p class="text-xs text-muted-foreground">
						{tr("Total tagihan menunggu pembayaran")} Rp {fmt(pendingTotalIdr)} · {tr("belum masuk saldo sampai pembayaran selesai")}
					</p>
				</div>
				{#if latestPending?.payment_url}
					<a class="app-btn app-btn-primary" href={latestPending.payment_url}>{tr("Lanjutkan pembayaran")}</a>
				{/if}
			</div>
		{/if}

		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="app-card flex items-center gap-3 p-4">
				<Wallet size={18} style="color: var(--primary);" />
				<div>
					<p class="text-xs text-muted-foreground">{tr("Saldo tersedia")}</p>
					<p class="text-2xl font-bold tabular-nums">Rp {fmt(data.wallet.available_tc * rateIdr)}</p>
					{#if pendingTotalIdr > 0}
						<p class="mt-0.5 text-[11px] font-medium" style="color: var(--warning);">
							+ Rp {fmt(pendingTotalIdr)} {tr("menunggu pembayaran")}
						</p>
					{/if}
				</div>
			</div>
			<div class="app-card flex items-center gap-3 p-4">
				<Lock size={18} style="color: var(--warning);" />
				<div>
					<p class="text-xs text-muted-foreground">{tr("Tertahan")}</p>
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


		<div class="app-card mt-4">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
				<div>
					<h2 class="text-sm font-semibold">{tr("Riwayat penggunaan saldo")}</h2>
					<p class="mt-0.5 text-[11px] text-muted-foreground">{tr("Dikelompokkan per kampanye — klik untuk melihat rinciannya.")}</p>
				</div>
				<span class="text-xs text-muted-foreground">{fmt(data.ledger_groups_total)} {tr("kampanye")}</span>
			</div>

			{#if pendingTopups.length > 0}
				<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3" style="background-color: var(--warning-soft);">
					<span class="flex min-w-0 items-center gap-2">
						<Clock size={14} class="shrink-0" style="color: var(--warning);" />
						<span class="min-w-0">
							<span class="block text-sm font-medium" style="color: var(--warning);">
								{tr("menunggu pembayaran")} · {pendingTopups.length} {tr("tagihan")}
							</span>
							<span class="mt-0.5 block text-xs text-muted-foreground">
								{tr("belum masuk saldo sampai pembayaran selesai")}
							</span>
						</span>
					</span>
					<span class="shrink-0 text-sm font-semibold tabular-nums" style="color: var(--warning);">+Rp {fmt(pendingTotalIdr)}</span>
				</div>
			{/if}

			{#if data.ledger_groups.length === 0}
				<p class="px-4 py-6 text-sm text-muted-foreground">
					{tr("Belum ada mutasi saldo. Top up atau publikasi artikel untuk melihat riwayat.")}
				</p>
			{:else}
				<ul>
					{#each data.ledger_groups as g (g.campaign_id || 'none')}
						{@const key = g.campaign_id || 'none'}
						{@const detail = groupDetail[key]}
						{@const open = openGroup === key}
						<li class="border-b last:border-b-0">
							<button
								class="flex w-full flex-wrap items-center justify-between gap-2 px-4 py-3 text-left transition-colors hover:bg-secondary/50"
								type="button"
								aria-expanded={open}
								onclick={() => toggleGroup(g)}
							>
								<span class="flex min-w-0 items-center gap-2">
									<ChevronRight size={14} class="shrink-0 text-muted-foreground transition-transform {open ? 'rotate-90' : ''}" />
									<span class="min-w-0">
										<span class="block truncate text-sm font-medium">
											{g.campaign_name || tr("Lainnya — top up & penyesuaian saldo")}
										</span>
										<span class="mt-0.5 block text-xs text-muted-foreground">
											{g.entries} {tr("transaksi")} · {jakartaDisplay(g.last_at)}
										</span>
									</span>
								</span>
								<span class="shrink-0 text-right">
									{#if g.campaign_id}
										<span class="block text-sm font-semibold tabular-nums" style="color: {g.spent_tc > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'};">
											{g.spent_tc > 0 ? `−${tcToRp(g.spent_tc, rateIdr)}` : tcToRp(0, rateIdr)}
										</span>
										{#if g.held_tc > 0}
											<span class="mt-0.5 block text-[11px] text-muted-foreground">{tcToRp(g.held_tc, rateIdr)} {tr("ditahan")}</span>
										{/if}
									{:else}
										<span
											class="block text-sm font-semibold tabular-nums"
											style="color: {g.net_tc > 0 ? 'var(--success)' : g.net_tc < 0 ? 'var(--destructive)' : 'var(--muted-foreground)'};"
										>
											{g.net_tc > 0 ? '+' : g.net_tc < 0 ? '−' : ''}{tcToRp(Math.abs(g.net_tc), rateIdr)}
										</span>
									{/if}
								</span>
							</button>

							{#if open}
								<div class="border-t bg-secondary/20 px-4 py-3">
									{#if !detail || detail.loading}
										<p class="text-xs text-muted-foreground">{tr("Memuat rincian…")}</p>
									{:else if detail.entries.length === 0}
										<p class="text-xs text-muted-foreground">{tr("Tidak ada rincian untuk grup ini.")}</p>
									{:else}
										<div class="overflow-x-auto">
											<table class="w-full text-sm">
												<thead>
													<tr class="text-left text-[11px] uppercase tracking-wide text-muted-foreground">
														<th class="py-1.5 pr-4 font-semibold">{tr("Tanggal")}</th>
														<th class="py-1.5 pr-4 font-semibold">{tr("Keterangan")}</th>
														<th class="py-1.5 text-right font-semibold">{tr("Nominal")}</th>
													</tr>
												</thead>
												<tbody>
													{#each detail.entries as e (e.id)}
														{@const row = ledgerRow(e, rateIdr)}
														<tr class="border-t border-border/60">
															<td class="whitespace-nowrap py-2 pr-4 text-xs text-muted-foreground">{jakartaDisplay(e.created_at)}</td>
															<td class="py-2 pr-4">
																<p class="font-medium">{row.label}</p>
																{#if row.sub}
																	<p class="mt-0.5 max-w-[420px] truncate text-xs text-muted-foreground">{row.sub}</p>
																{/if}
															</td>
															<td
																class="whitespace-nowrap py-2 text-right font-semibold tabular-nums"
																style="color: {row.tone === 'in'
																	? 'var(--success)'
																	: row.tone === 'out'
																		? 'var(--destructive)'
																		: 'var(--muted-foreground)'};"
															>
																{row.sign}{row.amount}
															</td>
														</tr>
													{/each}
												</tbody>
											</table>
										</div>
										{#if detail.total > detail.entries.length}
											<p class="mt-2 text-[11px] text-muted-foreground">
												{tr("Menampilkan")} {detail.entries.length} {tr("entri terbaru dari")} {fmt(detail.total)}.
											</p>
										{/if}
									{/if}
								</div>
							{/if}
						</li>
					{/each}
				</ul>

				<div class="flex flex-wrap items-center justify-between gap-2 border-t px-4 py-2.5">
					<span class="text-xs text-muted-foreground">
						{tr("Menampilkan")} {ledgerFrom}–{ledgerTo} {tr("dari")} {fmt(data.ledger_groups_total)} {tr("kampanye")}
					</span>
					<div class="flex gap-2">
						<button
							class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
							type="button"
							disabled={!ledgerHasPrev || loading}
							onclick={() => ledgerPage(-LEDGER_PAGE_SIZE)}
						>
							<ChevronLeft size={13} />{tr("Sebelumnya")}
						</button>
						<button
							class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
							type="button"
							disabled={!ledgerHasNext || loading}
							onclick={() => ledgerPage(LEDGER_PAGE_SIZE)}
						>
							{tr("Berikutnya")}<ChevronRight size={13} />
						</button>
					</div>
				</div>
			{/if}
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