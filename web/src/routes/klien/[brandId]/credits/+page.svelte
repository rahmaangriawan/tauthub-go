<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kredit Saldo — wallet, konfigurasi TC, ledger, transaksi (port section
	// credits app lama). Termasuk aksi Top Up dan laporan topup gagal.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ChevronLeft, ChevronRight, Coins, FileText, Lock, Plus, Wallet, X, Flag } from '@lucide/svelte';
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

	// Komposisi dana: tersedia / tertahan / terpakai dari angka wallet yang sama
	// dengan tiga kartu, supaya bilah di kartu utama membaca porsi, bukan data baru.
	const split = $derived.by(() => {
		const w = data?.wallet;
		const total = w ? w.available_tc + w.reserved_tc + w.captured_tc : 0;
		const pct = (n: number) => (total > 0 ? (n / total) * 100 : 0);
		return {
			total,
			available: pct(w?.available_tc ?? 0),
			reserved: pct(w?.reserved_tc ?? 0),
			captured: pct(w?.captured_tc ?? 0)
		};
	});

	// Penanda kampanye: warna dipilih dari nama (stabil antar muat ulang) di
	// antara lima rona lembut, agar baris mudah dibedakan saat memindai.
	const TILE_HUES = [214, 168, 32, 340, 262];
	const tileHue = (name: string) => {
		let h = 0;
		for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) % 997;
		return TILE_HUES[h % TILE_HUES.length];
	};
	const tileInitial = (name: string) => (name.trim().charAt(0) || '·').toUpperCase();

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

	type LedgerRow = { tone: 'in' | 'out' | 'hold'; sign: string; amount: string; label: string; title: string | null };

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
		const title = e.document_title?.trim() || null;
		const label = ledgerLabel(e);
		if (e.ref_type === 'reserve') return { tone: 'hold', sign: '', amount, label, title };
		const outgoing = e.ref_type === 'capture' || e.ref_type === 'refund' || e.amount_tc < 0;
		return outgoing
			? { tone: 'out', sign: '−', amount, label, title }
			: { tone: 'in', sign: '+', amount, label, title };
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

<section class="bal">
	<header class="bal-head">
		<div>
			<h1 class="page-title">{tr("Saldo")}</h1>
			<p class="page-desc">{tr("Saldo rupiah untuk eksekusi publikasi")}</p>
		</div>
		<div class="bal-actions">
			<button class="app-btn app-btn-soft" type="button" onclick={openReport}>
				<Flag size={16} strokeWidth={1.5} />{tr("Laporkan Topup Gagal")}</button>
			<button class="app-btn app-btn-primary" type="button" onclick={openTopUp}>
				<Plus size={16} strokeWidth={1.5} />{tr("Top Up Saldo")}</button>
		</div>
	</header>

	{#if loading}
		<p class="bal-sub bal-block">{tr("Memuat saldo…")}</p>
	{:else if data}
		{#if pendingTopups.length > 0}
			<div class="app-card bal-pend bal-block">
				<div>
					<p class="bal-pend-title">
						<span class="bal-dot" aria-hidden="true"></span>
						{pendingTopups.length === 1
							? tr("Ada pembayaran top up yang belum selesai")
							: `${pendingTopups.length} ${tr("tagihan top up menunggu pembayaran")}`}
					</p>
					<p class="bal-meta">
						{tr("Total tagihan menunggu pembayaran")} Rp {fmt(pendingTotalIdr)} · {tr("belum masuk saldo sampai pembayaran selesai")}
					</p>
				</div>
				{#if latestPending?.payment_url}
					<a class="app-btn app-btn-primary" href={latestPending.payment_url}>{tr("Lanjutkan pembayaran")}</a>
				{/if}
			</div>
		{/if}

		<div class="bal-stats bal-block">
			<div class="app-card bal-stat bal-stat-lead">
				<div class="bal-lead-top">
					<p class="bal-label">{tr("Saldo tersedia")}</p>
					<Wallet size={18} strokeWidth={1.5} />
				</div>
				<p class="bal-figure bal-figure-lead"><span class="bal-cur">Rp</span>{fmt(data.wallet.available_tc * rateIdr)}</p>
				{#if pendingTotalIdr > 0}
					<p class="bal-lead-note">+ Rp {fmt(pendingTotalIdr)} {tr("menunggu pembayaran")}</p>
				{/if}
				{#if split.total > 0}
					<div class="bal-split" role="img" aria-label="{tr('Tersedia')} {Math.round(split.available)}%, {tr('Tertahan')} {Math.round(split.reserved)}%, {tr('Terpakai')} {Math.round(split.captured)}%">
						<span class="bal-split-a" style="width: {split.available}%"></span>
						<span class="bal-split-r" style="width: {split.reserved}%"></span>
						<span class="bal-split-c" style="width: {split.captured}%"></span>
					</div>
					<p class="bal-split-key">
						<span><i class="bal-key bal-split-a"></i>{tr("Tersedia")} {Math.round(split.available)}%</span>
						<span><i class="bal-key bal-split-r"></i>{tr("Tertahan")} {Math.round(split.reserved)}%</span>
						<span><i class="bal-key bal-split-c"></i>{tr("Terpakai")} {Math.round(split.captured)}%</span>
					</p>
				{/if}
			</div>
			<div class="app-card bal-stat bal-stat-side">
				<span class="bal-icon bal-icon-hold"><Lock size={18} strokeWidth={1.5} /></span>
				<div>
					<p class="bal-label">{tr("Tertahan")}</p>
					<p class="bal-figure"><span class="bal-cur">Rp</span>{fmt(data.wallet.reserved_tc * rateIdr)}</p>
				</div>
			</div>
			<div class="app-card bal-stat bal-stat-side">
				<span class="bal-icon bal-icon-used"><Coins size={18} strokeWidth={1.5} /></span>
				<div>
					<p class="bal-label">{tr("Total terpakai")}</p>
					<p class="bal-figure"><span class="bal-cur">Rp</span>{fmt(data.wallet.captured_tc * rateIdr)}</p>
				</div>
			</div>
		</div>

		<div class="app-card bal-ledger bal-block">
			<div class="bal-ledger-head">
				<div>
					<h2 class="bal-h2">{tr("Riwayat penggunaan saldo")}</h2>
					<p class="bal-meta">{tr("Dikelompokkan per kampanye — klik untuk melihat rinciannya")}</p>
				</div>
				<span class="bal-meta">{fmt(data.ledger_groups_total)} {tr("kampanye")}</span>
			</div>

			{#if pendingTopups.length > 0}
				<div class="bal-ledger-pend">
					<span class="bal-dot" aria-hidden="true"></span>
					<span class="bal-ledger-pend-text">
						<span class="bal-row-title">{tr("menunggu pembayaran")} · {pendingTopups.length} {tr("tagihan")}</span>
						<span class="bal-meta">{tr("belum masuk saldo sampai pembayaran selesai")}</span>
					</span>
					<span class="bal-amt">+Rp {fmt(pendingTotalIdr)}</span>
				</div>
			{/if}

			{#if data.ledger_groups.length === 0}
				<p class="bal-empty">
					{tr("Belum ada mutasi saldo. Top up atau publikasi artikel untuk melihat riwayat.")}
				</p>
			{:else}
				<ul>
					{#each data.ledger_groups as g (g.campaign_id || 'none')}
						{@const key = g.campaign_id || 'none'}
						{@const detail = groupDetail[key]}
						{@const open = openGroup === key}
						<li class="bal-item">
							<button class="bal-row" type="button" aria-expanded={open} onclick={() => toggleGroup(g)}>
								<span class="bal-row-main">
									<ChevronRight size={16} strokeWidth={1.5} class="bal-chev {open ? 'bal-chev-open' : ''}" />
									{#if g.campaign_id}
										<span class="bal-tile" style="--h: {tileHue(g.campaign_name || g.campaign_id)}">{tileInitial(g.campaign_name || '·')}</span>
									{:else}
										<span class="bal-tile bal-tile-in"><Wallet size={16} strokeWidth={1.5} /></span>
									{/if}
									<span class="bal-row-text">
										<span class="bal-row-title">
											{g.campaign_name || tr("Lainnya — top up & penyesuaian saldo")}
										</span>
										<span class="bal-meta">
											{g.entries} {tr("transaksi")} · {jakartaDisplay(g.last_at)}
										</span>
									</span>
								</span>
								<span class="bal-row-side">
									{#if g.campaign_id}
										<span class="bal-amt" class:bal-amt-out={g.spent_tc > 0} class:bal-amt-muted={g.spent_tc <= 0}>
											{g.spent_tc > 0 ? `−${tcToRp(g.spent_tc, rateIdr)}` : tcToRp(0, rateIdr)}
										</span>
										{#if g.held_tc > 0}
											<span class="bal-meta">{tcToRp(g.held_tc, rateIdr)} {tr("ditahan")}</span>
										{/if}
									{:else}
										<span class="bal-amt" class:bal-amt-in={g.net_tc > 0} class:bal-amt-out={g.net_tc < 0} class:bal-amt-muted={g.net_tc === 0}>
											{g.net_tc > 0 ? '+' : g.net_tc < 0 ? '−' : ''}{tcToRp(Math.abs(g.net_tc), rateIdr)}
										</span>
									{/if}
								</span>
							</button>

							{#if open}
								<div class="bal-detail">
									{#if !detail || detail.loading}
										<p class="bal-meta">{tr("Memuat rincian…")}</p>
									{:else if detail.entries.length === 0}
										<p class="bal-meta">{tr("Tidak ada rincian untuk grup ini.")}</p>
									{:else}
										<div class="overflow-x-auto">
											<table class="bal-table">
												<thead>
													<tr>
														<th>{tr("Tanggal")}</th>
														<th>{tr("Keterangan")}</th>
														<th class="bal-th-end">{tr("Nominal")}</th>
													</tr>
												</thead>
												<tbody>
													{#each detail.entries as e (e.id)}
														{@const row = ledgerRow(e, rateIdr)}
														<tr>
															<td class="bal-td-date">{jakartaDisplay(e.created_at)}</td>
															<td>
																<p class="bal-row-title">{row.label}</p>
																{#if row.title}
																	<p class="bal-art" title={row.title}><FileText size={14} strokeWidth={1.5} />{row.title}</p>
																{/if}
															</td>
															<td class="bal-td-amt">
																<span class="bal-amt" class:bal-amt-in={row.tone === 'in'} class:bal-amt-out={row.tone === 'out'} class:bal-amt-muted={row.tone === 'hold'}>
																	{row.sign}{row.amount}
																</span>
															</td>
														</tr>
													{/each}
												</tbody>
											</table>
										</div>
										{#if detail.total > detail.entries.length}
											<p class="bal-meta bal-detail-note">
												{tr("Menampilkan")} {detail.entries.length} {tr("entri terbaru dari")} {fmt(detail.total)}.
											</p>
										{/if}
									{/if}
								</div>
							{/if}
						</li>
					{/each}
				</ul>

				<div class="bal-foot">
					<span class="bal-meta">
						{tr("Menampilkan")} {ledgerFrom}–{ledgerTo} {tr("dari")} {fmt(data.ledger_groups_total)} {tr("kampanye")}
					</span>
					<div class="bal-pager">
						<button
							class="app-btn app-btn-soft bal-pager-btn"
							type="button"
							disabled={!ledgerHasPrev || loading}
							onclick={() => ledgerPage(-LEDGER_PAGE_SIZE)}
						>
							<ChevronLeft size={16} strokeWidth={1.5} />{tr("Sebelumnya")}
						</button>
						<button
							class="app-btn app-btn-soft bal-pager-btn"
							type="button"
							disabled={!ledgerHasNext || loading}
							onclick={() => ledgerPage(LEDGER_PAGE_SIZE)}
						>
							{tr("Berikutnya")}<ChevronRight size={16} strokeWidth={1.5} />
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
					<h2 class="text-lg font-medium">{tr("Laporkan Topup Gagal")}</h2>
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

				<div class="flex justify-end gap-2 border-t pt-4">
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
	/* Halaman Saldo: ritme 4px, angka tabular. Warna nominal mengikuti arah uang:
	   merah = keluar, hijau = masuk, abu-abu = dana tertahan (belum terpakai). */
	.bal { display: flex; flex-direction: column; }
	.bal-block { margin-top: 24px; }

	.bal-head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: 16px; }
	.bal-sub { margin-top: 4px; font-size: 14px; line-height: 20px; color: var(--muted-foreground); }
	.bal-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.bal-actions :global(.app-btn) { font-weight: 500; }

	.bal-meta { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.bal-label { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.bal-h2 { font-size: 15px; line-height: 20px; font-weight: 500; }
	.bal-dot { display: inline-block; width: 8px; height: 8px; margin-right: 8px; border-radius: 9999px; background-color: var(--warning); }

	/* Tagihan yang belum selesai: satu-satunya blok yang meminta tindakan. */
	.bal-pend { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; padding: 16px 24px; }
	.bal-pend-title { display: flex; align-items: center; font-size: 14px; line-height: 20px; font-weight: 500; }
	.bal-pend .bal-meta { margin-top: 4px; }

	/* Tiga angka: yang pertama paling lebar, supaya saldo tersedia terbaca lebih dulu. */
	/* Tiga angka: kartu pertama adalah momen halaman, objek biru brand dengan bilah
	   komposisi dana. Dua lainnya putih dengan penanda warna sesuai segmen bilah. */
	.bal-stats { display: grid; grid-template-columns: 1fr; gap: 16px; }
	@media (min-width: 640px) { .bal-stats { grid-template-columns: 5fr 3fr; grid-template-rows: auto auto; } .bal-stat-lead { grid-row: span 2; } }
	.bal-stat { display: flex; flex-direction: column; gap: 4px; padding: 20px 24px; }
	.bal-stat-side { flex-direction: row; align-items: center; gap: 16px; }
	.bal-figure { font-size: 28px; line-height: 36px; font-weight: 500; font-variant-numeric: tabular-nums; }
	.bal-figure-lead { font-size: 40px; line-height: 48px; }
	.bal-cur { margin-right: 6px; font-size: 14px; font-weight: 400; color: var(--muted-foreground); }

	.bal-stat-lead {
		justify-content: space-between; gap: 8px; padding: 24px; color: #fff; border-color: transparent;
		background: linear-gradient(155deg, #3b77f2 0%, #2b67ec 45%, #1d4fc7 100%);
		box-shadow: 0 8px 24px rgba(43, 103, 236, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.18);
	}
	.bal-stat-lead .bal-label, .bal-stat-lead .bal-cur { color: rgba(255, 255, 255, 0.78); }
	.bal-lead-top { display: flex; align-items: center; justify-content: space-between; color: rgba(255, 255, 255, 0.78); }
	.bal-lead-note { font-size: 13px; line-height: 20px; color: rgba(255, 255, 255, 0.85); }
	.bal-split { display: flex; height: 8px; margin-top: 16px; overflow: hidden; border-radius: 9999px; background: rgba(255, 255, 255, 0.16); }
	.bal-split span { display: block; height: 100%; }
	.bal-split-a { background: #fff; }
	.bal-split-r { background: #ffc857; }
	.bal-split-c { background: rgba(255, 255, 255, 0.42); }
	.bal-split-key { display: flex; flex-wrap: wrap; gap: 4px 16px; font-size: 12px; line-height: 16px; color: rgba(255, 255, 255, 0.85); }
	.bal-split-key span { display: inline-flex; align-items: center; gap: 6px; }
	.bal-key { display: inline-block; width: 8px; height: 8px; border-radius: 9999px; }

	.bal-icon { display: inline-flex; width: 40px; height: 40px; flex-shrink: 0; align-items: center; justify-content: center; border-radius: 12px; }
	.bal-icon-hold { background: #fff4d6; color: #9a6700; }
	.bal-icon-used { background: #e8efff; color: #2b67ec; }

	.bal-ledger { padding: 0; }
	.bal-ledger-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; padding: 16px 24px; border-bottom: 1px solid var(--border); }
	.bal-ledger-head .bal-meta:first-child, .bal-ledger-head div .bal-meta { margin-top: 2px; }
	.bal-ledger-pend { display: flex; align-items: center; gap: 0; padding: 12px 24px; border-bottom: 1px solid var(--border); }
	.bal-ledger-pend-text { display: flex; flex: 1; min-width: 0; flex-direction: column; }
	.bal-empty { padding: 24px; font-size: 14px; line-height: 20px; color: var(--muted-foreground); }

	.bal-item { border-bottom: 1px solid var(--border); }
	.bal-item:last-child { border-bottom: 0; }
	.bal-row {
		display: flex; width: 100%; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px;
		padding: 12px 24px; text-align: left; transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.bal-row:hover { background-color: color-mix(in srgb, var(--foreground) 3%, transparent); }
	.bal-row:focus-visible { outline: 2px solid var(--primary); outline-offset: -2px; }
	.bal-row-main { display: flex; min-width: 0; align-items: center; gap: 12px; }
	.bal-row-text { display: flex; min-width: 0; flex-direction: column; }
	.bal-row-title { font-size: 14px; line-height: 20px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.bal-row-side { display: flex; flex-shrink: 0; flex-direction: column; align-items: flex-end; }
	:global(.bal-chev) { flex-shrink: 0; color: var(--muted-foreground); transition: transform 160ms cubic-bezier(0.22, 1, 0.36, 1); }
	:global(.bal-chev-open) { transform: rotate(90deg); }

	.bal-tile {
		display: inline-flex; width: 32px; height: 32px; flex-shrink: 0; align-items: center; justify-content: center;
		border-radius: 10px; font-size: 14px; font-weight: 500;
		background: hsl(var(--h) 85% 94%); color: hsl(var(--h) 60% 32%);
	}
	.bal-tile-in { background: #e3f6ec; color: #17803d; }

	.bal-amt { font-size: 14px; line-height: 20px; font-weight: 500; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.bal-amt-in { color: var(--success); }
	.bal-amt-out { color: var(--destructive); }
	.bal-amt-muted { color: var(--muted-foreground); }

	.bal-detail { padding: 12px 24px 16px 84px; border-top: 1px solid var(--border); background-color: color-mix(in srgb, var(--foreground) 2%, transparent); }
	.bal-table { width: 100%; border-collapse: collapse; font-size: 14px; }
	.bal-table th { padding: 4px 16px 8px 0; font-size: 12px; line-height: 16px; font-weight: 400; text-align: left; color: var(--muted-foreground); }
	.bal-table th.bal-th-end { padding-right: 0; text-align: right; }
	.bal-table td { padding: 8px 16px 8px 0; border-top: 1px solid var(--border); vertical-align: top; }
	.bal-td-date { white-space: nowrap; font-size: 12px; line-height: 20px; color: var(--muted-foreground); }
	.bal-td-amt { padding-right: 0; text-align: right; }
	/* Judul artikel di bawah keterangan: dibuat terbaca (bukan meta redup) supaya
	   klien langsung ingat biaya ini untuk artikel yang mana. */
	.bal-art { display: flex; align-items: flex-start; gap: 6px; margin-top: 4px; font-size: 13px; line-height: 20px; color: var(--accent-foreground); }
	.bal-art :global(svg) { flex-shrink: 0; margin-top: 3px; }
	.bal-detail-note { margin-top: 8px; }

	.bal-foot { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; padding: 12px 24px; border-top: 1px solid var(--border); }
	.bal-pager { display: flex; gap: 8px; }
	.bal-pager-btn { min-height: 32px; padding: 0 12px; font-size: 13px; font-weight: 500; }

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
		border-radius: 16px;
		padding: 24px;
		background-color: var(--card);
		color: var(--foreground);
		box-shadow: var(--shadow-md);
	}
</style>