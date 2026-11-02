<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Laporan keuangan: uang masuk dari topup klien yang lunas vs uang keluar
	// untuk pencairan mitra yang sudah ditransfer. Fokusnya arus kas — bukan
	// laba — jadi kompensasi saldo dan invoice manual ditampilkan terpisah.
	import { onMount } from 'svelte';
	import {
		ArrowLeftRight, Download, RotateCcw, RotateCw, TriangleAlert
	} from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
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
		invoice_paid: number; invoice_paid_count: number;
		compensation_credits: number; compensation_count: number;
	};
	type Monthly = { month: string; label: string; money_in: number; money_out: number };
	type Report = {
		from: string; to: string; summary: Summary; monthly: Monthly[];
		entries: Entry[]; total: number; limit: number; offset: number;
	};

	type Preset = 'bulan-ini' | 'bulan-lalu' | '7-hari' | '30-hari' | 'tahun-ini' | 'kustom';

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
		{ value: 'tahun-ini', label: 'Tahun ini' }
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
</script>

<section>
	<PageHeader title={tr("Keuangan Tauthub")}>
		<button class="app-btn app-btn-soft headerBtn" type="button" disabled={refreshing} onclick={() => void load(pageNum, true)}>
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
		<div class="app-card mt-5 flex flex-col gap-3 px-4 py-3.5 xl:flex-row xl:items-end xl:justify-between">
			<div class="flex min-w-0 flex-wrap items-center gap-2 xl:pb-0.5">
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

		<!-- Tiga angka utama -->
		<div class="mt-5 grid gap-4 md:grid-cols-3">
			<article class="kpiCard kpiIn">
				<div class="flex items-start justify-between gap-2">
					<span class="text-[13px] font-semibold text-muted-foreground">{tr("Uang Masuk")}</span>
					<span class="kpiBadge kpiBadgeIn">{fmtNum(summary.money_in_count)} {tr("topup")}</span>
				</div>
				<p class="kpiValue mt-2.5">{rp(summary.money_in)}</p>
				<p class="mt-auto pt-2.5 text-[11.5px] leading-relaxed text-muted-foreground">{tr("Topup klien yang sudah lunas pada periode ini.")}</p>
			</article>

			<article class="kpiCard kpiOut">
				<div class="flex items-start justify-between gap-2">
					<span class="text-[13px] font-semibold text-muted-foreground">{tr("Uang Keluar")}</span>
					<span class="kpiBadge kpiBadgeOut">{fmtNum(summary.money_out_count)} {tr("pencairan")}</span>
				</div>
				<p class="kpiValue mt-2.5">{rp(summary.money_out)}</p>
				<p class="mt-auto pt-2.5 text-[11.5px] leading-relaxed text-muted-foreground">
					{tr("Pencairan mitra yang sudah ditransfer (net setelah biaya).")}
					{#if summary.money_out_fee > 0}
						<span class="block">{tr("Bruto")} {rp(summary.money_out_gross)} · {tr("biaya transfer")} {rp(summary.money_out_fee)}</span>
					{/if}
				</p>
			</article>

			<article class="kpiCard kpiNet" class:kpiNetNegative={netNegative}>
				<div class="flex items-start justify-between gap-2">
					<Tooltip content={tr("Uang masuk dikurangi uang keluar pada periode ini. Ini arus kas, bukan laba.")}>
						<span class="text-[13px] font-semibold text-muted-foreground">{tr("Arus Kas Bersih")}</span>
					</Tooltip>
					<span class="kpiBadge kpiBadgeNet" class:kpiBadgeNetNeg={netNegative}>{netNegative ? tr('Defisit') : tr('Surplus')}</span>
				</div>
				<p class="kpiValue kpiValueNet mt-2.5" class:kpiValueNegative={netNegative}>
					{netNegative ? `− ${rp(Math.abs(summary.net))}` : rp(summary.net)}
				</p>
				<p class="mt-auto pt-2.5 text-[11.5px] leading-relaxed text-muted-foreground">{tr("Masuk − keluar pada periode ini.")}</p>
			</article>
		</div>

		<!-- Konteks posisi berjalan -->
		<div class="mt-6 flex flex-wrap items-center gap-2.5">
			<h2 class="text-base font-semibold">{tr("Posisi Saat Ini")}</h2>
			<span class="h-4 w-px" style="background-color: var(--border);"></span>
			<span class="text-xs text-muted-foreground">{tr("Tidak mengikuti filter periode.")}</span>
		</div>
		<div class="app-card positionGrid mt-3">
			<div class="positionCell">
				<Tooltip content={tr("Topup yang sudah dibuat klien tetapi belum lunas.")}>
					<span class="text-xs text-muted-foreground">{tr("Menunggu topup")}</span>
				</Tooltip>
				<p class="mt-2 text-xl font-semibold tabular-nums">{rp(summary.topup_pending)}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{fmtNum(summary.topup_pending_count)} {tr("transaksi")}</p>
			</div>
			<div class="positionCell">
				<Tooltip content={tr("Pencairan mitra yang sudah diajukan tetapi belum ditransfer.")}>
					<span class="text-xs text-muted-foreground">{tr("Menunggu pencairan")}</span>
				</Tooltip>
				<p class="mt-2 text-xl font-semibold tabular-nums">{rp(summary.settlement_pending)}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{fmtNum(summary.settlement_pending_count)} {tr("pengajuan")}</p>
			</div>
			<div class="positionCell">
				<Tooltip content={tr("Saldo klien yang masih bisa dipakai. Belum menjadi pendapatan, jadi dicatat sebagai kewajiban.")}>
					<span class="text-xs text-muted-foreground">{tr("Saldo klien")}</span>
				</Tooltip>
				<p class="mt-2 text-xl font-semibold tabular-nums">{rp(summary.client_balance)}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{tr("Belum terpakai")}</p>
			</div>
			<div class="positionCell">
				<Tooltip content={tr("Pendapatan mitra yang sudah tercatat tetapi belum masuk pencairan yang lunas.")}>
					<span class="text-xs text-muted-foreground">{tr("Saldo mitra")}</span>
				</Tooltip>
				<p class="mt-2 text-xl font-semibold tabular-nums">{rp(summary.partner_unpaid)}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{tr("Belum dicairkan")}</p>
			</div>
		</div>

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
								<td class="px-4 py-2.5 font-mono text-xs text-muted-foreground">{entry.reference || '—'}</td>
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

	/* KPI utama: angka sebagai fokus, tanpa ikon. Aksen warna hanya pada badge
	   dan garis bawah tipis supaya arah arus terbaca tanpa ramai. */
	.kpiCard {
		display: flex;
		flex-direction: column;
		min-height: 132px;
		padding: 18px;
		border: 1px solid color-mix(in srgb, var(--border) 85%, transparent);
		border-radius: 12px;
		background: var(--card);
	}
	.kpiIn {
		border-bottom: 2px solid color-mix(in srgb, #118c4f 42%, white);
	}
	.kpiOut {
		border-bottom: 2px solid color-mix(in srgb, var(--destructive) 34%, white);
	}
	.kpiNet {
		background: color-mix(in srgb, #118c4f 4%, var(--card));
		border-color: color-mix(in srgb, #118c4f 22%, white);
	}
	.kpiNetNegative {
		background: color-mix(in srgb, var(--destructive) 4%, var(--card));
		border-color: color-mix(in srgb, var(--destructive) 22%, white);
	}
	.kpiValue {
		font-size: 28px;
		font-weight: 700;
		line-height: 1.05;
		letter-spacing: -0.02em;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.kpiValueNet {
		color: #118c4f;
	}
	.kpiValueNegative {
		color: var(--destructive);
	}
	.kpiBadge {
		flex: 0 0 auto;
		padding: 2px 8px;
		border-radius: 999px;
		font-size: 11px;
		font-weight: 600;
		line-height: 1.5;
		white-space: nowrap;
	}
	.kpiBadgeIn {
		background: color-mix(in srgb, #118c4f 10%, white);
		color: #118c4f;
	}
	.kpiBadgeOut {
		background: color-mix(in srgb, var(--destructive) 9%, white);
		color: var(--destructive);
	}
	.kpiBadgeNet {
		background: color-mix(in srgb, #118c4f 10%, white);
		color: #118c4f;
	}
	.kpiBadgeNetNeg {
		background: color-mix(in srgb, var(--destructive) 9%, white);
		color: var(--destructive);
	}

	/* Posisi Saat Ini: satu panel, empat kolom setara, dipisah garis tipis —
	   bukan empat card terpisah. */
	.positionGrid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		overflow: hidden;
		border-radius: 12px;
		box-shadow: none;
	}
	.positionCell {
		padding: 18px 20px;
	}
	.positionCell + .positionCell {
		border-left: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
	}
	@media (max-width: 1023px) {
		.positionGrid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.positionCell:nth-child(3) {
			border-left: 0;
		}
		.positionCell:nth-child(3),
		.positionCell:nth-child(4) {
			border-top: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
		}
	}
	@media (max-width: 639px) {
		.positionGrid {
			grid-template-columns: 1fr;
		}
		.positionCell {
			border-left: 0 !important;
			padding: 14px 16px;
		}
		.positionCell + .positionCell {
			border-top: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
		}
	}
</style>
