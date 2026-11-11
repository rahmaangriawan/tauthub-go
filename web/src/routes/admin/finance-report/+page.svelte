<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Dashboard keuangan admin — fokus balance-sheet (posisi uang milik siapa)
	// + kemampuan drill-down ke detail per KPI. Pandangan arus kas periode
	// sengaja dihilangkan (menyesatkan: mencampur titipan klien dgn omzet
	// Tauthub); tetap ada chart Top Up vs GMV vs Pencairan untuk tren.
	import { onMount } from 'svelte';
	import {
		ArrowLeftRight, ChevronRight, Download, ExternalLink, RotateCcw, RotateCw, TriangleAlert
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
	type Recon = {
		gateway_in_gross: number; gateway_fee: number; gateway_in_net: number;
		admin_withdrawn: number; gateway_balance_est: number; required: number; gap: number;
		components: Array<{ key: string; label: string; amount: number; note: string }>;
		orphan_count: number;
		orphan_publications: Array<{ media: string; partner: string; client_price: number; credit: number }>;
	};
	type Summary = {
		money_in: number; money_in_count: number;
		money_in_fee?: number; money_in_net?: number;
		money_out: number; money_out_count: number;
		money_out_gross: number; money_out_fee: number;
		net: number;
		topup_pending: number; topup_pending_count: number;
		settlement_pending: number; settlement_pending_count: number;
		client_balance: number; partner_unpaid: number;
		partner_payable_total?: number; partner_payable_count?: number;
		invoice_paid: number; invoice_paid_count: number;
		compensation_credits: number; compensation_count: number;
		// Posisi keuangan (balance sheet): hadir di API baru. Opsional supaya
		// halaman tetap aman ketika binary API belum di-restart.
		client_available?: number;             // Dana Titipan Klien (uang nyata)
		client_adjusted?: number;              // Saldo Penyesuaian (bukan uang nyata)
		client_reserved?: number;              // Dana Tertahan (Escrow)
		tauthub_revenue_gross?: number;        // pendapatan Tauthub kotor (komisi 25% + porsi mitra internal)
		tauthub_commission_gross?: number;     // bagian Tauthub dari order mitra luar (kredit dibayar - bagian mitra)
		tauthub_nonpublication?: number;       // pemakaian kredit non-publikasi (AI, potongan manual)
		tauthub_gateway_fee?: number;          // fee gateway dari topup
		tauthub_internal_gross?: number;       // porsi mitra internal (akun milik Tauthub)
		tauthub_withdrawn?: number;            // sudah dicatat tertarik admin
		tauthub_available_to_withdraw?: number;// Profit Bersih siap ditarik
		cash_required_in_bank?: number;        // jumlah uang fisik wajib ada di rekening
		reconciliation?: Recon;                // pencocokan dengan saldo gateway
	};
	type Monthly = { month: string; label: string; money_in: number; money_out: number; gmv?: number };
	type DormantBrand = {
		brand_id: string; brand_name: string; client_name: string; ownership_type: string;
		available_tc: number; reserved_tc: number; total_tc: number;
		last_activity_at: string | null; days_dormant: number;
		owner_email: string; owner_name: string;
	};
	type AdminWithdrawal = {
		id: string; amount: number; note: string;
		bank_from: string; bank_to: string; reference: string;
		receipt_url: string; status: 'paid' | 'void';
		created_by: string; created_by_name: string;
		created_at: string; voided_at: string | null; void_reason: string;
	};
	type AdminWithdrawalsResponse = {
		entries: AdminWithdrawal[]; total: number; limit: number; offset: number;
		profit_gross: number; profit_withdrawn: number; profit_available: number;
	};

	// Detail breakdown per KPI balance-sheet (modal pop-up).
	type ClientDeposit = {
		brand_id: string; brand_name: string; client_name: string; ownership_type: string;
		available_tc: number; reserved_tc: number; total_tc: number;
		real_in?: number; real_tc?: number; adjusted_tc?: number;
		last_activity_at: string | null; owner_email: string; owner_name: string;
	};
	type EscrowItem = {
		publication_id: string; status: string;
		client_price: number; partner_price: number; created_at: string;
		brand_id: string; brand_name: string; client_name: string;
		media_id: string; media_name: string;
		partner_id: string; partner_name: string;
	};
	type PartnerUnpaid = {
		partner_id: string; partner_name: string; partner_email: string;
		amount_pending: number; amount_settled_unpaid: number; total: number;
		count_pending: number; count_settled_unpaid: number;
		last_earning_at: string | null;
	};
	type ProfitItem = {
		publication_id: string; client_price: number; credit: number;
		partner_outside: number; internal_share: number; revenue: number;
		published_at: string | null; published_url: string;
		brand_name: string; client_name: string; media_name: string; partner_name: string;
	};
	type ProfitMonth = { month: string; gmv: number; credit: number; revenue: number; count: number };
	type ProfitTotals = {
		commission: number; internal: number; non_publication: number; gateway_fee: number;
		gross: number; withdrawn: number; available: number;
	};
	type Report = {
		from: string; to: string; summary: Summary; monthly: Monthly[];
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
	let kindFilter = $state('');
	let pageNum = $state(1);
	let data = $state<Report | null>(null);
	let loading = $state(true);
	let refreshing = $state(false);
	let exporting = $state(false);
	let error = $state('');

	// Saldo klien mengendap >= 90 hari: posisi real-time, tidak ikut filter periode.
	let dormant = $state<DormantBrand[] | null>(null);
	let dormantError = $state('');

	// Riwayat penarikan profit admin + posisi profit.
	let withdrawals = $state<AdminWithdrawal[]>([]);
	let withdrawalsTotal = $state(0);
	let withdrawalsPage = $state(1);
	let withdrawalsLoading = $state(false);
	let withdrawalsError = $state('');
	let profitGross = $state(0);
	let profitWithdrawn = $state(0);
	let profitAvailable = $state(0);

	// Modal "Tarik Profit".
	let withdrawOpen = $state(false);
	let withdrawAmount = $state('');
	let withdrawNote = $state('');
	let withdrawBankFrom = $state('');
	let withdrawBankTo = $state('');
	let withdrawRef = $state('');
	let withdrawBusy = $state(false);
	let withdrawError = $state('');

	const WITHDRAW_PAGE_SIZE = 10;

	// Modal rincian per KPI balance-sheet.
	let depositsOpen = $state(false);
	let depositsScope = $state<'available' | 'reserved'>('available');
	let depositsLoading = $state(false);
	let depositsError = $state('');
	let depositsEntries = $state<ClientDeposit[] | null>(null);

	let escrowOpen = $state(false);
	let escrowLoading = $state(false);
	let escrowError = $state('');
	let escrowEntries = $state<EscrowItem[] | null>(null);
	let escrowTotalPrice = $state(0);

	let unpaidOpen = $state(false);
	let unpaidLoading = $state(false);
	let unpaidError = $state('');
	let unpaidEntries = $state<PartnerUnpaid[] | null>(null);

	let profitOpen = $state(false);
	let profitLoading = $state(false);
	let profitError = $state('');
	let profitEntries = $state<ProfitItem[] | null>(null);
	let profitTotalCount = $state(0);
	let profitMonthly = $state<ProfitMonth[]>([]);
	let profitTotals = $state<ProfitTotals | null>(null);

	let cashOpen = $state(false);


	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(1), loadDormant(), loadWithdrawals(1)]);
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
		if (kindFilter) params.set('kind', kindFilter);
		if (preset === 'semua') params.set('range', 'all');
		return params;
	}

	async function loadDormant() {
		dormantError = '';
		try {
			const res = await apiRequest('/v1/platform/finance/dormant-brands?days=90');
			if (!res.ok) {
				dormantError = res.status === 404
					? tr('Endpoint saldo mengendap belum tersedia di API yang sedang berjalan. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as { entries?: DormantBrand[] };
			dormant = body.entries ?? [];
		} catch {
			dormantError = tr('Tidak dapat terhubung ke server API.');
		}
	}

	async function loadWithdrawals(targetPage: number) {
		withdrawalsLoading = true;
		withdrawalsError = '';
		try {
			const params = new URLSearchParams({
				limit: String(WITHDRAW_PAGE_SIZE),
				offset: String((targetPage - 1) * WITHDRAW_PAGE_SIZE)
			});
			const res = await apiRequest(`/v1/platform/finance/admin-withdrawals?${params.toString()}`);
			if (!res.ok) {
				withdrawalsError = res.status === 404
					? tr('Endpoint penarikan profit belum tersedia di API yang sedang berjalan. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as AdminWithdrawalsResponse;
			withdrawals = body.entries ?? [];
			withdrawalsTotal = body.total ?? 0;
			profitGross = body.profit_gross ?? 0;
			profitWithdrawn = body.profit_withdrawn ?? 0;
			profitAvailable = body.profit_available ?? 0;
			withdrawalsPage = targetPage;
		} catch {
			withdrawalsError = tr('Tidak dapat terhubung ke server API.');
		} finally {
			withdrawalsLoading = false;
		}
	}

	function openWithdraw() {
		withdrawAmount = '';
		withdrawNote = '';
		withdrawBankFrom = '';
		withdrawBankTo = '';
		withdrawRef = '';
		withdrawError = '';
		withdrawOpen = true;
	}

	function closeWithdraw() {
		if (withdrawBusy) return;
		withdrawOpen = false;
	}

	async function submitWithdraw() {
		withdrawError = '';
		const amount = Number(withdrawAmount.replace(/[^\d]/g, ''));
		if (!amount || amount <= 0) {
			withdrawError = tr('Jumlah penarikan wajib diisi dan lebih dari 0.');
			return;
		}
		if (amount > profitAvailable) {
			withdrawError = tr('Jumlah melebihi profit yang tersedia untuk ditarik.');
			return;
		}
		withdrawBusy = true;
		try {
			const res = await apiRequest('/v1/platform/finance/admin-withdrawals', {
				method: 'POST',
				body: JSON.stringify({
					amount,
					note: withdrawNote.trim(),
					bank_from: withdrawBankFrom.trim(),
					bank_to: withdrawBankTo.trim(),
					reference: withdrawRef.trim()
				})
			});
			if (!res.ok) {
				withdrawError = await platformError(res);
				return;
			}
			withdrawOpen = false;
			await Promise.all([loadWithdrawals(1), load(pageNum)]);
		} catch {
			withdrawError = tr('Tidak dapat terhubung ke server API.');
		} finally {
			withdrawBusy = false;
		}
	}

	async function loadDeposits() {
		depositsLoading = true;
		depositsError = '';
		try {
			const res = await apiRequest('/v1/platform/finance/client-deposits-detail?limit=300');
			if (!res.ok) {
				depositsError = res.status === 404
					? tr('Endpoint rincian saldo klien belum tersedia. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as { entries?: ClientDeposit[] };
			depositsEntries = body.entries ?? [];
		} catch {
			depositsError = tr('Tidak dapat terhubung ke server API.');
		} finally {
			depositsLoading = false;
		}
	}
	function openDeposits(scope: 'available' | 'reserved') {
		depositsScope = scope;
		depositsOpen = true;
		if (depositsEntries === null) void loadDeposits();
	}

	async function loadEscrow() {
		escrowLoading = true;
		escrowError = '';
		try {
			const res = await apiRequest('/v1/platform/finance/escrow-detail?limit=300');
			if (!res.ok) {
				escrowError = res.status === 404
					? tr('Endpoint rincian escrow belum tersedia. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as { entries?: EscrowItem[]; total_client_price?: number };
			escrowEntries = body.entries ?? [];
			escrowTotalPrice = body.total_client_price ?? 0;
		} catch {
			escrowError = tr('Tidak dapat terhubung ke server API.');
		} finally {
			escrowLoading = false;
		}
	}
	function openEscrow() {
		escrowOpen = true;
		if (escrowEntries === null) void loadEscrow();
	}

	async function loadPartnerUnpaid() {
		unpaidLoading = true;
		unpaidError = '';
		try {
			const res = await apiRequest('/v1/platform/finance/partner-unpaid-detail?limit=500');
			if (!res.ok) {
				unpaidError = res.status === 404
					? tr('Endpoint rincian hak mitra belum tersedia. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as { entries?: PartnerUnpaid[] };
			unpaidEntries = body.entries ?? [];
		} catch {
			unpaidError = tr('Tidak dapat terhubung ke server API.');
		} finally {
			unpaidLoading = false;
		}
	}
	function openPartnerUnpaid() {
		unpaidOpen = true;
		if (unpaidEntries === null) void loadPartnerUnpaid();
	}

	async function loadProfit() {
		profitLoading = true;
		profitError = '';
		try {
			// Ikut filter periode utama supaya modal profit konsisten dengan KPI card.
			const params = new URLSearchParams({ limit: '100', from, to });
			const res = await apiRequest(`/v1/platform/finance/profit-breakdown?${params.toString()}`);
			if (!res.ok) {
				profitError = res.status === 404
					? tr('Endpoint rincian profit belum tersedia. Restart API lalu coba lagi.')
					: await platformError(res);
				return;
			}
			const body = (await res.json()) as {
				entries?: ProfitItem[]; total?: number;
				monthly?: ProfitMonth[]; totals?: ProfitTotals;
			};
			profitEntries = body.entries ?? [];
			profitTotalCount = body.total ?? 0;
			profitMonthly = body.monthly ?? [];
			profitTotals = body.totals ?? null;
		} catch {
			profitError = tr('Tidak dapat terhubung ke server API.');
		} finally {
			profitLoading = false;
		}
	}
	function openProfit() {
		profitOpen = true;
		// Selalu reload: filter periode mungkin sudah berubah sejak modal terakhir dibuka.
		void loadProfit();
	}

	// Label status publikasi untuk tabel escrow (statusnya internal, perlu ID friendly).
	const PUB_STATUS_LABEL: Record<string, string> = {
		pending: 'Menunggu review',
		waiting_partner: 'Menunggu mitra',
		partner_approved: 'Disetujui mitra',
		queued: 'Dalam antrean',
		processing: 'Sedang tayang',
		retrying: 'Coba ulang',
	};
	const pubStatusLabel = (s: string) => PUB_STATUS_LABEL[s] ?? s;

	async function voidWithdrawal(row: AdminWithdrawal) {
		const reason = window.prompt(tr('Alasan pembatalan (opsional):'), '');
		if (reason === null) return;
		const ok = window.confirm(tr('Batalkan pencatatan penarikan ini? Profit tersedia akan kembali naik sebesar jumlahnya.'));
		if (!ok) return;
		const res = await apiRequest(`/v1/platform/finance/admin-withdrawals/${row.id}/void`, {
			method: 'PUT',
			body: JSON.stringify({ reason })
		});
		if (!res.ok) {
			withdrawalsError = await platformError(res);
			return;
		}
		await Promise.all([loadWithdrawals(withdrawalsPage), load(pageNum)]);
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
		kindFilter = '';
		applyPreset('tahun-ini');
	}

	// Format rupiah untuk input "jumlah tarik profit" supaya mudah dibaca
	// saat admin mengetik angka besar. Simpan raw ke string state, format saat
	// ditampilkan di input.
	function onWithdrawAmountInput(e: Event) {
		const raw = (e.target as HTMLInputElement).value.replace(/[^\d]/g, '');
		withdrawAmount = raw ? Number(raw).toLocaleString('id-ID') : '';
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
	const netLabel = $derived(
		!summary ? '' : summary.net === 0 ? tr('Seimbang') : summary.net > 0 ? tr('Surplus') : tr('Defisit')
	);

</script>

<section class="fin">
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

		<!-- Filter periode & rentang tanggal untuk chart + tabel rincian -->
		<div class="app-card mt-6 flex flex-col gap-3 px-4 py-3.5 xl:flex-row xl:items-center xl:justify-between">
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

		<!-- Posisi keuangan sebagai satu neraca: angka wajib-ada, komposisinya,
		     dan pencocokan dengan saldo gateway. Mengikuti filter periode di atas;
		     akun internal dikecualikan di backend. -->
		{#if summary.client_available !== undefined}
			{@const total = summary.cash_required_in_bank ?? 0}
			{@const parts = [
				{ key: 'titipan', label: tr('Dana titipan klien'), hint: tr('Saldo klien dari topup asli. Milik klien, bukan uang Tauthub.'), value: summary.client_available ?? 0, color: 'var(--s1)', open: () => openDeposits('available') },
				{ key: 'mitra', label: tr('Hak mitra belum cair'), hint: tr('Penghasilan mitra yang belum ditransfer.'), value: summary.partner_unpaid ?? 0, color: 'var(--s2)', open: openPartnerUnpaid },
				{ key: 'profit', label: tr('Profit bersih Tauthub'), hint: tr('Uang yang benar-benar diterima Tauthub dan boleh ditarik.'), value: summary.tauthub_available_to_withdraw ?? 0, color: 'var(--s3)', open: openProfit },
				{ key: 'escrow', label: tr('Dana tertahan (escrow)'), hint: tr('Terkunci untuk order yang sedang dikerjakan mitra.'), value: summary.client_reserved ?? 0, color: 'var(--s4)', open: openEscrow }
			]}
			{@const rc = summary.reconciliation}
			<div class="app-card mt-5 grid overflow-hidden xl:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
				<div class="p-5 sm:p-6">
					<div class="flex flex-wrap items-baseline justify-between gap-2">
						<h2 class="text-sm font-semibold text-muted-foreground">{tr("Wajib ada di rekening")}</h2>
						<span class="text-[11px] tabular-nums text-muted-foreground">{from} → {to}</span>
					</div>
					<p class="finHero mt-2">{rp(total)}</p>

					<div class="finBar mt-5" role="img" aria-label={tr("Komposisi uang yang wajib ada di rekening")}>
						{#each parts as part (part.key)}
							{#if part.value > 0}
								<span class="finSeg" style="flex-grow: {part.value}; background-color: {part.color};" title={`${part.label}: ${rp(part.value)}`}></span>
							{/if}
						{/each}
					</div>

					<div class="mt-4">
						{#each parts as part (part.key)}
							<button type="button" class="finRow" onclick={part.open}>
								<span class="finSwatch" style="background-color: {part.color};"></span>
								<span class="min-w-0">
									<span class="block text-sm font-medium">{part.label}</span>
									<span class="block truncate text-xs text-muted-foreground">{part.hint}</span>
								</span>
								<span class="text-xs tabular-nums text-muted-foreground">{total > 0 ? Math.round((part.value / total) * 100) : 0}%</span>
								<span class="finAmount">{rp(part.value)}</span>
								<ChevronRight size={15} class="finChev" />
							</button>
						{/each}
					</div>

					{#if (summary.client_adjusted ?? 0) > 0}
						<button type="button" class="finNote mt-3" onclick={() => openDeposits('available')}>
							{tr("Saldo penyesuaian")} {rp(summary.client_adjusted ?? 0)} · {tr("bukan uang nyata, tidak dihitung")}
						</button>
					{/if}
				</div>

				<div class="finSide p-5 sm:p-6">
					<h2 class="text-sm font-semibold text-muted-foreground">{tr("Cocokkan dengan gateway")}</h2>
					{#if rc}
						{@const unexplained = rc.components.find((c) => c.key === 'other')?.amount ?? 0}
						<dl class="mt-4 space-y-3">
							<div class="flex items-baseline justify-between gap-3">
								<dt class="text-sm">{tr("Saldo gateway (perkiraan)")}</dt>
								<dd class="text-base font-semibold tabular-nums">{rp(rc.gateway_balance_est)}</dd>
							</div>
							<div class="flex items-baseline justify-between gap-3">
								<dt class="text-sm">{tr("Wajib ada, kumulatif")}</dt>
								<dd class="text-base font-semibold tabular-nums">{rp(rc.required)}</dd>
							</div>
							<div class="flex items-baseline justify-between gap-3 border-t pt-3">
								<dt class="text-sm font-semibold">{tr("Selisih")}</dt>
								<dd class="text-xl font-semibold tabular-nums">{rc.gap < 0 ? '− ' : ''}{rp(Math.abs(rc.gap))}</dd>
							</div>
						</dl>
						<div class="mt-4 flex flex-wrap items-center gap-2">
							<Badge
								label={unexplained === 0 ? tr('Selisih terjelaskan') : tr('Ada selisih belum tercatat')}
								tone={unexplained === 0 ? 'success' : 'warning'}
							/>
							{#if rc.orphan_count > 0}
								<Badge label={`${fmtNum(rc.orphan_count)} ${tr('publikasi tanpa penghasilan')}`} tone="warning" />
							{/if}
						</div>
						<p class="mt-3 text-xs text-muted-foreground">
							{tr("Net topup production")} {rp(rc.gateway_in_net)} · {tr("fee gateway")} {rp(rc.gateway_fee)}
						</p>
					{:else}
						<p class="mt-4 text-sm text-muted-foreground">{tr("Data rekonsiliasi belum tersedia.")}</p>
					{/if}
					<button type="button" class="app-btn app-btn-soft mt-5 w-full" onclick={() => (cashOpen = true)}>
						{tr("Lihat rekonsiliasi")}
					</button>
				</div>
			</div>
		{/if}

		<!-- Tren 12 bulan: top up, GMV, dan pencairan mitra -->
		<div class="app-card mt-6 overflow-hidden">
			<div class="finCardHead">
				<div>
					<h2 class="finTitle">{tr("Top up, GMV, dan pencairan")}</h2>
					<p class="finSub">
						{tr("12 bulan terakhir. Klik satu bulan untuk memfilter periode. GMV adalah nilai publikasi terbit; selisihnya dengan top up adalah saldo klien yang belum terpakai.")}
					</p>
				</div>
			</div>
			<div class="finCardBody">
				<CashflowBars rows={monthly} onSelect={selectMonth} />
			</div>
		</div>

		<!-- Saldo klien mengendap >= 90 hari -->
		<div class="app-card mt-5 overflow-hidden">
			<div class="finCardHead">
				<div>
					<h2 class="finTitle">{tr("Saldo klien mengendap ≥ 90 hari")}</h2>
					<p class="finSub">
						{tr("Brand yang masih punya saldo tetapi tidak ada aktivitas wallet selama 90 hari. Hubungi agar saldonya terpakai.")}
					</p>
				</div>
				{#if dormant && dormant.length > 0}
					<Badge label={`${fmtNum(dormant.length)} ${tr('brand')}`} tone="warning" />
				{/if}
			</div>
			<div class="finCardBody">
				{#if dormantError}
					<p class="text-sm" style="color: var(--destructive);">{dormantError}</p>
				{:else if dormant === null}
					<p class="text-sm text-muted-foreground">{tr("Memuat…")}</p>
				{:else if dormant.length === 0}
					<div class="finEmpty">{tr("Tidak ada saldo klien yang mengendap ≥ 90 hari.")}</div>
				{:else}
					<div class="finTableWrap">
						<table class="finTable" style="min-width: 800px;">
							<thead>
								<tr class="text-left">
									<th>{tr("Brand")}</th>
									<th>{tr("Klien / pemilik")}</th>
									<th class="num">{tr("Available")}</th>
									<th class="num">{tr("Reserved")}</th>
									<th class="num">{tr("Total")}</th>
									<th class="num">{tr("Terakhir aktif")}</th>
									<th></th>
								</tr>
							</thead>
							<tbody>
								{#each dormant as row (row.brand_id)}
									<tr>
										<td>
											<p class="font-medium">{row.brand_name}</p>
											<p class="finMuted">{row.ownership_type === 'client' ? tr('Klien') : tr('Internal')}</p>
										</td>
										<td>
											<p>{row.client_name || row.owner_name || '—'}</p>
											{#if row.owner_email}<p class="finMuted">{row.owner_email}</p>{/if}
										</td>
										<td class="num">{rp(row.available_tc)}</td>
										<td class="num" style="color: var(--muted-foreground);">{rp(row.reserved_tc)}</td>
										<td class="num font-semibold">{rp(row.total_tc)}</td>
										<td class="num">
											{#if row.last_activity_at}
												{fmtDateTime(row.last_activity_at)}
												<span class="finMuted block">{row.days_dormant} {tr("hari lalu")}</span>
											{:else}
												<span class="finMuted">{tr("Belum pernah")}</span>
											{/if}
										</td>
										<td class="num">
											{#if row.owner_email}
												<a
													class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
													href={`mailto:${row.owner_email}?subject=${encodeURIComponent(tr('Follow-up saldo kredit Tauthub'))}`}
												>{tr("Follow up")}</a>
											{:else}
												<span class="finMuted">—</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		</div>

		<!-- Rincian -->
		<div class="app-card mt-5 overflow-hidden">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
				<div class="flex flex-wrap items-center gap-2">
					<h2 class="finTitle">{tr("Rincian arus kas")}</h2>
					<span class="rounded-full px-2.5 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
						{fmtNum(data.total)} {tr("baris")}
					</span>
				</div>
				<div class="flex flex-wrap items-center gap-2">
					<Select
						ariaLabel={tr("Filter arah arus")}
						minWidth={170}
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
					<Select
						ariaLabel={tr("Filter jenis mutasi")}
						minWidth={210}
						value={kindFilter}
						placeholder={tr("Semua jenis mutasi")}
						onchange={(v) => {
							kindFilter = v;
							void load(1);
						}}
						options={[
							{ value: '', label: tr('Semua jenis mutasi') },
							{ value: 'topup', label: tr('Top Up Klien') },
							{ value: 'settlement', label: tr('Pencairan Mitra') },
							{ value: 'admin_withdrawal', label: tr('Penarikan Profit Admin') }
						]}
					/>
					{#if direction || kindFilter}
						<button
							class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
							type="button"
							onclick={() => {
								direction = '';
								kindFilter = '';
								void load(1);
							}}
						>{tr("Bersihkan")}</button>
					{/if}
				</div>
			</div>

			<!-- Ringkasan top up nyata pada periode, untuk dicocokkan dengan halaman
			     Payments di gateway (Total Amount, Total Fees, Net). -->
			<div class="finStats finStatsBleed">
				<div class="finStat">
					<span class="finStatLabel">{tr("Top up diterima")}</span>
					<span class="finStatValue">{fmtNum(summary.money_in_count)} <small>{tr("transaksi")}</small></span>
				</div>
				<div class="finStat">
					<span class="finStatLabel">{tr("Bruto")}</span>
					<span class="finStatValue">{rp(summary.money_in)}</span>
				</div>
				<div class="finStat">
					<span class="finStatLabel">{tr("Fee gateway")}</span>
					<span class="finStatValue">− {rp(summary.money_in_fee ?? 0)}</span>
				</div>
				<div class="finStat">
					<span class="finStatLabel">{tr("Net masuk wallet gateway")}</span>
					<span class="finStatValue" style="color: var(--success);">{rp(summary.money_in_net ?? summary.money_in)}</span>
				</div>
			</div>
			<p class="finFootnote">{tr("Sandbox dan akun uji tidak dihitung.")}</p>

			<div class="overflow-x-auto">
				<table class="finTable" style="min-width: 1100px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">{tr("Tanggal")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Arus")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Keterangan")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Klien / Mitra")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Referensi")}</th>
							<th class="px-4 py-2.5 font-medium">{tr("Metode")}</th>
							<th class="px-4 py-2.5 text-right font-medium">{tr("Bruto")}</th>
							<th class="px-4 py-2.5 text-right font-medium">{tr("Fee")}</th>
							<th class="px-4 py-2.5 text-right font-medium">{tr("Net")}</th>
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
								<td class="px-4 py-2.5">
									{#if entry.kind === 'admin_withdrawal'}
										{tr('Penarikan Profit Admin')}
									{:else if entry.direction === 'in'}
										{tr('Top Up Saldo')}
									{:else}
										{tr('Pencairan Mitra')}
									{/if}
								</td>
								<td class="px-4 py-2.5">
									<p class="font-medium">{entry.counterparty || '—'}</p>
								</td>
								<td class="px-4 py-2.5 font-mono text-xs text-muted-foreground" title={entry.external_id ? `${tr('ID transaksi')}: ${entry.external_id}` : undefined}>{entry.reference || '—'}</td>
								<td class="px-4 py-2.5">{methodLabel(entry.method)}</td>
								<td class="px-4 py-2.5 text-right tabular-nums">{rp(entry.gross_amount)}</td>
								<td class="px-4 py-2.5 text-right tabular-nums text-muted-foreground">
									{entry.fee_amount > 0 ? `− ${rp(entry.fee_amount)}` : '—'}
								</td>
								<td class="px-4 py-2.5 text-right font-semibold tabular-nums" style="color: {entry.direction === 'in' ? 'var(--success)' : 'var(--destructive)'};">
									{entry.direction === 'in' ? '+' : '−'} {rp(entry.amount)}
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

		<!-- Tarik profit admin: pencatatan pemindahan uang dari rekening escrow ke
		     rekening operasional. Aplikasi tidak mengirim uang; hanya mencatat. -->
		<div class="app-card mt-5 overflow-hidden">
			<div class="finCardHead">
				<div>
					<h2 class="finTitle">{tr("Tarik profit admin")}</h2>
					<p class="finSub">
						{tr("Catat setiap kali Anda memindahkan profit dari rekening escrow ke rekening operasional. Transfer tetap lewat m-banking; aplikasi hanya mencatat.")}
					</p>
				</div>
				<button type="button" class="app-btn app-btn-primary" disabled={profitAvailable <= 0} onclick={openWithdraw}>
					{tr("Catat penarikan")}
				</button>
			</div>
			<div class="finCardBody">
				<div class="finStats">
					<div class="finStat">
						<span class="finStatLabel">{tr("Pendapatan kotor")}</span>
						<span class="finStatValue">{rp(profitGross)}</span>
					</div>
					<div class="finStat">
						<span class="finStatLabel">{tr("Sudah ditarik atau dicairkan")}</span>
						<span class="finStatValue">{rp(profitWithdrawn)}</span>
					</div>
					<div class="finStat finStatKey">
						<span class="finStatLabel">{tr("Tersedia untuk ditarik")}</span>
						<span class="finStatValue" style="color: var(--success);">{rp(profitAvailable)}</span>
					</div>
				</div>

				{#if withdrawalsError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{withdrawalsError}</p>
				{/if}

				{#if withdrawalsLoading && withdrawals.length === 0}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat riwayat…")}</p>
				{:else if withdrawals.length === 0}
					<div class="finEmpty mt-4">{tr("Belum ada penarikan profit yang tercatat.")}</div>
				{:else}
					<div class="finTableWrap mt-4">
						<table class="finTable" style="min-width: 860px;">
							<thead>
								<tr class="text-left">
									<th>{tr("Tanggal")}</th>
									<th class="num">{tr("Jumlah")}</th>
									<th>{tr("Dari → ke")}</th>
									<th>{tr("Referensi")}</th>
									<th>{tr("Dicatat oleh")}</th>
									<th>{tr("Status")}</th>
									<th></th>
								</tr>
							</thead>
							<tbody>
								{#each withdrawals as row (row.id)}
									<tr>
										<td class="finMuted whitespace-nowrap">{fmtDateTime(row.created_at)}</td>
										<td class="num font-semibold" style="color: {row.status === 'void' ? 'var(--muted-foreground)' : 'var(--foreground)'};">
											{row.status === 'void' ? '' : '− '}{rp(row.amount)}
										</td>
										<td>
											{#if row.bank_from || row.bank_to}
												<p>{row.bank_from || '—'} <span class="finMuted">→</span> {row.bank_to || '—'}</p>
											{:else}
												<span class="finMuted">—</span>
											{/if}
											{#if row.note}<p class="finMuted">{row.note}</p>{/if}
										</td>
										<td class="font-mono finMuted">{row.reference || '—'}</td>
										<td>{row.created_by_name || '—'}</td>
										<td>
											<Badge
												label={row.status === 'void' ? tr('Dibatalkan') : tr('Tercatat')}
												tone={row.status === 'void' ? 'neutral' : 'success'}
											/>
											{#if row.status === 'void' && row.void_reason}
												<p class="finMuted mt-1">{row.void_reason}</p>
											{/if}
										</td>
										<td class="num">
											{#if row.status === 'paid'}
												<button
													type="button"
													class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
													onclick={() => void voidWithdrawal(row)}
												>{tr("Batalkan")}</button>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					{#if withdrawalsTotal > WITHDRAW_PAGE_SIZE}
						<div class="mt-3">
							<Pagination
								total={withdrawalsTotal}
								limit={WITHDRAW_PAGE_SIZE}
								offset={(withdrawalsPage - 1) * WITHDRAW_PAGE_SIZE}
								onChange={(next) => void loadWithdrawals(Math.floor(next / WITHDRAW_PAGE_SIZE) + 1)}
							/>
						</div>
					{/if}
				{/if}
			</div>
		</div>

		<!-- ============ Modal: catat penarikan profit ============ -->
		{#if withdrawOpen}
			<Modal title={tr("Catat penarikan profit")} maxWidth="lg" onClose={closeWithdraw}>
				<p class="finLead">
					{tr("Catat nominal yang Anda transfer dari rekening escrow ke rekening operasional. Aplikasi hanya mencatat; uangnya ditransfer sendiri lewat m-banking.")}
				</p>
				<div class="finStats mt-4">
					<div class="finStat finStatKey">
						<span class="finStatLabel">{tr("Tersedia untuk ditarik")}</span>
						<span class="finStatValue" style="color: var(--success);">{rp(profitAvailable)}</span>
					</div>
					<div class="finStat">
						<span class="finStatLabel">{tr("Sudah ditarik atau dicairkan")}</span>
						<span class="finStatValue">{rp(profitWithdrawn)}</span>
					</div>
				</div>
				<div class="mt-5 grid gap-4">
					<label class="finField">
						<span class="finFieldLabel">{tr("Jumlah (Rupiah)")}</span>
						<span class="finInputRow">
							<input
								class="app-input w-full tabular-nums"
								inputmode="numeric"
								placeholder="0"
								value={withdrawAmount}
								oninput={onWithdrawAmountInput}
							/>
							<button
								type="button"
								class="app-btn app-btn-soft shrink-0"
								onclick={() => (withdrawAmount = profitAvailable.toLocaleString('id-ID'))}
							>{tr("Pakai semua")}</button>
						</span>
					</label>
					<div class="grid gap-4 sm:grid-cols-2">
						<label class="finField">
							<span class="finFieldLabel">{tr("Rekening asal (escrow)")}</span>
							<input class="app-input w-full" bind:value={withdrawBankFrom} maxlength="200" placeholder="BCA 1234567890" />
						</label>
						<label class="finField">
							<span class="finFieldLabel">{tr("Rekening tujuan (operasional)")}</span>
							<input class="app-input w-full" bind:value={withdrawBankTo} maxlength="200" placeholder="BCA 9876543210" />
						</label>
					</div>
					<label class="finField">
						<span class="finFieldLabel">{tr("Nomor referensi transfer")} <span class="finOptional">{tr("opsional")}</span></span>
						<input class="app-input w-full" bind:value={withdrawRef} maxlength="200" />
					</label>
					<label class="finField">
						<span class="finFieldLabel">{tr("Catatan")} <span class="finOptional">{tr("opsional")}</span></span>
						<textarea class="app-input w-full" rows="2" bind:value={withdrawNote} maxlength="2000"></textarea>
					</label>
				</div>
				{#if withdrawError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{withdrawError}</p>
				{/if}
				{#snippet footer()}
					<button type="button" class="app-btn app-btn-soft" disabled={withdrawBusy} onclick={closeWithdraw}>{tr("Batal")}</button>
					<button type="button" class="app-btn app-btn-primary" disabled={withdrawBusy} onclick={() => void submitWithdraw()}>
						{withdrawBusy ? tr('Menyimpan…') : tr('Simpan pencatatan')}
					</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- ============ Modal: saldo klien per brand ============ -->
		{#if depositsOpen}
			<Modal
				title={depositsScope === 'reserved' ? tr('Dana tertahan per brand') : tr('Saldo klien per brand')}
				maxWidth="workspace"
				onClose={() => (depositsOpen = false)}
			>
				<p class="finLead">
					{tr("Semua brand yang masih punya saldo, dari yang terbesar. Titipan nyata ditopang topup asli lewat gateway; penyesuaian adalah sisanya (manual atau bonus); reserved terkunci untuk order berjalan.")}
				</p>
				{#if depositsLoading && depositsEntries === null}
					<p class="mt-5 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if depositsError}
					<p class="mt-5 text-sm" style="color: var(--destructive);">{depositsError}</p>
				{:else if depositsEntries && depositsEntries.length === 0}
					<div class="finEmpty mt-5">{tr("Tidak ada brand dengan saldo.")}</div>
				{:else if depositsEntries}
					{@const sumReal = depositsEntries.reduce((a, r) => a + (r.real_tc ?? 0), 0)}
					{@const sumAdj = depositsEntries.reduce((a, r) => a + (r.adjusted_tc ?? r.available_tc), 0)}
					{@const sumRes = depositsEntries.reduce((a, r) => a + r.reserved_tc, 0)}
					<div class="finStats mt-5">
						<div class="finStat">
							<span class="finStatLabel">{tr("Brand bersaldo")}</span>
							<span class="finStatValue">{fmtNum(depositsEntries.length)}</span>
						</div>
						<div class="finStat finStatKey">
							<span class="finStatLabel">{tr("Titipan nyata")}</span>
							<span class="finStatValue">{rp(sumReal)}</span>
						</div>
						<div class="finStat">
							<span class="finStatLabel">{tr("Penyesuaian")}</span>
							<span class="finStatValue">{rp(sumAdj)}</span>
						</div>
						<div class="finStat">
							<span class="finStatLabel">{tr("Reserved")}</span>
							<span class="finStatValue">{rp(sumRes)}</span>
						</div>
					</div>
					<div class="finTableWrap mt-4">
						<table class="finTable" style="min-width: 980px;">
							<thead>
								<tr class="text-left">
									<th>{tr("Brand")}</th>
									<th>{tr("Klien / pemilik")}</th>
									<th class="num">{tr("Topup asli")}</th>
									<th class="num">{tr("Titipan nyata")}</th>
									<th class="num">{tr("Penyesuaian")}</th>
									<th class="num">{tr("Reserved")}</th>
									<th class="num">{tr("Total saldo")}</th>
									<th class="num">{tr("Terakhir aktif")}</th>
								</tr>
							</thead>
							<tbody>
								{#each depositsEntries as row (row.brand_id)}
									<tr>
										<td>
											<p class="font-medium">{row.brand_name}</p>
											<p class="finMuted">{row.ownership_type === 'client' ? tr('Klien') : tr('Internal')}</p>
										</td>
										<td>
											<p>{row.client_name || row.owner_name || '—'}</p>
											{#if row.owner_email}<p class="finMuted">{row.owner_email}</p>{/if}
										</td>
										<td class="num" style="color: var(--muted-foreground);">{rp(row.real_in ?? 0)}</td>
										<td class="num font-semibold">{rp(row.real_tc ?? 0)}</td>
										<td class="num">{rp(row.adjusted_tc ?? row.available_tc)}</td>
										<td class="num" style="color: var(--muted-foreground);">{rp(row.reserved_tc)}</td>
										<td class="num">{rp(row.total_tc)}</td>
										<td class="num finMuted">{row.last_activity_at ? fmtDateTime(row.last_activity_at) : '—'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
				{#snippet footer()}
					<button type="button" class="app-btn app-btn-soft" onclick={() => (depositsOpen = false)}>{tr("Tutup")}</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- ============ Modal: dana tertahan (escrow) per publikasi ============ -->
		{#if escrowOpen}
			<Modal title={tr("Dana tertahan per publikasi")} maxWidth="workspace" onClose={() => (escrowOpen = false)}>
				<p class="finLead">
					{tr("Publikasi yang belum terbit atau belum dibatalkan. Daftar ini menjelaskan order mana yang mengunci kredit klien.")}
				</p>
				{#if escrowLoading && escrowEntries === null}
					<p class="mt-5 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if escrowError}
					<p class="mt-5 text-sm" style="color: var(--destructive);">{escrowError}</p>
				{:else if escrowEntries && escrowEntries.length === 0}
					<div class="finEmpty mt-5">{tr("Tidak ada order escrow aktif saat ini.")}</div>
				{:else if escrowEntries}
					<div class="finStats mt-5">
						<div class="finStat">
							<span class="finStatLabel">{tr("Order tertahan")}</span>
							<span class="finStatValue">{fmtNum(escrowEntries.length)}</span>
						</div>
						<div class="finStat finStatKey">
							<span class="finStatLabel">{tr("Nilai harga list")}</span>
							<span class="finStatValue">{rp(escrowTotalPrice)}</span>
						</div>
					</div>
					<div class="finTableWrap mt-4">
						<table class="finTable" style="min-width: 820px;">
							<thead>
								<tr class="text-left">
									<th>{tr("Dibuat")}</th>
									<th>{tr("Brand")}</th>
									<th>{tr("Media / mitra")}</th>
									<th>{tr("Status")}</th>
									<th class="num">{tr("Harga list")}</th>
								</tr>
							</thead>
							<tbody>
								{#each escrowEntries as row (row.publication_id)}
									<tr>
										<td class="finMuted whitespace-nowrap">{fmtDateTime(row.created_at)}</td>
										<td>
											<p class="font-medium">{row.brand_name}</p>
											{#if row.client_name}<p class="finMuted">{row.client_name}</p>{/if}
										</td>
										<td>
											<p>{row.media_name || '—'}</p>
											{#if row.partner_name}<p class="finMuted">{row.partner_name}</p>{/if}
										</td>
										<td><Badge label={tr(pubStatusLabel(row.status))} tone={row.status === 'processing' ? 'accent' : 'neutral'} /></td>
										<td class="num font-semibold">{rp(row.client_price)}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
				{#snippet footer()}
					<button type="button" class="app-btn app-btn-soft" onclick={() => (escrowOpen = false)}>{tr("Tutup")}</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- ============ Modal: hak mitra belum cair per mitra ============ -->
		{#if unpaidOpen}
			<Modal title={tr("Hak mitra belum cair per mitra")} maxWidth="workspace" onClose={() => (unpaidOpen = false)}>
				<p class="finLead">
					{tr("Utang ke mitra, dipecah per partner. Pending belum masuk settlement; settled belum dibayar sudah masuk settlement tetapi belum ditransfer.")}
				</p>
				{#if unpaidLoading && unpaidEntries === null}
					<p class="mt-5 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if unpaidError}
					<p class="mt-5 text-sm" style="color: var(--destructive);">{unpaidError}</p>
				{:else if unpaidEntries && unpaidEntries.length === 0}
					<div class="finEmpty mt-5">{tr("Tidak ada utang ke mitra saat ini.")}</div>
				{:else if unpaidEntries}
					{@const sumPending = unpaidEntries.reduce((a, r) => a + r.amount_pending, 0)}
					{@const sumSettled = unpaidEntries.reduce((a, r) => a + r.amount_settled_unpaid, 0)}
					<div class="finStats mt-5">
						<div class="finStat">
							<span class="finStatLabel">{tr("Mitra")}</span>
							<span class="finStatValue">{fmtNum(unpaidEntries.length)}</span>
						</div>
						<div class="finStat">
							<span class="finStatLabel">{tr("Pending")}</span>
							<span class="finStatValue">{rp(sumPending)}</span>
						</div>
						<div class="finStat">
							<span class="finStatLabel">{tr("Settled belum dibayar")}</span>
							<span class="finStatValue">{rp(sumSettled)}</span>
						</div>
						<div class="finStat finStatKey">
							<span class="finStatLabel">{tr("Total")}</span>
							<span class="finStatValue">{rp(sumPending + sumSettled)}</span>
						</div>
					</div>
					<div class="finTableWrap mt-4">
						<table class="finTable" style="min-width: 820px;">
							<thead>
								<tr class="text-left">
									<th>{tr("Mitra")}</th>
									<th class="num">{tr("Pending")}</th>
									<th class="num">{tr("Settled belum dibayar")}</th>
									<th class="num">{tr("Total")}</th>
									<th class="num">{tr("Penghasilan terakhir")}</th>
								</tr>
							</thead>
							<tbody>
								{#each unpaidEntries as row (row.partner_id)}
									<tr>
										<td>
											<p class="font-medium">{row.partner_name || row.partner_id.slice(0, 8)}</p>
											{#if row.partner_email}<p class="finMuted">{row.partner_email}</p>{/if}
										</td>
										<td class="num">
											{rp(row.amount_pending)}
											<span class="finMuted block">{fmtNum(row.count_pending)} {tr("penghasilan")}</span>
										</td>
										<td class="num">
											{rp(row.amount_settled_unpaid)}
											<span class="finMuted block">{fmtNum(row.count_settled_unpaid)} {tr("penghasilan")}</span>
										</td>
										<td class="num font-semibold">{rp(row.total)}</td>
										<td class="num finMuted">{row.last_earning_at ? fmtDateTime(row.last_earning_at) : '—'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
				{#snippet footer()}
					<a class="mr-auto self-center text-xs underline underline-offset-2" style="color: var(--primary);" href="/admin/settlements">{tr("Kelola pencairan mitra")}</a>
					<button type="button" class="app-btn app-btn-soft" onclick={() => (unpaidOpen = false)}>{tr("Tutup")}</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- ============ Modal: profit bersih Tauthub per publikasi ============ -->
		{#if profitOpen}
			<Modal title={tr("Profit bersih Tauthub per publikasi")} maxWidth="workspace" onClose={() => (profitOpen = false)}>
				<p class="finLead">
					{tr("Pendapatan Tauthub dihitung dari uang yang benar-benar diterima: kredit yang dibayar klien untuk publikasi, dikurangi penghasilan mitra luar. Penghasilan mitra internal (akun milik Tauthub sendiri) tidak dikurangkan karena itu uang Tauthub.")}
				</p>
				{#if profitLoading && profitEntries === null}
					<p class="mt-5 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if profitError}
					<p class="mt-5 text-sm" style="color: var(--destructive);">{profitError}</p>
				{:else if profitEntries && profitEntries.length === 0}
					<div class="finEmpty mt-5">{tr("Belum ada publikasi terbit.")}</div>
				{:else if profitEntries}
					{#if profitTotals}
						{@const pt = profitTotals}
						<dl class="finStatement mt-5">
							<div><dt>{tr("Dari order mitra luar (kredit dibayar − bagian mitra)")}</dt><dd>{rp(pt.commission)}</dd></div>
							<div><dt>{tr("Dari order mitra internal")}</dt><dd>{rp(pt.internal)}</dd></div>
							<div><dt>{tr("Pemakaian kredit non-publikasi (AI, potongan manual)")}</dt><dd>{rp(pt.non_publication)}</dd></div>
							<div><dt>{tr("Fee gateway")}</dt><dd>− {rp(pt.gateway_fee)}</dd></div>
							<div class="sub"><dt>{tr("Pendapatan kotor")}</dt><dd>{rp(pt.gross)}</dd></div>
							<div><dt>{tr("Sudah ditarik atau dicairkan")}</dt><dd>− {rp(pt.withdrawn)}</dd></div>
							<div class="total"><dt>{tr("Profit bersih tersedia")}</dt><dd style="color: var(--success);">{rp(pt.available)}</dd></div>
						</dl>
					{/if}

					{#if profitMonthly.length > 0}
						<h3 class="finSection">{tr("Per bulan")}</h3>
						<div class="finTableWrap mt-2">
							<table class="finTable" style="min-width: 560px;">
								<thead>
									<tr class="text-left">
										<th>{tr("Bulan")}</th>
										<th class="num">{tr("Harga list")}</th>
										<th class="num">{tr("Dibayar klien")}</th>
										<th class="num">{tr("Pendapatan Tauthub")}</th>
										<th class="num">{tr("Terbit")}</th>
									</tr>
								</thead>
								<tbody>
									{#each profitMonthly as m (m.month)}
										<tr>
											<td>{m.month}</td>
											<td class="num" style="color: var(--muted-foreground);">{rp(m.gmv)}</td>
											<td class="num">{rp(m.credit)}</td>
											<td class="num font-semibold">{rp(m.revenue)}</td>
											<td class="num">{fmtNum(m.count)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}

					<h3 class="finSection">
						{tr("100 publikasi terbaru")}
						<span class="finMuted">· {fmtNum(profitTotalCount)} {tr("publikasi terbit")}</span>
					</h3>
					<div class="finTableWrap mt-2">
						<table class="finTable" style="min-width: 1020px;">
							<thead>
								<tr class="text-left">
									<th>{tr("Terbit")}</th>
									<th>{tr("Brand")}</th>
									<th>{tr("Media / mitra")}</th>
									<th class="num">{tr("Harga list")}</th>
									<th class="num">{tr("Dibayar klien")}</th>
									<th class="num">{tr("Bagian mitra")}</th>
									<th class="num">{tr("Pendapatan Tauthub")}</th>
									<th></th>
								</tr>
							</thead>
							<tbody>
								{#each profitEntries as row (row.publication_id)}
									<tr>
										<td class="finMuted whitespace-nowrap">{row.published_at ? fmtDateTime(row.published_at) : '—'}</td>
										<td>
											<p class="font-medium">{row.brand_name}</p>
											{#if row.client_name}<p class="finMuted">{row.client_name}</p>{/if}
										</td>
										<td>
											<p>{row.media_name || '—'}</p>
											{#if row.partner_name}<p class="finMuted">{row.partner_name}</p>{/if}
										</td>
										<td class="num" style="color: var(--muted-foreground);">{rp(row.client_price)}</td>
										<td class="num">{rp(row.credit)}</td>
										<td class="num">
											{#if row.partner_outside > 0}
												{rp(row.partner_outside)}
											{:else if row.internal_share > 0}
												<span class="finMuted">{tr("mitra internal")}</span>
											{:else}
												<span class="finMuted">—</span>
											{/if}
										</td>
										<td class="num font-semibold" style="color: {row.revenue < 0 ? 'var(--destructive)' : 'var(--foreground)'};">
											{row.revenue < 0 ? '− ' : ''}{rp(Math.abs(row.revenue))}
											{#if row.internal_share > 0}
												<span class="finMuted block font-normal">{tr("termasuk porsi mitra")} {rp(row.internal_share)}</span>
											{/if}
										</td>
										<td class="num">
											{#if row.published_url}
												<a class="inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={row.published_url} target="_blank" rel="noopener noreferrer">
													<ExternalLink size={12} /> {tr("Buka")}
												</a>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
				{#snippet footer()}
					<button type="button" class="app-btn app-btn-soft" onclick={() => (profitOpen = false)}>{tr("Tutup")}</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- ============ Modal: rekonsiliasi saldo rekening wajib ada ============ -->
		{#if cashOpen}
			<Modal title={tr("Rekonsiliasi saldo rekening")} maxWidth="4xl" onClose={() => (cashOpen = false)}>
				<p class="finLead">
					{tr("Angka pembanding dengan saldo bank fisik. Kalau saldo rekening escrow kurang dari angka wajib ada, ada uang milik orang lain yang terpakai operasional.")}
				</p>
				<dl class="finStatement mt-4">
					<div><dt>{tr("Dana titipan klien (uang nyata)")}</dt><dd>{rp(summary.client_available ?? 0)}</dd></div>
					<div><dt>{tr("Hak mitra belum cair")}</dt><dd>{rp(summary.partner_unpaid ?? 0)}</dd></div>
					<div><dt>{tr("Profit bersih Tauthub (belum ditarik, termasuk mitra internal)")}</dt><dd>{rp(summary.tauthub_available_to_withdraw ?? 0)}</dd></div>
					<div><dt>{tr("Dana tertahan (escrow)")}</dt><dd>{rp(summary.client_reserved ?? 0)}</dd></div>
					<div class="total"><dt>{tr("Total wajib ada di rekening")}</dt><dd>{rp(summary.cash_required_in_bank ?? 0)}</dd></div>
				</dl>

				{#if summary.reconciliation}
					{@const rc = summary.reconciliation}
					<h3 class="finSection">{tr("Cocokkan dengan saldo gateway")}</h3>
					<p class="finLead mt-1">
						{tr("Kumulatif sampai tanggal akhir rentang, tidak bergantung pada filter periode. Pencairan mitra yang dibayar lewat gateway belum dikurangkan, jadi cocokkan dengan saldo di halaman gateway.")}
					</p>
					<div class="finStats mt-3">
						<div class="finStat">
							<span class="finStatLabel">{tr("Saldo gateway (perkiraan)")}</span>
							<span class="finStatValue">{rp(rc.gateway_balance_est)}</span>
							<span class="finMuted">{tr("Net topup")} {rp(rc.gateway_in_net)}{#if rc.admin_withdrawn > 0} − {tr("penarikan")} {rp(rc.admin_withdrawn)}{/if}</span>
						</div>
						<div class="finStat">
							<span class="finStatLabel">{tr("Wajib ada, kumulatif")}</span>
							<span class="finStatValue">{rp(rc.required)}</span>
						</div>
						<div class="finStat finStatKey">
							<span class="finStatLabel">{tr("Selisih")}</span>
							<span class="finStatValue">{rc.gap < 0 ? '− ' : ''}{rp(Math.abs(rc.gap))}</span>
						</div>
					</div>

					<h3 class="finSection">{tr("Isi selisih")}</h3>
					<div class="finTableWrap mt-2">
						<table class="finTable" style="min-width: 520px;">
							<tbody>
								{#each rc.components as c (c.key)}
									<tr>
										<td>
											<p class="font-medium">{tr(c.label)}</p>
											<p class="finMuted">{tr(c.note)}</p>
										</td>
										<td class="num font-semibold" style="color: {c.amount < 0 ? 'var(--destructive)' : 'var(--foreground)'};">
											{c.amount < 0 ? '− ' : ''}{rp(Math.abs(c.amount))}
										</td>
									</tr>
								{/each}
								<tr class="finTotalRow">
									<td class="font-semibold">{tr("Total")}</td>
									<td class="num font-semibold">{rc.gap < 0 ? '− ' : ''}{rp(Math.abs(rc.gap))}</td>
								</tr>
							</tbody>
						</table>
					</div>

					{#if rc.orphan_publications.length > 0}
						<h3 class="finSection">
							{tr("Publikasi terbit tanpa catatan penghasilan mitra")}
							<span class="finMuted">· {fmtNum(rc.orphan_count)}</span>
						</h3>
						<p class="finLead mt-1">
							{tr("Kredit dari publikasi ini dihitung seluruhnya sebagai pendapatan Tauthub sampai penghasilan mitranya dicatat.")}
						</p>
						<div class="finTableWrap mt-2">
							<table class="finTable" style="min-width: 560px;">
								<thead>
									<tr class="text-left">
										<th>{tr("Media")}</th>
										<th>{tr("Mitra")}</th>
										<th class="num">{tr("Harga")}</th>
										<th class="num">{tr("Kredit terpakai")}</th>
									</tr>
								</thead>
								<tbody>
									{#each rc.orphan_publications as o, i (i)}
										<tr>
											<td>{o.media || '—'}</td>
											<td>{o.partner || '—'}</td>
											<td class="num" style="color: var(--muted-foreground);">{rp(o.client_price)}</td>
											<td class="num font-semibold">{rp(o.credit)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				{/if}
				{#snippet footer()}
					<button type="button" class="app-btn app-btn-soft" onclick={() => (cashOpen = false)}>{tr("Tutup")}</button>
				{/snippet}
			</Modal>
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
	/* Warna seri data (komposisi neraca + grafik). Empat hue kategorikal yang
	   sudah divalidasi untuk permukaan terang dan gelap; teks tetap memakai
	   token teks, warna hanya di tanda (batang, swatch). */
	.fin {
		--s1: #2a78d6;
		--s2: #eb6834;
		--s3: #1baf7a;
		--s4: #eda100;
	}
	:global(.dark) .fin {
		--s1: #3987e5;
		--s2: #d95926;
		--s3: #199e70;
		--s4: #c98500;
	}

	.finHero {
		font-size: clamp(30px, 4vw, 42px);
		font-weight: 700;
		line-height: 1.05;
		letter-spacing: -0.025em;
		font-variant-numeric: tabular-nums;
		color: var(--foreground);
	}
	.finBar {
		display: flex;
		gap: 2px;
		height: 12px;
	}
	.finSeg {
		min-width: 4px;
		border-radius: 2px;
	}
	.finSeg:first-child {
		border-top-left-radius: 4px;
		border-bottom-left-radius: 4px;
	}
	.finSeg:last-child {
		border-top-right-radius: 4px;
		border-bottom-right-radius: 4px;
	}
	.finRow {
		appearance: none;
		display: grid;
		grid-template-columns: 10px minmax(0, 1fr) auto auto 16px;
		align-items: center;
		gap: 12px;
		width: 100%;
		min-height: 52px;
		padding: 8px;
		border: 0;
		border-top: 1px solid var(--border);
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}
	.finRow:hover {
		background: color-mix(in srgb, var(--primary) 5%, transparent);
	}
	.finRow:active {
		background: color-mix(in srgb, var(--primary) 9%, transparent);
	}
	.finRow:focus-visible,
	.finNote:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: -2px;
		border-radius: 6px;
	}
	.finSwatch {
		width: 10px;
		height: 10px;
		border-radius: 3px;
	}
	.finAmount {
		min-width: 104px;
		font-size: 14px;
		font-weight: 600;
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.finRow :global(.finChev) {
		color: var(--muted-foreground);
		transition: transform 0.15s ease, color 0.15s ease;
	}
	.finRow:hover :global(.finChev) {
		color: var(--primary);
		transform: translateX(2px);
	}
	.finNote {
		appearance: none;
		padding: 0;
		border: 0;
		background: transparent;
		font: inherit;
		font-size: 12px;
		color: var(--muted-foreground);
		text-align: left;
		text-decoration: underline;
		text-underline-offset: 3px;
		cursor: pointer;
	}
	.finNote:hover {
		color: var(--primary);
	}
	.finSide {
		border-top: 1px solid var(--border);
		background: color-mix(in srgb, var(--secondary) 45%, var(--card));
	}
	@media (min-width: 1280px) {
		.finSide {
			border-top: 0;
			border-left: 1px solid var(--border);
		}
	}
	@media (max-width: 520px) {
		.finRow {
			grid-template-columns: 10px minmax(0, 1fr) auto 16px;
		}
		.finRow > span:nth-child(3) {
			display: none;
		}
		.finAmount {
			min-width: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.finRow,
		.finRow :global(.finChev) {
			transition: none;
		}
		.finRow:hover :global(.finChev) {
			transform: none;
		}
	}

	/* ===== Kerangka bersama kartu, statistik, tabel, dan formulir ===== */
	.finCardHead {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px 20px;
		padding: 20px 24px 16px;
	}
	.finCardBody {
		padding: 0 24px 24px;
	}
	.finTitle {
		font-size: 15px;
		font-weight: 600;
		letter-spacing: -0.01em;
		color: var(--foreground);
	}
	.finSub {
		margin-top: 4px;
		max-width: 70ch;
		font-size: 12.5px;
		line-height: 1.55;
		color: var(--muted-foreground);
	}
	.finLead {
		max-width: 72ch;
		font-size: 13px;
		line-height: 1.6;
		color: var(--muted-foreground);
	}
	.finSection {
		margin-top: 28px;
		font-size: 13px;
		font-weight: 600;
		color: var(--foreground);
	}
	.finMuted {
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.finOptional {
		margin-left: 4px;
		font-weight: 400;
		color: var(--muted-foreground);
	}
	.finFootnote {
		padding: 8px 24px;
		border-bottom: 1px solid var(--border);
		font-size: 11.5px;
		color: var(--muted-foreground);
	}
	.finEmpty {
		padding: 28px 16px;
		border: 1px dashed var(--border);
		border-radius: 10px;
		text-align: center;
		font-size: 13px;
		color: var(--muted-foreground);
	}

	/* Deretan angka kunci: satu garis tipis antar angka, tanpa kotak bertumpuk. */
	.finStats {
		display: flex;
		flex-wrap: wrap;
		gap: 1px;
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--border);
	}
	.finStatsBleed {
		border-width: 1px 0;
		border-radius: 0;
	}
	.finStat {
		display: flex;
		flex: 1 1 170px;
		flex-direction: column;
		gap: 4px;
		padding: 14px 18px;
		background: var(--card);
	}
	.finStatKey {
		background: color-mix(in srgb, var(--primary) 4%, var(--card));
	}
	.finStatLabel {
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.finStatValue {
		font-size: 19px;
		font-weight: 600;
		letter-spacing: -0.015em;
		font-variant-numeric: tabular-nums;
		color: var(--foreground);
	}
	.finStatValue small {
		font-size: 12px;
		font-weight: 400;
		letter-spacing: 0;
		color: var(--muted-foreground);
	}

	/* Tabel: kepala huruf kecil kapital, baris lapang, angka rata kanan. */
	.finTableWrap {
		overflow-x: auto;
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.finTable {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}
	.finTable th {
		padding: 10px 14px;
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		white-space: nowrap;
		color: var(--muted-foreground);
		background: color-mix(in srgb, var(--secondary) 55%, var(--card));
	}
	.finTable td {
		padding: 12px 14px;
		vertical-align: top;
	}
	.finTable tbody tr {
		border-top: 1px solid var(--border);
		transition: background-color 0.15s ease;
	}
	.finTable tbody tr:first-child {
		border-top: 0;
	}
	.finTable tbody tr:hover {
		background: color-mix(in srgb, var(--primary) 4%, transparent);
	}
	.finTable .num {
		text-align: right;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.finTable .finTotalRow {
		background: color-mix(in srgb, var(--secondary) 55%, var(--card));
	}

	/* Laporan bertingkat: label di kiri, angka di kanan. */
	.finStatement {
		max-width: 680px;
	}
	.finStatement > div {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 24px;
		padding: 10px 0;
		border-top: 1px solid var(--border);
		font-size: 13px;
	}
	.finStatement > div:first-child {
		border-top: 0;
	}
	.finStatement dt {
		color: var(--muted-foreground);
	}
	.finStatement dd {
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		color: var(--foreground);
	}
	.finStatement .sub dt {
		font-weight: 600;
		color: var(--foreground);
	}
	.finStatement .total {
		border-top: 1px solid var(--foreground);
	}
	.finStatement .total dt {
		font-weight: 600;
		color: var(--foreground);
	}
	.finStatement .total dd {
		font-size: 17px;
	}

	/* Formulir di modal. */
	.finField {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.finFieldLabel {
		font-size: 12.5px;
		font-weight: 600;
		color: var(--foreground);
	}
	.finInputRow {
		display: flex;
		gap: 8px;
	}
	@media (max-width: 640px) {
		.finCardHead {
			padding: 16px 16px 12px;
		}
		.finCardBody {
			padding: 0 16px 16px;
		}
		.finFootnote {
			padding: 8px 16px;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.finTable tbody tr {
			transition: none;
		}
	}
</style>
