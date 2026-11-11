<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Dashboard keuangan admin — fokus balance-sheet (posisi uang milik siapa)
	// + kemampuan drill-down ke detail per KPI. Pandangan arus kas periode
	// sengaja dihilangkan (menyesatkan: mencampur titipan klien dgn omzet
	// Tauthub); tetap ada chart Top Up vs GMV vs Pencairan untuk tren.
	import { onMount } from 'svelte';
	import {
		ArrowLeftRight, Download, ExternalLink, Handshake, Landmark, Lock,
		PiggyBank, RotateCcw, RotateCw, TriangleAlert, Users
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

		<!-- Posisi Keuangan: 5 KPI balance-sheet, MENGIKUTI filter periode
		     di atas. Dana Titipan + Escrow + Hak Mitra = posisi pada tanggal
		     akhir rentang. Profit = akumulasi dalam rentang.
		     Akun internal farihcoid@ dan wantek@ dikecualikan dari semua
		     perhitungan di sisi backend. -->
		{#if summary.client_available !== undefined}
			<div class="mt-5 flex items-end justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">{tr("Posisi Keuangan — per Periode")}</h2>
					<p class="text-xs text-muted-foreground">
						{tr("Posisi pada tanggal akhir rentang; profit dihitung akumulasi dalam rentang. Akun internal dikecualikan.")}
					</p>
				</div>
				<span class="text-[11px] text-muted-foreground tabular-nums">{from} → {to}</span>
			</div>
			<div class="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
				<!-- 1. Dana Titipan Klien -->
				<article class="kpiCard kpiPosLiab">
					<div class="flex items-center justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2.5">
							<span class="kpiIcon kpiIconPosLiab"><Users size={15} /></span>
							<Tooltip content={tr("Saldo klien yang ditopang topup asli lewat gateway. Secara hukum ini milik klien — dilarang dipakai operasional.")}>
								<span class="kpiLabel">{tr("Dana Titipan Klien")}</span>
							</Tooltip>
						</div>
						<span class="kpiBadge kpiBadgePosLiab">{tr("Liabilitas")}</span>
					</div>
					<p class="kpiValue kpiValuePosLiab mt-3">{rp(summary.client_available ?? 0)}</p>
					<p class="kpiDesc mt-2">{tr("Bagian saldo klien yang berasal dari topup asli.")}</p>
					<button type="button" class="kpiDetailLink mt-2" onclick={() => openDeposits('available')}>
						{tr("Lihat milik siapa saja")} →
					</button>
				</article>

				<!-- 1b. Saldo Penyesuaian: kredit dari penyesuaian manual / bonus /
				     kompensasi. Bukan uang yang masuk, jadi tidak ikut wajib-ada. -->
				<article class="kpiCard kpiPosAdj">
					<div class="flex items-center justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2.5">
							<span class="kpiIcon kpiIconPosAdj"><Users size={15} /></span>
							<Tooltip content={tr("Saldo klien yang tidak ditopang topup asli: penyesuaian manual admin, bonus, atau kompensasi. Per klien, saldo penyesuaian dianggap terpakai lebih dulu.")}>
								<span class="kpiLabel">{tr("Saldo Penyesuaian")}</span>
							</Tooltip>
						</div>
						<span class="kpiBadge kpiBadgePosAdj">{tr("Bukan uang nyata")}</span>
					</div>
					<p class="kpiValue kpiValuePosAdj mt-3">{rp(summary.client_adjusted ?? 0)}</p>
					<p class="kpiDesc mt-2">{tr("Tidak dihitung di Saldo Rek. Wajib Ada.")}</p>
					<button type="button" class="kpiDetailLink mt-2" onclick={() => openDeposits('available')}>
						{tr("Lihat milik siapa saja")} →
					</button>
				</article>

				<!-- 2. Hak Mitra Belum Cair -->
				<article class="kpiCard kpiPosDebt">
					<div class="flex items-center justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2.5">
							<span class="kpiIcon kpiIconPosDebt"><Handshake size={15} /></span>
							<Tooltip content={tr("Komisi mitra media yang sudah jadi hak mereka tetapi belum ditransfer. Siap mereka tarik kapan saja.")}>
								<span class="kpiLabel">{tr("Hak Mitra Belum Cair")}</span>
							</Tooltip>
						</div>
						<span class="kpiBadge kpiBadgePosDebt">{tr("Utang")}</span>
					</div>
					<p class="kpiValue kpiValuePosDebt mt-3">{rp(summary.partner_unpaid ?? 0)}</p>
					<p class="kpiDesc mt-2">{tr("Earning mitra pending + settled yang belum dibayar.")}</p>
					<button type="button" class="kpiDetailLink mt-2" onclick={openPartnerUnpaid}>
						{tr("Lihat per mitra")} →
					</button>
				</article>

				<!-- 3. Dana Tertahan (Escrow) -->
				<article class="kpiCard kpiPosEscrow">
					<div class="flex items-center justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2.5">
							<span class="kpiIcon kpiIconPosEscrow"><Lock size={15} /></span>
							<Tooltip content={tr("Saldo klien yang dicadangkan untuk order yang sedang dikerjakan mitra. Terkunci sampai publikasi terbit atau dibatalkan.")}>
								<span class="kpiLabel">{tr("Dana Tertahan (Escrow)")}</span>
							</Tooltip>
						</div>
						<span class="kpiBadge kpiBadgePosEscrow">{tr("Terkunci")}</span>
					</div>
					<p class="kpiValue kpiValuePosEscrow mt-3">{rp(summary.client_reserved ?? 0)}</p>
					<p class="kpiDesc mt-2">{tr("Reserved credit untuk order in-progress.")}</p>
					<button type="button" class="kpiDetailLink mt-2" onclick={openEscrow}>
						{tr("Lihat per order")} →
					</button>
				</article>

				<!-- 4. Profit Bersih Tauthub -->
				<article class="kpiCard kpiPosProfit">
					<div class="flex items-center justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2.5">
							<span class="kpiIcon kpiIconPosProfit"><PiggyBank size={15} /></span>
							<Tooltip content={tr("Uang yang benar-benar diterima Tauthub: kredit publikasi yang dibayar klien dikurangi bagian mitra, ditambah pemakaian kredit non-publikasi (AI, potongan manual), dikurangi fee gateway, lalu dikurangi yang sudah ditarik atau dicairkan. Hanya ini yang sah Anda tarik ke rekening operasional.")}>
								<span class="kpiLabel">{tr("Profit Bersih Tauthub")}</span>
							</Tooltip>
						</div>
						<button
							type="button"
							class="kpiBadge kpiBadgePosProfit kpiBadgeAction"
							onclick={openWithdraw}
							disabled={(summary.tauthub_available_to_withdraw ?? 0) <= 0}
						>{tr("Tarik")} →</button>
					</div>
					<p class="kpiValue kpiValuePosProfit mt-3">{rp(summary.tauthub_available_to_withdraw ?? 0)}</p>
					<p class="kpiDesc mt-2">
						{tr("Kotor")}: {rp(summary.tauthub_revenue_gross ?? 0)} · {tr("sudah ditarik")}: {rp(summary.tauthub_withdrawn ?? 0)}
						<span class="block">
							{tr("Komisi")} {rp(summary.tauthub_commission_gross ?? 0)} + {tr("mitra internal")} {rp(summary.tauthub_internal_gross ?? 0)}
							+ {tr("non-publikasi")} {rp(summary.tauthub_nonpublication ?? 0)} − {tr("fee")} {rp(summary.tauthub_gateway_fee ?? 0)}
						</span>
					</p>
					<button type="button" class="kpiDetailLink mt-2" onclick={openProfit}>
						{tr("Lihat per publikasi")} →
					</button>
				</article>

				<!-- 5. Saldo Rekening Wajib Ada -->
				<article class="kpiCard kpiPosCash">
					<div class="flex items-center justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2.5">
							<span class="kpiIcon kpiIconPosCash"><Landmark size={15} /></span>
							<Tooltip content={tr("Jumlah uang fisik yang WAJIB ada di rekening escrow. Jika saldo bank kurang dari ini, berarti ada dana titipan yang terpakai operasional.")}>
								<span class="kpiLabel">{tr("Saldo Rek. Wajib Ada")}</span>
							</Tooltip>
						</div>
						<span class="kpiBadge kpiBadgePosCash">{tr("Rek. Escrow")}</span>
					</div>
					<p class="kpiValue kpiValuePosCash mt-3">{rp(summary.cash_required_in_bank ?? 0)}</p>
					<p class="kpiDesc mt-2">{tr("Titipan nyata + Escrow + Utang Mitra + Profit belum ditarik.")}</p>
					<button type="button" class="kpiDetailLink mt-2" onclick={() => (cashOpen = true)}>
						{tr("Lihat breakdown")} →
					</button>
				</article>
			</div>

		{/if}


		<!-- Tren 12 bulan: Top Up vs GMV (omzet publikasi) vs Pencairan Mitra -->
		<div class="app-card mt-6 p-5">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-base font-semibold">{tr("Top Up vs GMV vs Pencairan — 12 Bulan Terakhir")}</h2>
				<span class="text-xs text-muted-foreground">{tr("Selalu 12 bulan terakhir. Klik satu bulan untuk memfilter periode.")}</span>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">
				{tr("GMV = total nilai publikasi terbit (omzet riil), bukan sekadar uang masuk. Beda besar GMV vs Top Up menunjukkan saldo klien yang belum terpakai.")}
			</p>
			<div class="mt-4">
				<CashflowBars rows={monthly} onSelect={selectMonth} />
			</div>
		</div>

		<!-- Saldo Klien Mengendap >= 90 hari -->
		<div class="app-card mt-5 p-5">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<div>
					<h2 class="text-base font-semibold">{tr("Saldo Klien Mengendap ≥ 90 Hari")}</h2>
					<p class="mt-1 text-xs text-muted-foreground">
						{tr("Brand dengan saldo kredit > 0 dan tidak ada aktivitas wallet sejak 90 hari terakhir. Hubungi untuk follow-up agar saldonya terpakai.")}
					</p>
				</div>
			</div>
			{#if dormantError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{dormantError}</p>
			{:else if dormant === null}
				<p class="mt-3 text-sm text-muted-foreground">{tr("Memuat…")}</p>
			{:else if dormant.length === 0}
				<div class="mt-3 rounded-lg border border-dashed p-6 text-center">
					<p class="text-sm text-muted-foreground">{tr("Tidak ada saldo klien yang mengendap ≥ 90 hari.")}</p>
				</div>
			{:else}
				<div class="mt-3 overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 760px;">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-3 py-2 font-medium">{tr("Brand")}</th>
								<th class="px-3 py-2 font-medium">{tr("Klien / Pemilik")}</th>
								<th class="px-3 py-2 text-right font-medium">{tr("Available")}</th>
								<th class="px-3 py-2 text-right font-medium">{tr("Reserved")}</th>
								<th class="px-3 py-2 text-right font-medium">{tr("Total")}</th>
								<th class="px-3 py-2 text-right font-medium">{tr("Terakhir Aktif")}</th>
								<th class="px-3 py-2 font-medium">{tr("Aksi")}</th>
							</tr>
						</thead>
						<tbody>
							{#each dormant as row (row.brand_id)}
								<tr class="border-t">
									<td class="px-3 py-2">
										<p class="font-medium">{row.brand_name}</p>
										<p class="text-[11px] text-muted-foreground">{row.ownership_type === 'client' ? tr('Klien') : tr('Internal')}</p>
									</td>
									<td class="px-3 py-2">
										<p>{row.client_name || row.owner_name || '—'}</p>
										{#if row.owner_email}
											<p class="text-[11px] text-muted-foreground">{row.owner_email}</p>
										{/if}
									</td>
									<td class="px-3 py-2 text-right tabular-nums">{rp(row.available_tc)}</td>
									<td class="px-3 py-2 text-right tabular-nums text-muted-foreground">{rp(row.reserved_tc)}</td>
									<td class="px-3 py-2 text-right tabular-nums font-semibold">{rp(row.total_tc)}</td>
									<td class="px-3 py-2 text-right text-xs text-muted-foreground">
										{#if row.last_activity_at}
											{fmtDateTime(row.last_activity_at)}
											<span class="block text-[10px]">({row.days_dormant} {tr("hari")})</span>
										{:else}
											{tr("Belum pernah")}
										{/if}
									</td>
									<td class="px-3 py-2">
										{#if row.owner_email}
											<a
												class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
												href={`mailto:${row.owner_email}?subject=${encodeURIComponent(tr('Follow-up saldo kredit Tauthub'))}`}
											>{tr("Follow up")}</a>
										{:else}
											<span class="text-[11px] text-muted-foreground">—</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
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

			<!-- Ringkasan top up nyata pada periode: untuk dicocokkan dengan
			     halaman Payments di gateway (Total Amount / Total Fees / Net). -->
			<div class="flex flex-wrap items-center gap-x-5 gap-y-1 border-b px-4 py-2.5 text-xs text-muted-foreground">
				<span>{tr("Top up diterima")}: <strong class="tabular-nums text-foreground">{fmtNum(summary.money_in_count)}</strong> {tr("transaksi")}</span>
				<span>{tr("Bruto")}: <strong class="tabular-nums text-foreground">{rp(summary.money_in)}</strong></span>
				<span>{tr("Fee gateway")}: <strong class="tabular-nums text-foreground">{rp(summary.money_in_fee ?? 0)}</strong></span>
				<span>{tr("Net masuk wallet gateway")}: <strong class="tabular-nums" style="color: var(--success);">{rp(summary.money_in_net ?? summary.money_in)}</strong></span>
				<span>{tr("Sandbox dan akun uji tidak dihitung.")}</span>
			</div>

			<div class="overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 1100px;">
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

		<!-- Tarik Profit Admin: pencatatan pemindahan uang dari rek. escrow ke
		     rek. operasional. Aplikasi tidak mengirim uang; hanya mencatat. -->
		<div class="app-card mt-5 p-5">
			<div class="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">{tr("Tarik Profit Admin")}</h2>
					<p class="mt-1 text-xs text-muted-foreground">
						{tr("Catat setiap kali Anda memindahkan profit dari rekening escrow ke rekening operasional. Transfer uang tetap Anda lakukan sendiri lewat m-banking.")}
					</p>
				</div>
				<button
					type="button"
					class="app-btn app-btn-primary"
					disabled={profitAvailable <= 0}
					onclick={openWithdraw}
				>{tr("Catat Penarikan")}</button>
			</div>
			<div class="mt-4 grid gap-3 sm:grid-cols-3">
				<div class="rounded-lg border px-3 py-2.5">
					<p class="text-[11px] text-muted-foreground">{tr("Profit kotor seumur hidup")}</p>
					<p class="mt-0.5 text-lg font-semibold tabular-nums">{rp(profitGross)}</p>
				</div>
				<div class="rounded-lg border px-3 py-2.5">
					<p class="text-[11px] text-muted-foreground">{tr("Sudah ditarik")}</p>
					<p class="mt-0.5 text-lg font-semibold tabular-nums">{rp(profitWithdrawn)}</p>
				</div>
				<div class="rounded-lg border px-3 py-2.5" style="background-color: color-mix(in srgb, #10b981 5%, var(--card));">
					<p class="text-[11px] text-muted-foreground">{tr("Tersedia untuk ditarik")}</p>
					<p class="mt-0.5 text-lg font-semibold tabular-nums" style="color: #059669;">{rp(profitAvailable)}</p>
				</div>
			</div>

			{#if withdrawalsError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{withdrawalsError}</p>
			{/if}

			{#if withdrawalsLoading && withdrawals.length === 0}
				<p class="mt-3 text-sm text-muted-foreground">{tr("Memuat riwayat…")}</p>
			{:else if withdrawals.length === 0}
				<div class="mt-4 rounded-lg border border-dashed p-6 text-center">
					<p class="text-sm text-muted-foreground">{tr("Belum ada penarikan profit yang tercatat.")}</p>
				</div>
			{:else}
				<div class="mt-4 overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 820px;">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-3 py-2 font-medium">{tr("Tanggal")}</th>
								<th class="px-3 py-2 text-right font-medium">{tr("Jumlah")}</th>
								<th class="px-3 py-2 font-medium">{tr("Dari → Ke")}</th>
								<th class="px-3 py-2 font-medium">{tr("Referensi")}</th>
								<th class="px-3 py-2 font-medium">{tr("Dicatat")}</th>
								<th class="px-3 py-2 font-medium">{tr("Status")}</th>
								<th class="px-3 py-2 font-medium">{tr("Aksi")}</th>
							</tr>
						</thead>
						<tbody>
							{#each withdrawals as row (row.id)}
								<tr class="border-t">
									<td class="whitespace-nowrap px-3 py-2 text-muted-foreground">{fmtDateTime(row.created_at)}</td>
									<td class="px-3 py-2 text-right tabular-nums font-semibold" style="color: {row.status === 'void' ? 'var(--muted-foreground)' : '#059669'};">
										{row.status === 'void' ? '' : '− '}{rp(row.amount)}
									</td>
									<td class="px-3 py-2 text-xs">
										{#if row.bank_from || row.bank_to}
											<p>{row.bank_from || '—'} <span class="text-muted-foreground">→</span> {row.bank_to || '—'}</p>
										{:else}
											<span class="text-muted-foreground">—</span>
										{/if}
										{#if row.note}
											<p class="text-[11px] text-muted-foreground">{row.note}</p>
										{/if}
									</td>
									<td class="px-3 py-2 font-mono text-xs text-muted-foreground">{row.reference || '—'}</td>
									<td class="px-3 py-2 text-xs">{row.created_by_name || '—'}</td>
									<td class="px-3 py-2">
										<Badge
											label={row.status === 'void' ? tr('Dibatalkan') : tr('Tercatat')}
											tone={row.status === 'void' ? 'neutral' : 'success'}
										/>
										{#if row.status === 'void' && row.void_reason}
											<p class="mt-0.5 text-[11px] text-muted-foreground">{row.void_reason}</p>
										{/if}
									</td>
									<td class="px-3 py-2">
										{#if row.status === 'paid'}
											<button
												type="button"
												class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
												onclick={() => void voidWithdrawal(row)}
											>{tr("Batalkan")}</button>
										{:else}
											<span class="text-[11px] text-muted-foreground">—</span>
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

		{#if withdrawOpen}
			<Modal title={tr("Catat Penarikan Profit")} maxWidth="lg" onClose={closeWithdraw}>
				<p class="text-sm text-muted-foreground">
					{tr("Masukkan nominal yang Anda transfer dari rekening escrow ke rekening operasional. Aplikasi hanya mencatat; uangnya ditransfer sendiri lewat m-banking.")}
				</p>
				<div class="mt-4 grid gap-3">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr("Jumlah (Rupiah)")}</span>
						<input
							class="app-input mt-1 w-full tabular-nums"
							inputmode="numeric"
							placeholder="0"
							value={withdrawAmount}
							oninput={onWithdrawAmountInput}
						/>
						<span class="mt-1 block text-[11px] text-muted-foreground">
							{tr("Maksimal")}: {rp(profitAvailable)}
						</span>
					</label>
					<div class="grid gap-3 sm:grid-cols-2">
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr("Rekening Asal (Escrow)")}</span>
							<input class="app-input mt-1 w-full" bind:value={withdrawBankFrom} maxlength="200" placeholder="BCA 1234567890" />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr("Rekening Tujuan (Operasional)")}</span>
							<input class="app-input mt-1 w-full" bind:value={withdrawBankTo} maxlength="200" placeholder="BCA 9876543210" />
						</label>
					</div>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr("Nomor Referensi Transfer (opsional)")}</span>
						<input class="app-input mt-1 w-full" bind:value={withdrawRef} maxlength="200" />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr("Catatan (opsional)")}</span>
						<textarea class="app-input mt-1 w-full" rows="2" bind:value={withdrawNote} maxlength="2000"></textarea>
					</label>
				</div>
				{#if withdrawError}
					<p class="mt-3 text-sm" style="color: var(--destructive);">{withdrawError}</p>
				{/if}
				{#snippet footer()}
					<button type="button" class="app-btn app-btn-soft" disabled={withdrawBusy} onclick={closeWithdraw}>{tr("Batal")}</button>
					<button type="button" class="app-btn app-btn-primary" disabled={withdrawBusy} onclick={() => void submitWithdraw()}>
						{withdrawBusy ? tr('Menyimpan…') : tr('Simpan pencatatan')}
					</button>
				{/snippet}
			</Modal>
		{/if}

		<!-- ============ Modal: Rincian Dana Titipan + Escrow per brand ============ -->
		{#if depositsOpen}
			<Modal
				title={depositsScope === 'reserved' ? tr('Dana Tertahan — per Brand') : tr('Dana Titipan Klien — per Brand')}
				maxWidth="workspace"
				onClose={() => (depositsOpen = false)}
			>
				<p class="text-sm text-muted-foreground">
					{tr("Saldo kredit semua brand yang masih > 0, diurutkan dari terbesar. Titipan nyata = bagian saldo yang ditopang topup asli; Penyesuaian = sisanya (manual/bonus); Reserved = terkunci untuk order berjalan.")}
				</p>
				{#if depositsLoading && depositsEntries === null}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if depositsError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{depositsError}</p>
				{:else if depositsEntries && depositsEntries.length === 0}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Tidak ada brand dengan saldo > 0.")}</p>
				{:else if depositsEntries}
					<div class="mt-4 overflow-x-auto">
						<table class="w-full text-sm" style="min-width: 820px;">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="px-2 py-2 font-medium">{tr("Brand")}</th>
									<th class="px-2 py-2 font-medium">{tr("Klien / Pemilik")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Topup asli")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Titipan nyata")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Penyesuaian")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Reserved")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Total saldo")}</th>
								</tr>
							</thead>
							<tbody>
								{#each depositsEntries as row (row.brand_id)}
									<tr class="border-t">
										<td class="px-2 py-2">
											<p class="font-medium">{row.brand_name}</p>
											<p class="text-[11px] text-muted-foreground">{row.ownership_type === 'client' ? tr('Klien') : tr('Internal')}</p>
										</td>
										<td class="px-2 py-2">
											<p>{row.client_name || row.owner_name || '—'}</p>
											{#if row.owner_email}
												<p class="text-[11px] text-muted-foreground">{row.owner_email}</p>
											{/if}
										</td>
										<td class="px-2 py-2 text-right tabular-nums text-muted-foreground">{rp(row.real_in ?? 0)}</td>
										<td class="px-2 py-2 text-right tabular-nums font-semibold" style="color: #2563eb;">{rp(row.real_tc ?? 0)}</td>
										<td class="px-2 py-2 text-right tabular-nums">{rp(row.adjusted_tc ?? row.available_tc)}</td>
										<td class="px-2 py-2 text-right tabular-nums text-muted-foreground" class:font-semibold={depositsScope === 'reserved'}>{rp(row.reserved_tc)}</td>
										<td class="px-2 py-2 text-right tabular-nums">{rp(row.total_tc)}</td>
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

		<!-- ============ Modal: Dana Tertahan (Escrow) per publikasi ============ -->
		{#if escrowOpen}
			<Modal title={tr("Dana Tertahan (Escrow) — per Publikasi")} maxWidth="workspace" onClose={() => (escrowOpen = false)}>
				<p class="text-sm text-muted-foreground">
					{tr("Publikasi yang belum terbit atau belum dibatalkan. Client price di sini menjelaskan ORDER MANA yang mengunci reserved credit klien.")}
				</p>
				{#if escrowLoading && escrowEntries === null}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if escrowError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{escrowError}</p>
				{:else if escrowEntries && escrowEntries.length === 0}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Tidak ada order escrow aktif saat ini.")}</p>
				{:else if escrowEntries}
					<p class="mt-3 text-xs text-muted-foreground">
						{tr("Total nilai order escrow")}:
						<span class="font-semibold tabular-nums">{rp(escrowTotalPrice)}</span>
						· {fmtNum(escrowEntries.length)} {tr("order")}
					</p>
					<div class="mt-3 overflow-x-auto">
						<table class="w-full text-sm" style="min-width: 760px;">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="px-2 py-2 font-medium">{tr("Dibuat")}</th>
									<th class="px-2 py-2 font-medium">{tr("Brand")}</th>
									<th class="px-2 py-2 font-medium">{tr("Media / Mitra")}</th>
									<th class="px-2 py-2 font-medium">{tr("Status")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Client Price")}</th>
								</tr>
							</thead>
							<tbody>
								{#each escrowEntries as row (row.publication_id)}
									<tr class="border-t">
										<td class="whitespace-nowrap px-2 py-2 text-xs text-muted-foreground">{fmtDateTime(row.created_at)}</td>
										<td class="px-2 py-2">
											<p class="font-medium">{row.brand_name}</p>
											{#if row.client_name}<p class="text-[11px] text-muted-foreground">{row.client_name}</p>{/if}
										</td>
										<td class="px-2 py-2">
											<p>{row.media_name || '—'}</p>
											{#if row.partner_name}<p class="text-[11px] text-muted-foreground">{row.partner_name}</p>{/if}
										</td>
										<td class="px-2 py-2">
											<Badge label={tr(pubStatusLabel(row.status))} tone={row.status === 'processing' ? 'accent' : 'neutral'} />
										</td>
										<td class="px-2 py-2 text-right tabular-nums font-semibold">{rp(row.client_price)}</td>
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

		<!-- ============ Modal: Hak Mitra Belum Cair per mitra ============ -->
		{#if unpaidOpen}
			<Modal title={tr("Hak Mitra Belum Cair — per Mitra")} maxWidth="workspace" onClose={() => (unpaidOpen = false)}>
				<p class="text-sm text-muted-foreground">
					{tr("Breakdown utang ke mitra per partner: pending (belum masuk settlement) vs settled-belum-dibayar (sudah masuk settlement tapi belum ditransfer).")}
				</p>
				{#if unpaidLoading && unpaidEntries === null}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if unpaidError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{unpaidError}</p>
				{:else if unpaidEntries && unpaidEntries.length === 0}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Tidak ada utang ke mitra saat ini.")}</p>
				{:else if unpaidEntries}
					<div class="mt-4 overflow-x-auto">
						<table class="w-full text-sm" style="min-width: 760px;">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="px-2 py-2 font-medium">{tr("Mitra")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Pending")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Settled belum dibayar")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Total")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Earning terakhir")}</th>
								</tr>
							</thead>
							<tbody>
								{#each unpaidEntries as row (row.partner_id)}
									<tr class="border-t">
										<td class="px-2 py-2">
											<p class="font-medium">{row.partner_name || row.partner_id.slice(0, 8)}</p>
											{#if row.partner_email}<p class="text-[11px] text-muted-foreground">{row.partner_email}</p>{/if}
										</td>
										<td class="px-2 py-2 text-right tabular-nums">
											{rp(row.amount_pending)}
											<p class="text-[10px] text-muted-foreground">{fmtNum(row.count_pending)} earning</p>
										</td>
										<td class="px-2 py-2 text-right tabular-nums">
											{rp(row.amount_settled_unpaid)}
											<p class="text-[10px] text-muted-foreground">{fmtNum(row.count_settled_unpaid)} earning</p>
										</td>
										<td class="px-2 py-2 text-right tabular-nums font-semibold" style="color: #ea580c;">{rp(row.total)}</td>
										<td class="px-2 py-2 text-right text-xs text-muted-foreground">
											{row.last_earning_at ? fmtDateTime(row.last_earning_at) : '—'}
										</td>
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

		<!-- ============ Modal: Profit Bersih Tauthub — breakdown per publikasi ============ -->
		{#if profitOpen}
			<Modal title={tr("Profit Bersih Tauthub — per Publikasi")} maxWidth="workspace" onClose={() => (profitOpen = false)}>
				<p class="text-sm text-muted-foreground">
					{tr("Pendapatan Tauthub dihitung dari uang yang benar-benar diterima: kredit yang dibayar klien untuk publikasi, dikurangi penghasilan mitra luar. Penghasilan mitra internal (akun milik Tauthub sendiri) tidak dikurangkan karena itu uang Tauthub. Rincian di bawah: 100 publikasi terbaru.")}
				</p>
				{#if profitLoading && profitEntries === null}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat rincian…")}</p>
				{:else if profitError}
					<p class="mt-4 text-sm" style="color: var(--destructive);">{profitError}</p>
				{:else if profitEntries && profitEntries.length === 0}
					<p class="mt-4 text-sm text-muted-foreground">{tr("Belum ada publikasi terbit.")}</p>
				{:else if profitEntries}
					<p class="mt-3 text-xs text-muted-foreground">
						{tr("Total publikasi terbit")}: <span class="font-semibold">{fmtNum(profitTotalCount)}</span>
						</p>
					{#if profitTotals}
						{@const pt = profitTotals}
						<div class="mt-3 max-w-xl space-y-1.5 text-sm">
							<div class="flex justify-between"><span>{tr("Dari order mitra luar (kredit dibayar − bagian mitra)")}</span><span class="tabular-nums">{rp(pt.commission)}</span></div>
							<div class="flex justify-between"><span>{tr("Dari order mitra internal")}</span><span class="tabular-nums">{rp(pt.internal)}</span></div>
							<div class="flex justify-between"><span>{tr("Pemakaian kredit non-publikasi (AI, potongan manual)")}</span><span class="tabular-nums">{rp(pt.non_publication)}</span></div>
							<div class="flex justify-between"><span>{tr("Fee gateway")}</span><span class="tabular-nums">− {rp(pt.gateway_fee)}</span></div>
							<div class="flex justify-between border-t pt-1.5 font-semibold"><span>{tr("Pendapatan kotor")}</span><span class="tabular-nums">{rp(pt.gross)}</span></div>
							<div class="flex justify-between"><span>{tr("Sudah ditarik atau dicairkan")}</span><span class="tabular-nums">− {rp(pt.withdrawn)}</span></div>
							<div class="flex justify-between border-t pt-1.5 font-semibold" style="color: #059669;"><span>{tr("Profit Bersih tersedia")}</span><span class="tabular-nums">{rp(pt.available)}</span></div>
						</div>
					{/if}
					{#if profitMonthly.length > 0}
						<div class="mt-3 overflow-x-auto">
							<table class="w-full text-xs" style="min-width: 520px;">
								<thead>
									<tr class="text-left text-muted-foreground">
										<th class="px-2 py-1 font-medium">{tr("Bulan")}</th>
										<th class="px-2 py-1 text-right font-medium">{tr("Harga list")}</th>
										<th class="px-2 py-1 text-right font-medium">{tr("Dibayar klien")}</th>
										<th class="px-2 py-1 text-right font-medium">{tr("Pendapatan Tauthub")}</th>
										<th class="px-2 py-1 text-right font-medium">{tr("Jumlah terbit")}</th>
									</tr>
								</thead>
								<tbody>
									{#each profitMonthly as m (m.month)}
										<tr class="border-t">
											<td class="px-2 py-1">{m.month}</td>
											<td class="px-2 py-1 text-right tabular-nums">{rp(m.gmv)}</td>
											<td class="px-2 py-1 text-right tabular-nums">{rp(m.credit)}</td>
											<td class="px-2 py-1 text-right tabular-nums font-semibold" style="color: #059669;">{rp(m.revenue)}</td>
											<td class="px-2 py-1 text-right tabular-nums">{fmtNum(m.count)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
					<p class="mt-5 text-xs font-semibold text-muted-foreground">{tr("100 publikasi terbaru:")}</p>
					<div class="mt-2 overflow-x-auto">
						<table class="w-full text-sm" style="min-width: 980px;">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="px-2 py-2 font-medium">{tr("Terbit")}</th>
									<th class="px-2 py-2 font-medium">{tr("Brand")}</th>
									<th class="px-2 py-2 font-medium">{tr("Media / Mitra")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Harga list")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Dibayar klien")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Bagian mitra")}</th>
									<th class="px-2 py-2 text-right font-medium">{tr("Pendapatan Tauthub")}</th>
									<th class="px-2 py-2 font-medium">{tr("Tautan")}</th>
								</tr>
							</thead>
							<tbody>
								{#each profitEntries as row (row.publication_id)}
									<tr class="border-t">
										<td class="whitespace-nowrap px-2 py-2 text-xs text-muted-foreground">{row.published_at ? fmtDateTime(row.published_at) : '—'}</td>
										<td class="px-2 py-2">
											<p class="font-medium">{row.brand_name}</p>
											{#if row.client_name}<p class="text-[11px] text-muted-foreground">{row.client_name}</p>{/if}
										</td>
										<td class="px-2 py-2">
											<p>{row.media_name || '—'}</p>
											{#if row.partner_name}<p class="text-[11px] text-muted-foreground">{row.partner_name}</p>{/if}
										</td>
										<td class="px-2 py-2 text-right tabular-nums text-muted-foreground">{rp(row.client_price)}</td>
										<td class="px-2 py-2 text-right tabular-nums">{rp(row.credit)}</td>
										<td class="px-2 py-2 text-right tabular-nums">
											{#if row.partner_outside > 0}
												{rp(row.partner_outside)}
											{:else if row.internal_share > 0}
												<span class="text-[11px] text-muted-foreground">{tr("mitra internal")}</span>
											{:else}
												—
											{/if}
										</td>
										<td class="px-2 py-2 text-right tabular-nums font-semibold" style="color: {row.revenue < 0 ? 'var(--destructive)' : '#059669'};">
											{row.revenue < 0 ? '− ' : ''}{rp(Math.abs(row.revenue))}
											{#if row.internal_share > 0}
												<p class="text-[10px] font-normal text-muted-foreground">{tr("termasuk porsi mitra")} {rp(row.internal_share)}</p>
											{/if}
										</td>
										<td class="px-2 py-2">
											{#if row.published_url}
												<a class="inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={row.published_url} target="_blank" rel="noopener noreferrer">
													<ExternalLink size={12} /> {tr("Buka")}
												</a>
											{:else}
												<span class="text-[11px] text-muted-foreground">—</span>
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

		<!-- ============ Modal: Saldo Rek. Wajib Ada — breakdown rumus ============ -->
		{#if cashOpen}
			<Modal title={tr("Saldo Rekening Wajib Ada — Breakdown")} maxWidth="2xl" onClose={() => (cashOpen = false)}>
				<p class="text-sm text-muted-foreground">
					{tr("Angka pembanding dengan saldo bank fisik. Jika saldo rekening escrow kurang dari ini, berarti ada uang milik orang lain yang terpakai operasional.")}
				</p>
				<div class="mt-4 space-y-2">
					<div class="flex items-center justify-between rounded-lg border px-3 py-2">
						<span class="text-sm">{tr("Dana Titipan Klien (uang nyata)")}</span>
						<span class="text-sm font-semibold tabular-nums">{rp(summary.client_available ?? 0)}</span>
					</div>
					<div class="flex items-center justify-between rounded-lg border px-3 py-2">
						<span class="text-sm">{tr("Dana Tertahan (Escrow / Reserved)")}</span>
						<span class="text-sm font-semibold tabular-nums">{rp(summary.client_reserved ?? 0)}</span>
					</div>
					<div class="flex items-center justify-between rounded-lg border px-3 py-2">
						<span class="text-sm">{tr("Hak Mitra Belum Cair")}</span>
						<span class="text-sm font-semibold tabular-nums">{rp(summary.partner_unpaid ?? 0)}</span>
					</div>
					<div class="flex items-center justify-between rounded-lg border px-3 py-2">
						<span class="text-sm">{tr("Profit Bersih Tauthub (belum ditarik, termasuk mitra internal)")}</span>
						<span class="text-sm font-semibold tabular-nums">{rp(summary.tauthub_available_to_withdraw ?? 0)}</span>
					</div>
					<div class="mt-2 flex items-center justify-between rounded-lg px-3 py-3" style="background-color: color-mix(in srgb, #8b5cf6 7%, var(--card)); border: 1px solid color-mix(in srgb, #8b5cf6 25%, var(--border));">
						<span class="text-sm font-semibold">{tr("Total yang Wajib Ada di Rekening")}</span>
						<span class="text-lg font-bold tabular-nums" style="color: #7c3aed;">{rp(summary.cash_required_in_bank ?? 0)}</span>
					</div>
				</div>

				{#if summary.reconciliation}
					{@const rc = summary.reconciliation}
					<h3 class="mt-6 text-sm font-semibold">{tr("Cocokkan dengan saldo gateway")}</h3>
					<p class="mt-1 text-xs text-muted-foreground">
						{tr("Kumulatif sampai tanggal akhir rentang, tidak bergantung pada filter periode. Pencairan mitra yang dibayar lewat gateway belum dikurangkan, jadi cocokkan angka ini dengan saldo di halaman gateway.")}
					</p>
					<div class="mt-3 space-y-2">
						<div class="flex items-center justify-between rounded-lg border px-3 py-2">
							<span class="text-sm">
								{tr("Perkiraan saldo gateway")}
								<span class="block text-[11px] text-muted-foreground">
									{tr("Net topup production")} {rp(rc.gateway_in_net)}
									({tr("bruto")} {rp(rc.gateway_in_gross)} − {tr("fee")} {rp(rc.gateway_fee)})
									{#if rc.admin_withdrawn > 0}− {tr("penarikan profit")} {rp(rc.admin_withdrawn)}{/if}
								</span>
							</span>
							<span class="text-sm font-semibold tabular-nums">{rp(rc.gateway_balance_est)}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg border px-3 py-2">
							<span class="text-sm">{tr("Wajib ada (kumulatif)")}</span>
							<span class="text-sm font-semibold tabular-nums">{rp(rc.required)}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg px-3 py-3" style="background-color: color-mix(in srgb, #10b981 7%, var(--card)); border: 1px solid color-mix(in srgb, #10b981 25%, var(--border));">
							<span class="text-sm font-semibold">{tr("Selisih: uang di gateway di luar kewajiban")}</span>
							<span class="text-lg font-bold tabular-nums" style="color: #059669;">{rp(rc.gap)}</span>
						</div>
					</div>

					<p class="mt-4 text-xs font-semibold text-muted-foreground">{tr("Isi selisih:")}</p>
					<div class="mt-2 overflow-x-auto">
						<table class="w-full text-sm" style="min-width: 520px;">
							<tbody>
								{#each rc.components as c (c.key)}
									<tr class="border-t">
										<td class="px-2 py-2">
											<p class="font-medium">{tr(c.label)}</p>
											<p class="text-[11px] text-muted-foreground">{tr(c.note)}</p>
										</td>
										<td class="px-2 py-2 text-right tabular-nums font-semibold" style="color: {c.amount < 0 ? 'var(--destructive)' : 'inherit'};">
											{c.amount < 0 ? '− ' : ''}{rp(Math.abs(c.amount))}
										</td>
									</tr>
								{/each}
								<tr class="border-t">
									<td class="px-2 py-2 font-semibold">{tr("Total")}</td>
									<td class="px-2 py-2 text-right tabular-nums font-bold">{rp(rc.gap)}</td>
								</tr>
							</tbody>
						</table>
					</div>

					{#if rc.orphan_publications.length > 0}
						<p class="mt-5 text-xs font-semibold text-muted-foreground">
							{tr("Publikasi terbit tanpa catatan penghasilan mitra")} ({fmtNum(rc.orphan_count)}):
						</p>
						<p class="mt-1 text-[11px] text-muted-foreground">
							{tr("Kredit dari publikasi ini dihitung seluruhnya sebagai pendapatan Tauthub sampai penghasilan mitranya dicatat.")}
						</p>
						<div class="mt-2 overflow-x-auto">
							<table class="w-full text-sm" style="min-width: 520px;">
								<thead>
									<tr class="text-left text-xs text-muted-foreground">
										<th class="px-2 py-1.5 font-medium">{tr("Media")}</th>
										<th class="px-2 py-1.5 font-medium">{tr("Mitra")}</th>
										<th class="px-2 py-1.5 text-right font-medium">{tr("Harga")}</th>
										<th class="px-2 py-1.5 text-right font-medium">{tr("Kredit terpakai")}</th>
									</tr>
								</thead>
								<tbody>
									{#each rc.orphan_publications as o, i (i)}
										<tr class="border-t">
											<td class="px-2 py-1.5">{o.media || '—'}</td>
											<td class="px-2 py-1.5">{o.partner || '—'}</td>
											<td class="px-2 py-1.5 text-right tabular-nums">{rp(o.client_price)}</td>
											<td class="px-2 py-1.5 text-right tabular-nums">{rp(o.credit)}</td>
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
	.kpiIcon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 28px;
		height: 28px;
		border-radius: 9px;
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
	/* ===== Posisi Keuangan (balance sheet view) =====
	   Lima warna khas supaya mudah dipindai; semua pakai color-mix dengan
	   token tema biar otomatis beradaptasi di light/dark. */
	/* 1. Dana Titipan Klien — biru info */
	.kpiPosLiab {
		background: color-mix(in srgb, #3b82f6 3%, var(--card));
		border-color: color-mix(in srgb, #3b82f6 18%, var(--border));
	}
	.kpiIconPosLiab {
		background: color-mix(in srgb, #3b82f6 12%, white);
		color: #2563eb;
	}
	.kpiValuePosLiab { color: #2563eb; }
	.kpiBadgePosLiab {
		background: color-mix(in srgb, #3b82f6 10%, white);
		color: #1d4ed8;
	}
	/* 1b. Saldo Penyesuaian — netral, sengaja tidak menonjol */
	.kpiPosAdj {
		background: color-mix(in srgb, var(--muted-foreground) 3%, var(--card));
		border-style: dashed;
	}
	.kpiIconPosAdj {
		background: var(--secondary);
		color: var(--muted-foreground);
	}
	.kpiValuePosAdj { color: var(--muted-foreground); }
	.kpiBadgePosAdj {
		background: var(--secondary);
		color: var(--muted-foreground);
	}
	/* 2. Hak Mitra Belum Cair — oranye utang */
	.kpiPosDebt {
		background: color-mix(in srgb, #f97316 3%, var(--card));
		border-color: color-mix(in srgb, #f97316 20%, var(--border));
	}
	.kpiIconPosDebt {
		background: color-mix(in srgb, #f97316 12%, white);
		color: #ea580c;
	}
	.kpiValuePosDebt { color: #ea580c; }
	.kpiBadgePosDebt {
		background: color-mix(in srgb, #f97316 11%, white);
		color: #c2410c;
	}
	/* 3. Dana Tertahan Escrow — kuning/amber */
	.kpiPosEscrow {
		background: color-mix(in srgb, #eab308 3%, var(--card));
		border-color: color-mix(in srgb, #eab308 18%, var(--border));
	}
	.kpiIconPosEscrow {
		background: color-mix(in srgb, #eab308 14%, white);
		color: #a16207;
	}
	.kpiValuePosEscrow { color: #a16207; }
	.kpiBadgePosEscrow {
		background: color-mix(in srgb, #eab308 13%, white);
		color: #854d0e;
	}
	/* 4. Profit Bersih Tauthub — hijau sah */
	.kpiPosProfit {
		background: color-mix(in srgb, #10b981 3%, var(--card));
		border-color: color-mix(in srgb, #10b981 20%, var(--border));
	}
	.kpiIconPosProfit {
		background: color-mix(in srgb, #10b981 13%, white);
		color: #059669;
	}
	.kpiValuePosProfit { color: #059669; }
	.kpiBadgePosProfit {
		background: color-mix(in srgb, #10b981 12%, white);
		color: #047857;
	}
	/* 5. Saldo Rekening Wajib Ada — ungu aksen */
	.kpiPosCash {
		background: color-mix(in srgb, #8b5cf6 4%, var(--card));
		border-color: color-mix(in srgb, #8b5cf6 22%, var(--border));
	}
	.kpiIconPosCash {
		background: color-mix(in srgb, #8b5cf6 13%, white);
		color: #7c3aed;
	}
	.kpiValuePosCash { color: #7c3aed; }
	.kpiBadgePosCash {
		background: color-mix(in srgb, #8b5cf6 12%, white);
		color: #6d28d9;
	}
	/* Link "Lihat rincian →" di tiap kartu posisi keuangan. Tombol tipis supaya
	   tidak bersaing dengan angka utama. */
	.kpiDetailLink {
		align-self: flex-start;
		padding: 2px 0;
		background: transparent;
		border: 0;
		cursor: pointer;
		font-size: 11.5px;
		font-weight: 500;
		color: var(--muted-foreground);
		text-decoration: underline;
		text-underline-offset: 3px;
		transition: color 120ms ease;
	}
	.kpiDetailLink:hover {
		color: var(--primary);
	}
	.kpiDetailLink:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
		border-radius: 3px;
	}

	/* Badge interaktif (tombol kecil di kartu Profit). */
	.kpiBadgeAction {
		border: 0;
		cursor: pointer;
		transition: background-color 120ms ease;
	}
	.kpiBadgeAction:hover:not(:disabled) {
		background: color-mix(in srgb, #10b981 20%, white);
	}
	.kpiBadgeAction:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	</style>
