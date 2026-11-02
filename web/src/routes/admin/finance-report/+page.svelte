<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Laporan keuangan: uang masuk dari topup klien yang lunas vs uang keluar
	// untuk pencairan mitra yang sudah ditransfer. Fokusnya arus kas — bukan
	// laba — jadi kompensasi saldo dan invoice manual ditampilkan terpisah.
	import { onMount } from 'svelte';
	import {
		ArrowDownLeft, ArrowLeftRight, ArrowUpRight, Download, FileText, Layers, RotateCcw, RotateCw, TriangleAlert, Upload, Wallet, X
	} from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import DatePicker from '$lib/components/date-picker.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime, fmtNum } from '$lib/admin-format';
	import { rp } from '$lib/credits/rupiah';
	import { platformError, isAccessDenied, ACCESS_DENIED } from '../_platform';
	import CashflowBars from './_cashflow-bars.svelte';
	import KpiSparkline from './_kpi-sparkline.svelte';

	type Entry = {
		id: string;
		direction: 'in' | 'out';
		occurred_at: string | null;
		kind: string;
		counterparty: string;
		reference: string;
		external_id?: string;
		method: string;
		gross_amount: number;
		fee_amount: number;
		amount: number;
		status: string;
	};
	type Summary = {
		money_in: number; money_in_count: number;
		money_out: number; money_out_count: number;
		money_out_gross: number; money_out_fee: number;
		net: number;
		topup_pending: number; topup_pending_count: number;
		settlement_pending: number; settlement_pending_count: number;
		client_balance: number; partner_unpaid: number;
		partner_payable_total?: number; partner_payable_count?: number;
		invoice_paid: number; invoice_paid_count: number;
		compensation_credits: number; compensation_count: number;
	};
	type PayableSettlement = {
		id: string; partner_id: string; partner_name: string | null; status: string;
		net_amount_idr: number; total_amount: number; transfer_fee_idr: number | null;
		payout_method: string | null; invoice_number: string | null;
		bank_name: string | null; bank_account_number: string | null; bank_account_name: string | null;
		created_at: string | null;
	};
	type Monthly = { month: string; label: string; money_in: number; money_out: number };
	type SparkPoint = { label: string; money_in: number; money_out: number; net: number; payable?: number };
	type Report = {
		from: string; to: string; summary: Summary; monthly: Monthly[];
		sparkline?: { granularity: 'day' | 'month'; points: SparkPoint[] };
		entries: Entry[]; total: number; limit: number; offset: number;
	};

	type Preset = 'bulan-ini' | 'bulan-lalu' | '7-hari' | '30-hari' | 'tahun-ini' | 'semua' | 'kustom';

	const PAGE_SIZE = 20;

	const pad2 = (n: number) => String(n).padStart(2, '0');
	const isoLocal = (d: Date) => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
	const startOfToday = () => {
		const now = new Date();
		return new Date(now.getFullYear(), now.getMonth(), now.getDate());
	};

	function presetRange(key: Preset): { from: string; to: string } {
		const today = startOfToday();
		const year = today.getFullYear();
		const month = today.getMonth();
		switch (key) {
			case 'bulan-lalu':
				return { from: isoLocal(new Date(year, month - 1, 1)), to: isoLocal(new Date(year, month, 0)) };
			case '7-hari':
				return { from: isoLocal(new Date(year, month, today.getDate() - 6)), to: isoLocal(today) };
			case '30-hari':
				return { from: isoLocal(new Date(year, month, today.getDate() - 29)), to: isoLocal(today) };
			case 'tahun-ini':
				return { from: isoLocal(new Date(year, 0, 1)), to: isoLocal(today) };
			case 'semua':
				// Titik awal sebenarnya ditentukan API (aktivitas paling awal);
				// nilai ini hanya placeholder sebelum respons datang.
				return { from: '2000-01-01', to: isoLocal(today) };
			case 'bulan-ini':
			case 'kustom':
			default:
				return { from: isoLocal(new Date(year, month, 1)), to: isoLocal(today) };
		}
	}

	const PRESETS: Array<{ value: Preset; label: string }> = [
		{ value: 'bulan-ini', label: 'Bulan ini' },
		{ value: 'bulan-lalu', label: 'Bulan lalu' },
		{ value: '7-hari', label: '7 hari' },
		{ value: '30-hari', label: '30 hari' },
		{ value: 'tahun-ini', label: 'Tahun ini' },
		{ value: 'semua', label: 'Semua' }
	];

	const METHOD_LABEL: Record<string, string> = {
		bank: 'Transfer bank', ewallet: 'E-wallet', xendit: 'Xendit', duitku: 'Duitku',
		midtrans: 'Midtrans', tripay: 'Tripay', sandbox: 'Sandbox', sumopod: 'Sumopod'
	};
	const methodLabel = (value: string) => METHOD_LABEL[value] ?? (value || '—');

	const initial = presetRange('tahun-ini');

	let from = $state(initial.from);
	let to = $state(initial.to);
	let preset = $state<Preset>('tahun-ini');
	let direction = $state('');
	let pageNum = $state(1);
	let data = $state<Report | null>(null);
	let loading = $state(true);
	let refreshing = $state(false);
	let exporting = $state(false);
	let error = $state('');

	// Popup "Siap transfer": pengajuan pencairan mitra yang belum ditransfer.
	let payablesOpen = $state(false);
	let payablesLoading = $state(false);
	let payablesError = $state('');
	let payables = $state<PayableSettlement[] | null>(null);
	// Langkah transfer per pengajuan (satu yang terbuka pada satu waktu).
	let transferId = $state<string | null>(null);
	let transferRef = $state('');
	let transferFile = $state<File | null>(null);
	// Kotak unggah bukti transfer: tarik & lepas + pratinjau gambar. Batasnya
	// sama dengan handler settlement receipt (gambar/PDF, maksimal 10 MB).
	let transferFileDragOver = $state(false);
	let transferFileError = $state('');
	let transferPreviewUrl = $state<string | null>(null);
	let transferBusy = $state(false);
	let transferError = $state('');
	let copiedId = $state<string | null>(null);

	function clearTransferPreview() {
		if (transferPreviewUrl) URL.revokeObjectURL(transferPreviewUrl);
		transferPreviewUrl = null;
	}

	const TRANSFER_FILE_MAX = 10 * 1024 * 1024;

	function fmtBytes(n: number) {
		return n >= 1024 * 1024
			? `${(n / (1024 * 1024)).toFixed(1).replace('.', ',')} MB`
			: `${Math.max(1, Math.round(n / 1024))} KB`;
	}

	/** Validasi di klien supaya admin tidak menunggu unggahan ditolak server. */
	function pickTransferFile(file: File | null) {
		transferFileError = '';
		if (!file) return;
		const isPDF = file.type === 'application/pdf' || /\.pdf$/i.test(file.name);
		const isImage = file.type.startsWith('image/') || /\.(jpe?g|png|webp|gif|heic|heif)$/i.test(file.name);
		if (!isPDF && !isImage) {
			transferFileError = tr('Format file harus berupa gambar (JPG/PNG) atau PDF.');
			return;
		}
		if (file.size > TRANSFER_FILE_MAX) {
			transferFileError = tr('Ukuran file maksimal 10 MB.');
			return;
		}
		clearTransferPreview();
		transferFile = file;
		transferPreviewUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : null;
	}

	function resetTransferFile() {
		transferFile = null;
		transferFileDragOver = false;
		transferFileError = '';
		clearTransferPreview();
	}

	function cancelTransfer() {
		transferId = null;
		transferError = '';
		resetTransferFile();
	}

	async function loadPayables() {
		payablesLoading = true;
		payablesError = '';
		try {
			const res = await apiRequest('/v1/platform/finance/partner-payables');
			if (!res.ok) {
				payablesError = res.status === 404
					? tr('Endpoint rincian belum tersedia di API yang sedang berjalan. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as { settlements?: PayableSettlement[] };
			payables = body.settlements ?? [];
		} catch {
			payablesError = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			payablesLoading = false;
		}
	}

	function openPayables() {
		payablesOpen = true;
		transferId = null;
		void loadPayables();
	}

	function closePayables() {
		payablesOpen = false;
		transferId = null;
		transferError = '';
		resetTransferFile();
	}

	function startTransfer(id: string) {
		transferId = id;
		transferRef = '';
		transferError = '';
		resetTransferFile();
	}

	async function copyText(id: string, text: string | null) {
		if (!text) return;
		try {
			await navigator.clipboard.writeText(text);
			copiedId = id;
			setTimeout(() => { if (copiedId === id) copiedId = null; }, 1500);
		} catch {
			/* clipboard bisa diblokir browser; nomor tetap terlihat di layar */
		}
	}

	// Sama dengan alur di halaman Pencairan: setujui bila masih menunggu, unggah
	// bukti (opsional), lalu tandai lunas. Aplikasi hanya mencatat; uangnya
	// ditransfer admin sendiri lewat bank.
	async function confirmTransfer(row: PayableSettlement) {
		transferBusy = true;
		transferError = '';
		try {
			if (row.status === 'pending') {
				const approve = await apiRequest(`/v1/platform/settlements/${row.id}`, {
					method: 'PUT',
					body: JSON.stringify({ status: 'approved' })
				});
				if (!approve.ok) {
					transferError = await platformError(approve);
					return;
				}
			}
			let receiptUrl: string | null = null;
			if (transferFile) {
				const fd = new FormData();
				fd.append('file', transferFile);
				const up = await apiRequest(`/v1/platform/settlements/${row.id}/receipt`, { method: 'POST', body: fd });
				if (!up.ok) {
					transferError = await platformError(up);
					return;
				}
				receiptUrl = ((await up.json()) as { receipt_url?: string }).receipt_url ?? null;
			}
			const paid = await apiRequest(`/v1/platform/settlements/${row.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					status: 'paid',
					payment_reference: transferRef.trim() || null,
					receipt_url: receiptUrl
				})
			});
			if (!paid.ok) {
				transferError = await platformError(paid);
				return;
			}
			transferId = null;
			resetTransferFile();
			await Promise.all([loadPayables(), load(pageNum)]);
		} catch {
			transferError = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			transferBusy = false;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(1);
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function buildParams(extra: Record<string, string> = {}, targetPage = pageNum) {
		const params = new URLSearchParams({
			from,
			to,
			limit: String(PAGE_SIZE),
			offset: String((targetPage - 1) * PAGE_SIZE),
			...extra
		});
		if (direction) params.set('type', direction);
		if (preset === 'semua') params.set('range', 'all');
		return params;
	}

	async function load(targetPage: number, manual = false) {
		if (manual) refreshing = true;
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/finance/report?${buildParams({}, targetPage).toString()}`);
			if (!res.ok) {
				const message = await platformError(res);
				// 404 teks polos dari chi = route belum ada di binary API yang
				// berjalan (bukan penolakan akses, yang memakai envelope JSON).
				error = res.status === 404 && message !== ACCESS_DENIED
					? tr('Endpoint laporan belum tersedia di API yang sedang berjalan. Restart API lalu coba lagi.')
					: message;
				return;
			}
			data = await res.json();
			// "Semua" dimulai dari transaksi paling awal; API mengembalikan
			// rentang efektifnya supaya date picker ikut menampilkan itu.
			if (preset === 'semua' && data && data.from && data.to) {
				from = data.from;
				to = data.to;
			}
			pageNum = targetPage;
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	function applyPreset(key: Preset) {
		preset = key;
		if (key === 'kustom') return;
		const range = presetRange(key);
		from = range.from;
		to = range.to;
		void load(1);
	}

	function onDateChange() {
		preset = 'kustom';
		void load(1);
	}

	function selectMonth(month: string) {
		const [year, monthNum] = month.split('-').map(Number);
		from = isoLocal(new Date(year, monthNum - 1, 1));
		to = isoLocal(new Date(year, monthNum, 0));
		preset = 'kustom';
		void load(1);
	}

	function resetFilters() {
		direction = '';
		applyPreset('tahun-ini');
	}

	async function exportCsv() {
		exporting = true;
		error = '';
		try {
			const params = buildParams({ format: 'csv', limit: '0', offset: '0' });
			const res = await apiRequest(`/v1/platform/finance/report?${params.toString()}`);
			if (!res.ok) {
				error = await platformError(res);
				return;
			}
			const blob = await res.blob();
			const a = document.createElement('a');
			a.href = URL.createObjectURL(blob);
			a.download = `tauthub-laporan-keuangan-${to}.csv`;
			a.click();
			URL.revokeObjectURL(a.href);
		} catch {
			error = tr('Tidak dapat mengunduh CSV. Coba lagi.');
		} finally {
			exporting = false;
		}
	}

	const summary = $derived(data?.summary);
	const entries = $derived(data?.entries ?? []);
	const monthly = $derived(data?.monthly ?? []);
	const denied = $derived(
		Boolean(!loading && !data && (isAccessDenied(error) || error.includes('tidak memiliki akses')))
	);
	const netNegative = $derived(Boolean(summary && summary.net < 0));
	const sparkPoints = $derived(data?.sparkline?.points ?? []);
	const sparkIn = $derived(sparkPoints.map((p) => ({ label: p.label, value: p.money_in })));
	const sparkOut = $derived(sparkPoints.map((p) => ({ label: p.label, value: p.money_out })));
	const sparkNet = $derived(sparkPoints.map((p) => ({ label: p.label, value: p.net })));
	const sparkPayable = $derived(sparkPoints.map((p) => ({ label: p.label, value: p.payable ?? 0 })));
	const netLabel = $derived(
		!summary ? '' : summary.net === 0 ? tr('Seimbang') : summary.net > 0 ? tr('Surplus') : tr('Defisit')
	);

	// Kompatibilitas saat deploy: respons lama (sebelum field sparkline/payable
	// ada) tidak punya seri grafik. Ambil ulang sekali supaya sparkline langsung
	// terisi tanpa perlu reload manual.
	let sparklineBackfill = false;
	$effect(() => {
		const stale = data && (!data.sparkline || data.sparkline.points.some((p) => p.payable === undefined));
		if (!sparklineBackfill && stale) {
			sparklineBackfill = true;
			void load(pageNum);
		}
	});
</script>

<section>
	<PageHeader title={tr("Keuangan Tauthub")}>
		<button class="app-btn app-btn-soft headerBtn" type="button" disabled={refreshing} onclick={() => { void load(pageNum, true); }}>
			<RotateCw size={14} />{tr("Muat ulang")}
		</button>
		<button class="app-btn app-btn-primary headerBtn" type="button" disabled={exporting || loading} onclick={() => void exportCsv()}>
			<Download size={14} />{exporting ? tr('Mengekspor…') : 'Export CSV'}
		</button>
	</PageHeader>

	{#if loading && !data}
		<Feedback loading loadingText={tr("Memuat laporan keuangan…")} rows={7} />
	{:else if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke laporan keuangan.")}</p>
		</div>
	{:else if data && summary}
		{#if error}
			<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
		{/if}

		<!-- Filter periode & rentang tanggal -->
		<div class="app-card mt-5 flex flex-col gap-3 px-4 py-3.5 xl:flex-row xl:items-center xl:justify-between">
			<div class="flex min-w-0 flex-wrap items-center gap-2">
				<span class="shrink-0 text-xs font-medium text-muted-foreground">{tr("Periode")}</span>
				<div class="periodScroll">
					<div class="periodGroup" role="group" aria-label={tr("Periode laporan")}>
						{#each PRESETS as item (item.value)}
							<button
								class="periodBtn"
								class:periodActive={preset === item.value}
								type="button"
								aria-pressed={preset === item.value}
								onclick={() => applyPreset(item.value)}
							>{tr(item.label)}</button>
						{/each}
					</div>
				</div>
			</div>
			<div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end xl:border-l xl:pl-4">
				<label class="flex w-full flex-col gap-1 sm:w-auto sm:min-w-[168px] sm:flex-none">
					<span class="text-[11px] font-medium text-muted-foreground">{tr("Dari tanggal")}</span>
					<DatePicker bind:value={from} max={to} ariaLabel={tr("Dari tanggal")} onchange={onDateChange} />
				</label>
				<label class="flex w-full flex-col gap-1 sm:w-auto sm:min-w-[168px] sm:flex-none">
					<span class="text-[11px] font-medium text-muted-foreground">{tr("Sampai tanggal")}</span>
					<DatePicker bind:value={to} min={from} ariaLabel={tr("Sampai tanggal")} onchange={onDateChange} />
				</label>
				<button class="resetBtn w-full sm:w-auto" type="button" onclick={resetFilters}>
					<RotateCcw size={13} />{tr("Reset filter")}
				</button>
			</div>
		</div>

		<!-- Empat angka utama -->
		<div class="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
			<article class="kpiCard kpiIn">
				<div class="flex items-center justify-between gap-2">
					<div class="flex min-w-0 items-center gap-2.5">
						<span class="kpiIcon kpiIconIn"><ArrowDownLeft size={15} /></span>
						<span class="kpiLabel">{tr("Uang Masuk")}</span>
					</div>
					<span class="kpiBadge kpiBadgeIn">{fmtNum(summary.money_in_count)} {tr("topup")}</span>
				</div>
				<p class="kpiValue kpiValueIn mt-3">{rp(summary.money_in)}</p>
				<div class="mt-auto flex items-end justify-between gap-3 pt-3">
					<p class="kpiDesc">{tr("Total top-up yang diterima pada periode ini.")}</p>
					<KpiSparkline points={sparkIn} color="#5dae8b" />
				</div>
			</article>

			<article class="kpiCard kpiOut">
				<div class="flex items-center justify-between gap-2">
					<div class="flex min-w-0 items-center gap-2.5">
						<span class="kpiIcon kpiIconOut"><ArrowUpRight size={15} /></span>
						<span class="kpiLabel">{tr("Uang Keluar")}</span>
					</div>
					<span class="kpiBadge kpiBadgeOut">{fmtNum(summary.money_out_count)} {tr("pencairan")}</span>
				</div>
				<p class="kpiValue kpiValueOut mt-3">{rp(summary.money_out)}</p>
				<div class="mt-auto flex items-end justify-between gap-3 pt-3">
					<p class="kpiDesc">
						{tr("Total pencairan mitra pada periode ini.")}
						{#if summary.money_out_fee > 0}
							<span class="block">{tr("Bruto")} {rp(summary.money_out_gross)} · {tr("biaya transfer")} {rp(summary.money_out_fee)}</span>
						{/if}
					</p>
					<KpiSparkline points={sparkOut} color="#da5c53" />
				</div>
			</article>

			<article class="kpiCard kpiNet">
				<div class="flex items-center justify-between gap-2">
					<div class="flex min-w-0 items-center gap-2.5">
						<span class="kpiIcon kpiIconNet"><Layers size={15} /></span>
						<Tooltip content={tr("Uang masuk dikurangi uang keluar pada periode ini. Ini arus kas, bukan laba.")}>
							<span class="kpiLabel">{tr("Arus Kas Bersih")}</span>
						</Tooltip>
					</div>
					<span
						class="kpiBadge"
						class:kpiBadgeNet={summary.net > 0}
						class:kpiBadgeNetNeg={summary.net < 0}
						class:kpiBadgeNetZero={summary.net === 0}
					>{netLabel}</span>
				</div>
				<p class="kpiValue kpiValueNet mt-3">
					{netNegative ? `− ${rp(Math.abs(summary.net))}` : rp(summary.net)}
				</p>
				<div class="mt-auto flex items-end justify-between gap-3 pt-3">
					<p class="kpiDesc">{tr("Selisih uang masuk dan keluar.")}</p>
					<KpiSparkline points={sparkNet} color="var(--primary)" />
				</div>
			</article>

			<button type="button" class="kpiCard kpiPayable" aria-haspopup="dialog" onclick={openPayables}>
				<div class="flex items-center justify-between gap-2">
					<div class="flex min-w-0 items-center gap-2.5">
						<span class="kpiIcon kpiIconPayable"><Wallet size={15} /></span>
						<Tooltip content={tr("Saldo yang sudah diminta mitra dan siap kamu transfer. Jumlahnya sudah dipotong biaya transfer.")}>
							<span class="kpiLabel">{tr("Siap transfer ke mitra")}</span>
						</Tooltip>
					</div>
					<span class="kpiBadge kpiBadgePayable">{fmtNum(summary.partner_payable_count ?? 0)} {tr("mitra")}</span>
				</div>
				<p class="kpiValue kpiValuePayable mt-3">{rp(summary.partner_payable_total ?? summary.settlement_pending)}</p>
				<div class="mt-auto flex items-end justify-between gap-3 pt-3">
					<p class="kpiDesc">{tr("Posisi saat ini")} · <span class="payableLink">{tr('Buka rincian')}</span></p>
					<KpiSparkline points={sparkPayable} color="#ff933e" />
				</div>
			</button>
		</div>

		{#if payablesOpen}
			<Modal title={tr("Siap transfer ke mitra")} maxWidth="2xl" onClose={closePayables}>
				<p class="text-sm text-muted-foreground">
					{tr("Saldo yang diminta mitra dan belum kamu transfer. Jumlah sudah dipotong biaya transfer.")}
				</p>

				{#if payablesLoading && !payables}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if payablesError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{payablesError}</p>
				{:else if payables && payables.length === 0}
					<p class="payableEmpty mt-4">{tr("Tidak ada pengajuan yang menunggu transfer saat ini.")}</p>
				{:else if payables}
					<div class="payableSum mt-4">
						<span class="text-xs text-muted-foreground">{tr("Total siap transfer")}</span>
						<span class="payableAmount text-lg font-semibold tabular-nums">{rp(payables.reduce((sum, r) => sum + (r.net_amount_idr ?? 0), 0))}</span>
						<span class="text-xs text-muted-foreground">· {fmtNum(payables.length)} {tr("pengajuan")}</span>
					</div>

					<ul class="mt-3 space-y-3">
						{#each payables as row (row.id)}
							<li class="payableCard">
								<div class="flex flex-wrap items-start justify-between gap-3">
									<div class="min-w-0">
										<p class="truncate text-sm font-semibold">{row.partner_name || row.partner_id.slice(0, 8)}</p>
										<p class="mt-0.5 text-[11px] text-muted-foreground">
											{row.invoice_number ?? ''}{#if row.created_at} · {tr("diajukan")} {fmtDateTime(row.created_at)}{/if}
										</p>
									</div>
									<div class="text-right">
										<p class="payableAmount text-lg font-semibold tabular-nums">{rp(row.net_amount_idr)}</p>
										<p class="text-[11px] text-muted-foreground">{tr("setelah biaya transfer")} {rp(row.transfer_fee_idr ?? 0)}</p>
									</div>
								</div>

								<div class="payableBank mt-3">
									<div class="min-w-0 flex-1">
										<p class="text-[11px] text-muted-foreground">{methodLabel(row.payout_method ?? '')}{#if row.bank_name} · {row.bank_name}{/if}</p>
										<p class="truncate text-sm font-medium tabular-nums">{row.bank_account_number ?? tr('Rekening belum diisi mitra')}</p>
										{#if row.bank_account_name}<p class="truncate text-[11px] text-muted-foreground">{tr("a.n.")} {row.bank_account_name}</p>{/if}
									</div>
									{#if transferId !== row.id}
										<div class="payableActions">
											{#if row.bank_account_number}
												<button type="button" class="app-btn payableCopy" onclick={() => void copyText(row.id, row.bank_account_number)}>
													{copiedId === row.id ? tr('Tersalin') : tr('Salin nomor')}
												</button>
											{/if}
											<button
												type="button"
												class="app-btn payablePrimary"
												disabled={!row.bank_account_number}
												title={row.bank_account_number ? undefined : tr('Mitra belum mengisi rekening atau e-wallet.')}
												onclick={() => startTransfer(row.id)}
											>{tr("Transfer")}</button>
										</div>
									{/if}
								</div>

								{#if transferId === row.id}
									<div class="payableForm mt-3">
										<p class="text-xs text-muted-foreground">
											{tr("Transfer dulu lewat bank, lalu klik Sudah ditransfer. Aplikasi ini hanya mencatat; uangnya tidak dikirim otomatis.")}
										</p>
										<label class="mt-3 block">
											<span class="text-xs font-medium text-muted-foreground">{tr("Nomor referensi transfer (opsional)")}</span>
											<input class="app-input mt-1 w-full" bind:value={transferRef} maxlength="200" />
										</label>
										<div class="mt-3">
											<span class="text-xs font-medium text-muted-foreground">{tr("Bukti transfer (opsional)")}</span>
											<label
												class="transferDrop mt-1"
												style="border-color: {transferFileDragOver ? 'var(--primary)' : 'var(--border)'}; background: {transferFileDragOver
													? 'var(--accent)'
													: 'transparent'};"
												ondragover={(e) => {
													e.preventDefault();
													transferFileDragOver = true;
												}}
												ondragleave={() => (transferFileDragOver = false)}
												ondrop={(e) => {
													e.preventDefault();
													transferFileDragOver = false;
													pickTransferFile(e.dataTransfer?.files?.[0] ?? null);
												}}
											>
												{#if transferFile && transferPreviewUrl}
													<img src={transferPreviewUrl} alt={transferFile.name} class="transferPreviewImg" />
													<div class="transferPreviewMeta">
														<span class="min-w-0 flex-1 truncate text-xs text-muted-foreground">
															{transferFile.name} · {fmtBytes(transferFile.size)} · {tr('Klik untuk mengganti')}
														</span>
														<button
															type="button"
															class="app-btn app-btn-soft shrink-0 !min-h-7 !px-2.5 !text-xs"
															onclick={(e) => {
																e.preventDefault();
																e.stopPropagation();
																resetTransferFile();
															}}
														><X size={13} /> {tr('Hapus')}</button>
													</div>
												{:else if transferFile}
													<div class="flex w-full items-center gap-3 text-left">
														<span class="transferFileIcon"><FileText size={18} /></span>
														<div class="min-w-0 flex-1">
															<span class="block truncate text-sm font-medium">{transferFile.name}</span>
															<span class="text-xs text-muted-foreground">
																{fmtBytes(transferFile.size)} · {tr('Klik untuk mengganti')}
															</span>
														</div>
														<button
															type="button"
															class="app-btn app-btn-soft shrink-0 !min-h-7 !px-2.5 !text-xs"
															onclick={(e) => {
																e.preventDefault();
																e.stopPropagation();
																resetTransferFile();
															}}
														><X size={13} /> {tr('Hapus')}</button>
													</div>
												{:else}
													<Upload size={20} class="text-muted-foreground" />
													<span class="text-sm font-medium">{tr('Tarik & lepas bukti transfer di sini')}</span>
													<span class="text-xs text-muted-foreground">
														{tr('atau klik untuk memilih berkas · gambar atau PDF, maksimal 10 MB')}
													</span>
												{/if}
												<input
													class="sr-only"
													type="file"
													accept="image/*,application/pdf"
													onchange={(e) => {
														const input = e.currentTarget as HTMLInputElement;
														pickTransferFile(input.files?.[0] ?? null);
														// Kosongkan agar berkas yang sama bisa dipilih ulang.
														input.value = '';
													}}
												/>
											</label>
											{#if transferFileError}
												<span class="mt-1 block text-xs" style="color: var(--destructive);">{transferFileError}</span>
											{/if}
										</div>
										{#if transferError}
											<p class="mt-3 text-sm" style="color: var(--destructive);">{transferError}</p>
										{/if}
										<div class="mt-3 flex flex-wrap justify-end gap-2">
											<button type="button" class="app-btn app-btn-soft" disabled={transferBusy} onclick={cancelTransfer}>{tr("Batal")}</button>
											<button type="button" class="app-btn payablePrimary" disabled={transferBusy} onclick={() => void confirmTransfer(row)}>
												{transferBusy ? tr('Menyimpan…') : tr('Sudah ditransfer')}
											</button>
										</div>
									</div>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}

				{#snippet footer()}
					<a class="mr-auto self-center text-xs underline underline-offset-2" style="color: var(--primary);" href="/admin/settlements">{tr("Buka halaman pencairan")}</a>
					<button type="button" class="app-btn app-btn-soft" onclick={closePayables}>{tr("Tutup")}</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- Tren 12 bulan -->
		<div class="app-card mt-6 p-5">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-base font-semibold">{tr("Arus Kas 12 Bulan Terakhir")}</h2>
				<span class="text-xs text-muted-foreground">{tr("Selalu 12 bulan terakhir. Klik satu bulan untuk memfilter periode.")}</span>
			</div>
			<div class="mt-4">
				<CashflowBars rows={monthly} onSelect={selectMonth} />
			</div>
		</div>

		<!-- Rincian -->
		<div class="app-card mt-5">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
				<div class="flex flex-wrap items-center gap-2">
					<h2 class="text-base font-semibold">{tr("Rincian Arus Kas")}</h2>
					<span class="rounded-full px-2.5 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
						{fmtNum(data.total)} {tr("baris")}
					</span>
				</div>
				<div class="flex items-center gap-2">
					<Select
						ariaLabel={tr("Filter rincian arus")}
						minWidth={190}
						value={direction}
						placeholder={tr("Masuk + keluar")}
						onchange={(v) => {
							direction = v;
							void load(1);
						}}
						options={[
							{ value: '', label: tr('Masuk + keluar') },
							{ value: 'in', label: tr('Hanya masuk') },
							{ value: 'out', label: tr('Hanya keluar') }
						]}
					/>
					{#if direction}
						<button
							class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
							type="button"
							onclick={() => {
								direction = '';
								void load(1);
							}}
						>{tr("Bersihkan")}</button>
					{/if}
				</div>
			</div>

			<div class="overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 920px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">{tr("Tanggal")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Arus")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Keterangan")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Klien / Mitra")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Referensi")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Metode")}</th>
							<th class="px-4 py-2.5 text-right font-medium">{tr("Nominal")}</th>
						</tr>
					</thead>
					<tbody>
						{#each entries as entry (entry.id)}
							<tr class="border-t">
								<td class="whitespace-nowrap px-4 py-2.5 text-muted-foreground">{fmtDateTime(entry.occurred_at)}</td>
								<td class="px-4 py-2.5">
									<Badge
										label={entry.direction === 'in' ? tr('Masuk') : tr('Keluar')}
										tone={entry.direction === 'in' ? 'success' : 'danger'}
									/>
								</td>
								<td class="px-4 py-2.5">{entry.direction === 'in' ? tr('Top Up Saldo') : tr('Pencairan Mitra')}</td>
								<td class="px-4 py-2.5">
									<p class="font-medium">{entry.counterparty || '—'}</p>
								</td>
								<td class="px-4 py-2.5 font-mono text-xs text-muted-foreground" title={entry.external_id ? `${tr('ID transaksi')}: ${entry.external_id}` : undefined}>{entry.reference || '—'}</td>
								<td class="px-4 py-2.5">{methodLabel(entry.method)}</td>
								<td class="px-4 py-2.5 text-right">
									<p class="font-semibold tabular-nums" style="color: {entry.direction === 'in' ? 'var(--success)' : 'var(--destructive)'};">
										{entry.direction === 'in' ? '+' : '−'} {rp(entry.amount)}
									</p>
									{#if entry.direction === 'out' && entry.fee_amount > 0}
										<p class="text-[11px] text-muted-foreground tabular-nums">
											{tr("bruto")} {rp(entry.gross_amount)} · {tr("biaya")} {rp(entry.fee_amount)}
										</p>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>

				{#if entries.length === 0}
					<div class="flex flex-col items-center px-4 py-12 text-center">
						<ArrowLeftRight size={26} class="text-muted-foreground" />
						<p class="mt-2 text-sm text-muted-foreground">{tr("Tidak ada baris pada filter ini.")}</p>
					</div>
				{/if}
			</div>

			<div class="px-4 pb-4">
				<Pagination
					total={data.total}
					limit={PAGE_SIZE}
					offset={(pageNum - 1) * PAGE_SIZE}
					onChange={(next) => void load(Math.floor(next / PAGE_SIZE) + 1)}
				/>
			</div>
		</div>

		{#if summary.invoice_paid > 0 || summary.compensation_credits > 0}
			<p class="mt-3 text-xs text-muted-foreground">
				{tr("Di luar total arus kas")}:
				{#if summary.invoice_paid > 0}
					{tr("invoice lunas")} {rp(summary.invoice_paid)} ({fmtNum(summary.invoice_paid_count)}),
				{/if}
				{#if summary.compensation_credits > 0}
					{tr("kompensasi topup")} {rp(summary.compensation_credits)} ({fmtNum(summary.compensation_count)}).
				{/if}
				{tr("Keduanya bukan uang yang masuk lewat gateway pembayaran.")}
			</p>
		{/if}
	{:else if error}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<TriangleAlert size={26} class="mx-auto text-muted-foreground" />
			<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>
			<button class="app-btn app-btn-soft mt-4" type="button" onclick={() => void load(1, true)}>{tr("Coba lagi")}</button>
		</div>
	{:else}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Belum ada data laporan keuangan.")}</p>
		</div>
	{/if}
</section>

<style>
	/* Tombol aksi header: lebih ringkas dari app-btn bawaan supaya baris judul
	   tetap tenang dan tidak bersaing dengan angka KPI. */
	.headerBtn {
		height: 36px;
		padding: 0 13px;
		font-size: 12.5px;
	}

	/* Segmented control periode: track netral tipis, segmen aktif biru Tauthub.
	   Tinggi total 40px supaya sejajar dengan input tanggal dan Reset. */
	.periodScroll {
		max-width: 100%;
		overflow-x: auto;
		overflow-y: hidden;
		scrollbar-width: none;
		-webkit-overflow-scrolling: touch;
	}
	.periodScroll::-webkit-scrollbar {
		display: none;
	}
	.periodGroup {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 3px;
		border: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		border-radius: 9px;
		background: color-mix(in srgb, var(--secondary) 62%, var(--card));
	}
	.periodBtn {
		appearance: none;
		height: 32px;
		padding: 0 12px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		line-height: 1;
		white-space: nowrap;
		cursor: pointer;
		transition: background-color 0.16s ease, color 0.16s ease;
	}
	.periodBtn:hover {
		color: var(--primary);
		background: color-mix(in srgb, var(--primary) 6%, transparent);
	}
	.periodActive,
	.periodActive:hover {
		background: var(--primary);
		color: var(--primary-foreground);
	}
	.periodBtn:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}

	.resetBtn {
		appearance: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		height: 40px;
		padding: 0 14px;
		border: 1px solid var(--border);
		border-radius: 9px;
		background: var(--card);
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		white-space: nowrap;
		cursor: pointer;
		transition: color 0.16s ease, border-color 0.16s ease, background-color 0.16s ease;
	}
	.resetBtn:hover {
		color: var(--primary);
		border-color: color-mix(in srgb, var(--primary) 45%, white);
		background: var(--accent);
	}
	.resetBtn:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}

	/* KPI utama: nominal sebagai fokus, sparkline sebagai konteks tren.
	   Warna semantik hanya pada nominal, badge, ikon, dan garis grafik. */
	.kpiCard {
		display: flex;
		flex-direction: column;
		min-height: 148px;
		padding: 16px;
		border: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
		border-radius: 14px;
		background: var(--card);
		box-shadow: 0 1px 2px rgba(16, 24, 40, 0.04);
	}
	.kpiIn {
		background: color-mix(in srgb, #5dae8b 2%, var(--card));
		border-color: color-mix(in srgb, #5dae8b 16%, var(--border));
	}
	.kpiOut {
		background: color-mix(in srgb, #da5c53 2%, var(--card));
		border-color: color-mix(in srgb, #da5c53 15%, var(--border));
	}
	.kpiNet {
		background: color-mix(in srgb, var(--primary) 4%, var(--card));
		border-color: color-mix(in srgb, var(--primary) 24%, white);
	}
	.kpiIcon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 28px;
		height: 28px;
		border-radius: 9px;
	}
	.kpiIconIn {
		background: color-mix(in srgb, #5dae8b 11%, white);
		color: #5dae8b;
	}
	.kpiIconOut {
		background: color-mix(in srgb, #da5c53 10%, white);
		color: #da5c53;
	}
	.kpiIconNet {
		background: var(--accent);
		color: var(--primary);
	}
	.kpiLabel {
		overflow: hidden;
		font-size: 13px;
		font-weight: 600;
		color: var(--foreground);
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.kpiValue {
		font-size: 27px;
		font-weight: 700;
		line-height: 1.05;
		letter-spacing: -0.02em;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.kpiValueIn {
		color: #5dae8b;
	}
	.kpiValueOut {
		color: #da5c53;
	}
	.kpiValueNet {
		color: var(--primary);
	}
	.kpiDesc {
		min-width: 0;
		font-size: 11.5px;
		line-height: 1.45;
		color: var(--muted-foreground);
	}
	.kpiBadge {
		flex: 0 0 auto;
		padding: 3px 9px;
		border-radius: 999px;
		font-size: 11px;
		font-weight: 600;
		line-height: 1.45;
		white-space: nowrap;
	}
	.kpiBadgeIn {
		background: color-mix(in srgb, #5dae8b 10%, white);
		color: #5dae8b;
	}
	.kpiBadgeOut {
		background: color-mix(in srgb, #da5c53 9%, white);
		color: #da5c53;
	}
	.kpiBadgeNet {
		background: var(--accent);
		color: var(--accent-foreground);
	}
	.kpiBadgeNetNeg {
		background: color-mix(in srgb, #da5c53 9%, white);
		color: #da5c53;
	}
	.kpiBadgeNetZero {
		background: var(--secondary);
		color: var(--muted-foreground);
	}
	/* Kartu "Siap transfer ke mitra": tint tipis oranye dan seluruh kartu bisa
	   diklik untuk membuka rincian pengajuan. */
	.kpiPayable {
		text-align: left;
		cursor: pointer;
		background: color-mix(in srgb, #ff933e 6%, var(--card));
		border-color: color-mix(in srgb, #ff933e 26%, var(--border));
		transition: background-color 150ms ease, border-color 150ms ease;
	}
	.kpiPayable:hover {
		background: color-mix(in srgb, #ff933e 10%, var(--card));
		border-color: color-mix(in srgb, #ff933e 40%, var(--border));
	}
	.kpiPayable:focus-visible {
		outline: 2px solid #ff933e;
		outline-offset: 2px;
	}
	.kpiIconPayable {
		background: color-mix(in srgb, #ff933e 14%, white);
		color: #e07f22;
	}
	.kpiValuePayable {
		color: #ff933e;
	}
	.kpiBadgePayable {
		background: color-mix(in srgb, #ff933e 12%, white);
		color: #d97a1f;
	}
	.kpiPayable .payableLink {
		color: #d97a1f;
	}

	.payableAmount {
		color: var(--primary);
	}
	.payableLink {
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	.payableSum {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 8px;
		padding: 10px 14px;
		border: 1px solid color-mix(in srgb, var(--primary) 22%, transparent);
		border-radius: 10px;
		background-color: color-mix(in srgb, var(--primary) 5%, white);
	}
	.payableCard {
		padding: 14px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background-color: #fff;
		list-style: none;
	}
	.payableBank {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: 10px;
		background-color: var(--secondary);
	}
	.payableActions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: flex-end;
		gap: 8px;
		margin-left: auto;
	}
	.payableCopy {
		flex-shrink: 0;
		min-height: 40px;
		padding: 0 14px;
		font-size: 13px;
		font-weight: 500;
		color: var(--primary);
		border: 1px solid color-mix(in srgb, var(--primary) 30%, transparent);
		border-radius: 10px;
		background-color: #fff;
	}
	.payableCopy:hover {
		background-color: color-mix(in srgb, var(--primary) 5%, white);
	}
	.payableForm {
		padding: 12px;
		border: 1px dashed color-mix(in srgb, var(--primary) 38%, transparent);
		border-radius: 10px;
	}
	.transferDrop {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 16px;
		text-align: center;
		cursor: pointer;
		border: 1px dashed var(--border);
		border-radius: 12px;
		transition: border-color 150ms ease, background-color 150ms ease;
	}
	.transferDrop:focus-within {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.transferPreviewImg {
		width: 100%;
		max-height: 170px;
		object-fit: contain;
		border-radius: 8px;
		background-color: #fff;
	}
	.transferPreviewMeta {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		text-align: left;
	}
	.transferFileIcon {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		width: 40px;
		height: 40px;
		border-radius: 8px;
		color: var(--primary);
		background-color: color-mix(in srgb, var(--primary) 12%, var(--card));
	}
	.payablePrimary {
		min-height: 40px;
		padding: 0 18px;
		color: #fff;
		background-color: var(--primary);
		border-radius: 10px;
	}
	.payablePrimary:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--primary) 85%, #000);
	}
	.payablePrimary:disabled {
		opacity: 0.55;
	}
	.payableEmpty {
		padding: 12px 14px;
		font-size: 13px;
		color: var(--primary);
		border-radius: 10px;
		background-color: color-mix(in srgb, var(--primary) 6%, white);
	}
</style>
