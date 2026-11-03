<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Penghasilan mitra — port section "Penghasilan & Saldo": ringkasan saldo,
	// daftar earning per publikasi dengan paginasi offset (limit/offset/total
	// dari API), penarikan dana dengan metode terkunci dari profil mitra
	// (payout_type) + biaya transfer dari tc_config, dan guard rekening.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banknote, Calendar, Check, Clock, Coins, FileText, Landmark, Search, Wallet, X } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const PAGE_SIZE = 20;

	// Favicon media di baris transaksi: domain diambil dari website_url media.
	const mediaDomain = (url: string | null | undefined): string =>
		(url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').toLowerCase();
	let faviconFailed = $state<Record<string, boolean>>({});

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let data = $state<any>(null);
	let settlements = $state<any>(null);
	let partner = $state<any>(null);
	let error = $state('');
	let notice = $state('');
	// Penarikan yang baru dikirim: tampilkan arahan arsipnya di halaman invoice.
	let withdrawDone = $state(false);
	let busy = $state(false);
	let showWithdraw = $state(false);
	let mode = $state<'all' | 'partial'>('all');
	let amount = $state(0);
	let notes = $state('');
	let offset = $state(0);

	// Filter klien (UI saja): pencarian + rentang tanggal di atas baris yang dimuat.
	let search = $state('');
	let dateFilter = $state('');

	// Biaya & minimum penarikan datang dari GET /partner/settlements (tc_config),
	// bukan nilai tetap — supaya perubahan konfigurasi admin langsung terpakai.
	const withdrawFee = $derived(Number(settlements?.withdraw_bank_fee_idr ?? 2400));
	const minimumWithdraw = $derived(
		Number(settlements?.minimum_withdraw_idr ?? data?.minimum_withdraw_idr ?? 50000)
	);
	const settledBalance = $derived(Number(data?.summary?.settled ?? 0));
	const earningsTotal = $derived(Number(data?.total ?? (data?.earnings ?? []).length));
	// Metode pencairan dikunci dari profil mitra (payout_type) — parity app lama.
	const payoutType = $derived(partner?.payout_type === 'ewallet' ? 'ewallet' : 'bank');
	const hasPayoutAccount = $derived(Boolean(partner?.bank_account_number));
	const payoutAccountLabel = $derived(
		partner?.bank_account_number
			? `${partner.bank_name || tr('Bank')} · ${partner.bank_account_number} ${tr('a.n.')} ${partner.bank_account_name || tr('Pemilik Akun')}`
			: ''
	);
	const ewalletFeePct = $derived(Number(settlements?.withdraw_ewallet_fee_percent ?? 0));
	const effectiveFee = $derived(
		payoutType === 'ewallet'
			? Math.round(((mode === 'partial' ? Number(amount) || 0 : settledBalance) * ewalletFeePct) / 100)
			: withdrawFee
	);
	const netWithdraw = $derived(
		Math.max(0, (mode === 'partial' ? Number(amount) || 0 : settledBalance) - effectiveFee)
	);
	const withdrawBlocked = $derived(
		mode === 'all'
			? settledBalance < minimumWithdraw
			: !(Number(amount) > 0) || Number(amount) < minimumWithdraw || Number(amount) > settledBalance
	);

	// ── Input nominal berformat ribuan ─────────────────────────────────────
	// Nilai yang DIKIRIM tetap angka murni (`amount`); teks hanya untuk tampilan.
	// Aturan bisnis (min/max/biaya) tetap milik backend & derived di atas.
	let amountText = $state('');
	const amountDigits = (text: string) => {
		const d = text.replace(/\D/g, '');
		return d ? Number(d) : 0;
	};
	const amountDisplay = (n: number) =>
		n > 0 ? n.toLocaleString('id-ID', { maximumFractionDigits: 0 }) : '';
	// Input hanya menerima ANGKA. Karakter lain ditolak di keydown (huruf,
	// simbol, titik manual) supaya nilai tidak sempat "kotor"; titik ribuan
	// ditambahkan otomatis oleh pemformat di oninput.
	function onAmountKeydown(e: KeyboardEvent) {
		if (e.ctrlKey || e.metaKey || e.altKey) return;
		const allowed = ['Backspace', 'Delete', 'Tab', 'ArrowLeft', 'ArrowRight', 'Home', 'End'];
		if (allowed.includes(e.key)) return;
		if (!/^[0-9]$/.test(e.key)) e.preventDefault();
	}
	function onAmountInput(e: Event) {
		const el = e.currentTarget as HTMLInputElement;
		const n = amountDigits(el.value);
		amount = n;
		amountText = amountDisplay(n);
		el.value = amountText;
	}
	function selectMode(next: 'all' | 'partial') {
		mode = next;
		amount = next === 'all' ? settledBalance : 0;
		amountText = amountDisplay(amount);
	}
	function openWithdrawal() {
		error = '';
		notes = '';
		mode = 'all';
		amount = settledBalance;
		amountText = amountDisplay(settledBalance);
		showWithdraw = true;
	}
	// Pesan validasi di dekat input; tombol submit ikut memakai kondisi ini.
	const partialError = $derived.by(() => {
		if (mode !== 'partial') return '';
		if (!(Number(amount) > 0)) return tr('Masukkan nominal penarikan.');
		if (Number(amount) < minimumWithdraw) return `${tr('Minimal penarikan')} ${rp(minimumWithdraw)}.`;
		if (Number(amount) > settledBalance) return `${tr('Nominal melebihi saldo siap ditarik')} (${rp(settledBalance)}).`;
		return '';
	});
	// Nomor rekening disamarkan untuk ditampilkan (data asli tetap di payload).
	const maskedAccountNumber = $derived.by(() => {
		const acc = String(partner?.bank_account_number ?? '').trim();
		if (!acc) return '';
		return acc.length > 8 ? `${acc.slice(0, 8)}•••` : acc;
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		const [earnRes, setRes, meRes] = await Promise.all([
			apiRequest(`/v1/partner/earnings?limit=${PAGE_SIZE}&offset=${offset}`),
			apiRequest('/v1/partner/settlements?limit=20'),
			apiRequest('/v1/partner/me')
		]);
		if (earnRes.ok) {
			data = await earnRes.json();
		} else {
			error = await readApiError(earnRes, tr('Gagal memuat penghasilan.'));
			data = null;
		}
		if (setRes.ok) settlements = await setRes.json();
		if (meRes.ok) partner = (await meRes.json()).partner ?? null;
	}

	function changeOffset(next: number) {
		offset = Math.max(0, next);
		void load();
	}

	async function withdraw() {
		error = '';
		if (!hasPayoutAccount) {
			error = tr('Lengkapi dulu rekening atau e-wallet pencairan di Profil, lalu coba lagi.');
			return;
		}
		if (mode === 'partial' && Number(amount) < minimumWithdraw) {
			error = `${tr('Minimal penarikan')} ${rp(minimumWithdraw)}.`;
			return;
		}
		if (mode === 'partial' && Number(amount) > settledBalance) {
			error = `${tr('Nominal melebihi saldo siap ditarik')} (${rp(settledBalance)}).`;
			return;
		}
		if (mode === 'all' && settledBalance < minimumWithdraw) {
			error = `${tr('Minimal saldo untuk penarikan')} ${rp(minimumWithdraw)}.`;
			return;
		}
		busy = true;
		try {
			const payload: Record<string, unknown> = { mode };
			if (mode === 'partial') payload.amount_idr = Number(amount) || 0;
			if (notes.trim()) payload.notes = notes.trim();
			const res = await apiRequest('/v1/partner/settlements', { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) {
				error = await readApiError(res, tr("Permintaan gagal."));
				return;
			}
			notice = tr('Permintaan penarikan dikirim dan menunggu review admin.');
			withdrawDone = true;
			showWithdraw = false;
			await load();
		} finally {
			busy = false;
		}
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-');
	// Tanggal + jam untuk waktu rilis saldo (masa tunggu dihitung dalam jam).
	const dtTime = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '-';
	const holdDays = $derived(Math.round(Number(data?.summary?.hold_hours ?? 48) / 24));
	// $derived, bukan konstanta: tr() membaca locale yang baru tersedia setelah
	// initLocaleFromStorage() di root layout, jadi konstanta biasa akan membeku
	// di bahasa awal dan badge tidak ikut berganti ke Inggris.
	const statusLabel = $derived<Record<string, string>>({
		pending: tr('Masa tunggu'), settled: tr('Siap ditarik'), voided: tr('Dibatalkan')
	});
	// Warna badge status memakai token semantik app (bukan accent generik).
	const statusBadge = (status: string) => {
		if (status === 'settled') return 'background-color: var(--success-soft); color: var(--success);';
		if (status === 'pending') return 'background-color: var(--warning-soft); color: var(--warning);';
		if (status === 'voided') return 'background-color: rgba(197,43,54,.08); color: var(--destructive);';
		return 'background-color: var(--secondary); color: var(--muted-foreground);';
	};

	// Filter klien diterapkan pada baris halaman yang dimuat (pencarian + rentang tanggal).
	const filteredRows = $derived(
		(data?.earnings ?? []).filter((e: any) => {
			if (dateFilter) {
				const ageDays = (Date.now() - new Date(e.created_at).getTime()) / 86400000;
				if (ageDays > Number(dateFilter)) return false;
			}
			if (!search.trim()) return true;
			const term = search.trim().toLowerCase();
			return (
				(e.document_title ?? '').toLowerCase().includes(term) ||
				(e.media_name ?? '').toLowerCase().includes(term)
			);
		})
	);
</script>

<PageTitle title={tr("Penghasilan")} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="page-title">{tr("Penghasilan")}</h1>
			<p class="page-desc">{tr("Saldo mitra bertambah saat artikel terbit di media Anda.")}</p>
		</div>
		<!-- Cadangan aksi penarikan saat ringkasan saldo gagal dimuat. -->
		{#if !loading && !data}
			<button class="app-btn app-btn-primary min-h-11 shrink-0" disabled={!hasPayoutAccount} onclick={() => openWithdrawal()}>
				<Banknote size={16} />{tr("Tarik Saldo")}
			</button>
		{/if}
	</header>

	{#if notice}
		<div class="app-card mt-3 px-4 py-3" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">
			{#if withdrawDone}
				<div class="flex flex-wrap items-center gap-x-3 gap-y-2.5">
					<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full" style="background-color: color-mix(in srgb, var(--success) 16%, transparent);">
						<Check size={16} strokeWidth={2.6} />
					</span>
					<div class="min-w-0 flex-1">
						<p class="m-0 text-[13px] font-semibold">{tr('Permintaan penarikan terkirim')}</p>
						<p class="m-0 mt-0.5 text-[12.5px] leading-relaxed" style="opacity: .9;">
							{tr('Menunggu review admin. Arsip penarikan beserta invoice-nya tersedia di halaman Invoice & Penarikan.')}
						</p>
					</div>
					<a class="app-btn !min-h-9 shrink-0 !px-3.5 !text-xs" style="background-color: var(--success); color: #fff;" href={`${base}/settlements`}>
						{tr('Buka Invoice')}
					</a>
				</div>
			{:else}
				<p class="m-0 text-sm">{notice}</p>
			{/if}
		</div>
	{/if}
	{#if error}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{error}</div>
	{/if}

	{#if !loading && !hasPayoutAccount && partner}
		<div class="app-card mt-3 p-4" style="border-color: var(--warning); background-color: var(--warning-soft);">
			<p class="text-sm font-medium" style="color: var(--warning);">{tr("Data rekening atau e-wallet belum diisi.")}</p>
			<p class="mt-1 text-xs text-muted-foreground">
				{tr("Isi tujuan penerimaan dana di profil sebelum mengajukan penarikan.")}
				<a class="ml-1 inline-flex items-center gap-1 font-medium" style="color: var(--primary);" href={`${base}/profile`}>
					<Wallet size={12} /> {tr("Buka Profil")}
				</a>
			</p>
		</div>
	{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat penghasilan…")}</p>
	{:else if data}
		<!-- Focal point: saldo siap ditarik + aksi utama penarikan, layout horizontal compact -->
		<div class="app-card mt-4 p-5">
			<div class="flex flex-wrap items-center gap-x-8 gap-y-5">
				<div class="min-w-0">
					<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{tr("Saldo siap ditarik")}</p>
					<p class="mt-1 text-4xl font-bold tracking-tight tabular-nums">{rp(data.summary?.settled)}</p>
					<p class="mt-1 text-xs text-muted-foreground">{tr("Minimal penarikan")} {rp(minimumWithdraw)}</p>
					<p class="mt-0.5 text-xs text-muted-foreground">{tr("Pendapatan baru bisa ditarik")} {holdDays} {tr("hari setelah artikel terbit.")}</p>
				</div>

				<div class="hidden h-12 w-px sm:block" style="background: var(--border);"></div>

				<div class="flex items-center gap-5">
					<div class="flex items-center gap-2.5">
						<span class="flex h-10 w-10 items-center justify-center rounded-lg" style="background: var(--warning-soft);">
							<Clock size={18} style="color: var(--warning);" />
						</span>
						<div>
							<p class="text-xs text-muted-foreground">{tr("Masa tunggu")}</p>
							<p class="text-sm font-semibold tabular-nums">{rp(data.summary?.pending)}</p>
							{#if data.summary?.next_release_at}
								<p class="text-[11px] text-muted-foreground">{tr("Rilis berikutnya")} {dtTime(data.summary.next_release_at)}</p>
							{/if}
						</div>
					</div>
					<div class="flex items-center gap-2.5">
						<span class="flex h-10 w-10 items-center justify-center rounded-lg" style="background: var(--success-soft);">
							<Coins size={18} style="color: var(--success);" />
						</span>
						<div>
							<p class="text-xs text-muted-foreground">{tr("Total sudah dibayar")}</p>
							<p class="text-sm font-semibold tabular-nums">{rp(data.summary?.paid_settlements)}</p>
						</div>
					</div>
				</div>

				<div class="flex shrink-0 flex-col items-end gap-1 md:ml-auto">
					<button class="app-btn app-btn-primary min-h-11 px-5" disabled={!hasPayoutAccount} onclick={() => openWithdrawal()}>
						<Banknote size={16} />{tr("Tarik Saldo")}
					</button>
				</div>
			</div>
		</div>

		<!-- Info bar: ketentuan biaya penarikan (nilai dari konfigurasi admin) -->
		<div class="app-card mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
			<span class="flex h-9 w-9 flex-none items-center justify-center rounded-lg" style="background: var(--accent);">
				<Landmark size={16} style="color: var(--primary);" />
			</span>
			<!-- Tarif diletakkan di bawah pernyataan "ditanggung mitra" supaya kalimat
			     pokoknya terbaca lebih dulu, rinciannya jadi keterangan. -->
			<div class="min-w-0">
				<p class="text-sm font-medium" style="color: var(--primary);">{tr("Biaya ditanggung mitra.")}</p>
				<p class="mt-0.5 text-xs text-muted-foreground">
					{tr("Biaya transfer")}: {tr("Bank")} {rp(withdrawFee)} · {tr("E-Wallet")} {ewalletFeePct}%
				</p>
			</div>
			<p class="ml-auto text-xs text-muted-foreground">
				{tr("Pembayaran diproses tim Tauthub setelah review Admin dan setiap pengajuan dibuatkan invoice otomatis.")}
			</p>
		</div>

		<div class="app-card mt-4 overflow-hidden">
			<div class="flex flex-wrap items-end justify-between gap-3 border-b px-5 py-4">
				<div>
					<h2 class="text-lg font-bold tracking-tight">{tr("Riwayat Penghasilan")}</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">{tr("Daftar artikel yang sudah menghasilkan dan bisa ditarik.")}</p>
				</div>
				<div class="flex flex-wrap items-center gap-2">
					<label class="relative block">
						<Search size={14} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
						<input class="app-input w-64 max-md:w-full pl-9" placeholder={tr("Cari artikel, media, atau domain...")} bind:value={search} />
					</label>
					<Select
						bind:value={dateFilter}
						ariaLabel={tr("Filter tanggal")}
						placeholder={tr("Semua Tanggal")}
						align="end"
						options={[
							{ value: '', label: tr('Semua Tanggal') },
							{ value: '7', label: tr('7 hari terakhir') },
							{ value: '30', label: tr('30 hari terakhir') },
							{ value: '90', label: tr('90 hari terakhir') }
						]}
					/>
				</div>
			</div>

			{#if filteredRows.length === 0}
				<div class="px-6 py-12 text-center">
					<Wallet size={26} class="mx-auto text-muted-foreground" />
					<p class="mt-3 text-sm font-medium">
						{search.trim() || dateFilter ? tr('Tidak ada penghasilan yang cocok dengan filter ini.') : tr('Belum ada penghasilan.')}
					</p>
					<p class="mt-1 text-xs text-muted-foreground">{tr('Penghasilan tercatat otomatis setelah artikel Anda terbit di media.')}</p>
				</div>
			{:else}
				<!-- Desktop/tablet: tabel asli. Wrapper-nya yang menggulir lokal
				     bila kolom butuh lebih banyak ruang (lihat blok <style>). -->
				<div class="earn-table-wrap hidden md:block">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-5 py-2.5 font-medium">{tr("Artikel")}</th>
								<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
								<th class="px-4 py-2.5 text-right font-medium">{tr("Nilai Diterima")}</th>
								<th class="px-4 py-2.5 font-medium">{tr("Status")}</th>
								<th class="px-4 py-2.5 font-medium">{tr("Tanggal")}</th>
							</tr>
						</thead>
						<tbody>
							{#each filteredRows as e (e.id)}
								<tr class="border-t">
									<td class="px-5 py-2.5">
										<div class="flex items-center gap-2.5">
											{@render earnDocIcon()}
											<span class="max-w-[320px] truncate" title={e.document_title ?? ''}>{e.document_title ?? '-'}</span>
										</div>
									</td>
									<td class="px-4 py-2.5">
										{@render earnMediaCell(e)}
									</td>
									<td class="px-4 py-2.5 text-right font-semibold tabular-nums" style="color: var(--success);">+{rp(e.amount)}</td>
									<td class="px-4 py-2.5">
										{@render earnStatusBadge(e)}
									</td>
									<td class="whitespace-nowrap px-4 py-2.5 text-xs text-muted-foreground">{dt(e.created_at)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- Mobile: setiap earning menjadi kartu vertikal (<768px). Data sama
					dengan tabel (snippet bersama); nominal memakai tabular-nums dan
					min-w-0 supaya tidak terpecah per digit. -->
				<div class="flex flex-col gap-2.5 p-3 md:hidden">
					{#each filteredRows as e (e.id)}
						<article class="app-card p-3.5">
							<!-- Header: ikon dokumen + judul artikel -->
							<div class="flex items-start gap-3">
								{@render earnDocIcon()}
								<div class="min-w-0 flex-1">
									<h3 class="line-clamp-3 text-[15px] font-semibold leading-snug" title={e.document_title ?? ''}>{e.document_title ?? '-'}</h3>
								</div>
							</div>
							<!-- Media dan nominal berdampingan supaya kartu tidak memanjang
							     ke bawah; nominal rata kanan agar mudah dibandingkan. -->
							<dl class="mt-2.5 flex items-center justify-between gap-3 border-t pt-2.5" style="border-color: var(--border);">
								<div class="min-w-0 flex-1">{@render earnMediaCell(e)}</div>
								<div class="min-w-0 shrink-0 text-right">
									<dt class="text-[11px] text-muted-foreground">{tr("Nilai Diterima")}</dt>
									<dd class="mt-0.5 text-[15px] font-semibold tabular-nums" style="color: var(--success);">+{rp(e.amount)}</dd>
								</div>
							</dl>
							<!-- Status & tanggal -->
							<div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5">
								{@render earnStatusBadge(e)}
								<span class="text-xs text-muted-foreground">{dt(e.created_at)}</span>
							</div>
						</article>
					{/each}
				</div>
			{/if}
			<div class="flex flex-wrap items-center justify-between gap-3 border-t px-5 py-3">
				<p class="text-xs text-muted-foreground">
					{tr("Menampilkan")} {filteredRows.length} {tr("dari")} {earningsTotal} {tr("data")}
				</p>
				<div class="flex items-center gap-2">
					<button
						class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
						disabled={offset === 0 || loading}
						onclick={() => changeOffset(offset - PAGE_SIZE)}
					>‹ {tr("Sebelumnya")}</button>
					<span
						class="inline-flex h-8 min-w-8 items-center justify-center rounded-lg text-xs font-semibold"
						style="background: var(--primary); color: var(--primary-foreground);"
					>{Math.floor(offset / PAGE_SIZE) + 1}</span>
					<button
						class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
						disabled={offset + PAGE_SIZE >= earningsTotal || loading}
						onclick={() => changeOffset(offset + PAGE_SIZE)}
					>{tr("Berikutnya")} ›</button>
				</div>
			</div>
		</div>
	{/if}
</section>

{#snippet earnDocIcon()}
	<span class="flex h-8 w-8 flex-none items-center justify-center rounded-md" style="background: var(--accent);">
		<FileText size={14} style="color: var(--primary);" />
	</span>
{/snippet}

{#snippet earnMediaCell(e: any)}
	{@const domain = mediaDomain(e.media_website_url)}
	<div class="flex min-w-0 items-center gap-2">
		<span class="flex h-7 w-7 flex-none items-center justify-center overflow-hidden rounded-full text-[11px] font-semibold" style="background: var(--muted); color: var(--muted-foreground);">
			{#if domain && !faviconFailed[e.id]}
				<img
					src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=64`}
					alt=""
					class="h-full w-full object-cover"
					loading="lazy"
					decoding="async"
					onerror={() => (faviconFailed = { ...faviconFailed, [e.id]: true })}
				/>
			{:else}
				{(e.media_name ?? '?').charAt(0).toUpperCase()}
			{/if}
		</span>
		<span class="min-w-0 leading-tight">
			<span class="block truncate text-muted-foreground" title={e.media_name ?? ''}>{e.media_name ?? '-'}</span>
			{#if e.media_type}
				<span class="block truncate text-[10.5px] font-medium capitalize text-muted-foreground/80">{e.media_type}</span>
			{/if}
		</span>
	</div>
{/snippet}

{#snippet earnStatusBadge(e: any)}
	<span class="inline-flex w-fit items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={statusBadge(e.status)}>
		<span class="h-1.5 w-1.5 rounded-full" style="background: currentColor;"></span>
		{statusLabel[e.status] ?? e.status}
	</span>
	{#if e.status === 'pending' && e.available_at}
		<span class="mt-0.5 block text-[10.5px] text-muted-foreground">{tr("Bisa ditarik")} {dtTime(e.available_at)}</span>
	{/if}
{/snippet}

{#if showWithdraw}
	<!-- Klik di luar / Escape sengaja TIDAK menutup: penarikan adalah aksi uang,
	     jadi hanya Batal, X, atau Ajukan Penarikan yang menutup modal. -->
	<div class="fixed inset-0 z-50 flex items-end justify-center sm:items-center sm:p-4" style="background-color: rgba(15,23,42,.5);" role="presentation">
		<dialog
			open
			class="flex max-h-[92vh] w-full flex-col overflow-hidden rounded-t-2xl border bg-white shadow-2xl sm:max-h-[90vh] sm:max-w-[700px] sm:rounded-2xl"
			style="border-color: var(--border);"
			aria-label={tr("Tarik Saldo")}
		>
			<!-- Header -->
			<div class="flex items-start justify-between gap-4 px-5 pt-5 sm:px-6">
				<div class="flex items-start gap-3.5">
					<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl" style="background-color: var(--accent); color: var(--primary);">
						<Wallet size={20} strokeWidth={2} />
					</span>
					<div class="min-w-0">
						<h2 class="text-xl font-semibold tracking-tight">{tr("Tarik Saldo")}</h2>
						<p class="mt-0.5 text-[13px] leading-relaxed text-muted-foreground">
							{tr("Pilih jumlah penarikan dan periksa rincian sebelum mengajukan.")}
						</p>
					</div>
				</div>
				<button
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border transition-colors hover:bg-slate-50"
					style="border-color: var(--border); color: var(--muted-foreground);"
					aria-label={tr("Tutup")}
					onclick={() => (showWithdraw = false)}
				>
					<X size={16} />
				</button>
			</div>

			<!-- Body (scroll internal di layar pendek) -->
			<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-5 pt-4 sm:px-6">
				{#if error}
					<div class="mb-4 rounded-lg border px-3 py-2 text-sm" style="border-color: var(--destructive); background-color: rgba(197,43,54,.08); color: var(--destructive);">{error}</div>
				{/if}

				<!-- Saldo tersedia -->
				<div class="flex flex-wrap items-center gap-3.5 rounded-xl border px-4 py-3.5" style="background-color: var(--accent); border-color: color-mix(in srgb, var(--primary) 18%, var(--border));">
					<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white/80" style="color: var(--primary);">
						<Coins size={18} strokeWidth={2} />
					</span>
					<div class="min-w-0 flex-1">
						<p class="text-xs font-medium text-muted-foreground">{tr("Saldo tersedia")}</p>
						<p class="text-2xl font-bold leading-tight tabular-nums" style="color: var(--foreground);">{rp(settledBalance)}</p>
					</div>
					<p class="hidden max-w-[210px] text-[11.5px] leading-relaxed text-muted-foreground sm:block">
						{tr("Rekening tujuan diambil dari Profil dan tidak dapat diubah di sini.")}
					</p>
				</div>

				<!-- Pilihan jumlah penarikan -->
				<div class="mt-4 grid gap-2 sm:grid-cols-2" role="radiogroup" aria-label={tr("Jumlah Penarikan")}>
					<button
						type="button"
						role="radio"
						aria-checked={mode === 'all'}
						class="flex items-start gap-2.5 rounded-xl border px-3.5 py-3 text-left text-sm transition-colors"
						style={mode === 'all'
							? 'border-color: var(--primary); background-color: var(--accent);'
							: 'border-color: var(--border); background-color: #fff;'}
						onclick={() => selectMode('all')}
					>
						<span class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2" style={mode === 'all' ? 'border-color: var(--primary);' : 'border-color: #cbd5e1;'}>
							{#if mode === 'all'}<span class="h-2 w-2 rounded-full" style="background-color: var(--primary);"></span>{/if}
						</span>
						<span class="min-w-0">
							<span class="block text-[13.5px] font-semibold" style="color: var(--foreground);">{tr("Tarik seluruh saldo")}</span>
							<span class="mt-0.5 block text-xs tabular-nums text-muted-foreground">{rp(settledBalance)}</span>
						</span>
					</button>
					<button
						type="button"
						role="radio"
						aria-checked={mode === 'partial'}
						class="flex items-start gap-2.5 rounded-xl border px-3.5 py-3 text-left text-sm transition-colors"
						style={mode === 'partial'
							? 'border-color: var(--primary); background-color: var(--accent);'
							: 'border-color: var(--border); background-color: #fff;'}
						onclick={() => selectMode('partial')}
					>
						<span class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2" style={mode === 'partial' ? 'border-color: var(--primary);' : 'border-color: #cbd5e1;'}>
							{#if mode === 'partial'}<span class="h-2 w-2 rounded-full" style="background-color: var(--primary);"></span>{/if}
						</span>
						<span class="min-w-0">
							<span class="block text-[13.5px] font-semibold" style="color: var(--foreground);">{tr("Tarik sebagian")}</span>
							<span class="mt-0.5 block text-xs text-muted-foreground">{tr("Masukkan nominal yang ingin ditarik.")}</span>
						</span>
					</button>
				</div>

				<!-- Nominal -->
				{#if mode === 'partial'}
					<label class="mt-4 block">
						<span class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
							<span class="text-[13px] font-medium" style="color: var(--foreground);">{tr("Nominal (IDR)")} <span style="color: var(--destructive);">*</span></span>
							{#if partialError}
								<!-- Pesan validasi menempel pada label dan SELALU merah,
								     termasuk saat nominal masih kosong. -->
								<span class="text-[11.5px] font-medium" style="color: var(--destructive);">{partialError}</span>
							{/if}
						</span>
						<input
							class="app-input mt-1.5 w-full tabular-nums"
							type="text"
							inputmode="numeric"
							autocomplete="off"
							placeholder={tr("Masukkan nominal penarikan")}
							value={amountText}
							disabled={busy}
							onkeydown={onAmountKeydown}
							oninput={onAmountInput}
						/>
						<span class="mt-1.5 block text-xs text-muted-foreground">
							{tr("Minimal")} {rp(minimumWithdraw)} · {tr("Maksimal")} {rp(settledBalance)}
						</span>
					</label>
				{/if}

				<!-- Catatan -->
				<label class="mt-4 block">
					<span class="text-[13px] font-medium" style="color: var(--foreground);">{tr("Catatan (opsional)")}</span>
					<input class="app-input mt-1.5 w-full" placeholder={tr("Tulis catatan jika diperlukan")} bind:value={notes} disabled={busy} />
				</label>

				<!-- Rincian penarikan -->
				<div class="mt-4 rounded-xl border px-4 py-3 text-[13px]" style="background-color: var(--muted); border-color: var(--border);">
					<div class="flex items-center justify-between gap-3">
						<span class="text-muted-foreground">{tr("Bruto")}</span>
						<span class="font-medium tabular-nums">{rp(mode === 'partial' ? Number(amount) || 0 : settledBalance)}</span>
					</div>
					<div class="mt-1.5 flex items-center justify-between gap-3">
						<span class="text-muted-foreground">{tr("Biaya transfer")} ({payoutType === 'ewallet' ? tr('E-Wallet') : tr('Bank')})</span>
						<span class="font-medium tabular-nums">{rp(effectiveFee)}</span>
					</div>
					<div class="mt-2.5 flex items-center justify-between gap-3 border-t pt-2.5" style="border-color: var(--border);">
						<span class="font-semibold" style="color: var(--foreground);">{tr("Diterima (net)")}</span>
						<span class="text-[15px] font-bold tabular-nums" style="color: var(--foreground);">{rp(netWithdraw)}</span>
					</div>
				</div>

				<!-- Rekening tujuan -->
				{#if hasPayoutAccount}
					<div class="mt-4 flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3" style="background-color: var(--accent); border-color: color-mix(in srgb, var(--primary) 18%, var(--border));">
						<span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-white/80" style="color: var(--primary);">
							<Landmark size={17} strokeWidth={2} />
						</span>
						<div class="min-w-0 flex-1">
							<p class="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{tr("Tujuan penarikan")}</p>
							<p class="mt-0.5 truncate text-[13.5px] font-semibold" style="color: var(--foreground);">{partner?.bank_name || tr('Bank')}</p>
							<p class="mt-0.5 truncate text-xs tabular-nums text-muted-foreground">
								{maskedAccountNumber}{#if partner?.bank_account_name} · {partner.bank_account_name}{/if}
							</p>
						</div>
						<div class="flex shrink-0 flex-col items-end gap-1">
							<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--secondary); color: var(--accent-foreground);">{tr("Dari Profil")}</span>
							<p class="text-[11px] text-muted-foreground">{tr("Ubah di Profil")}</p>
						</div>
					</div>
				{:else}
					<div class="mt-4 rounded-xl border px-4 py-3 text-xs" style="background-color: var(--warning-soft); border-color: var(--warning);">
						<p class="font-medium" style="color: var(--warning);">{tr("Data rekening atau e-wallet belum diisi.")}</p>
						<p class="mt-1 text-muted-foreground">{tr("Isi tujuan penerimaan dana di profil sebelum mengajukan penarikan.")}</p>
						<a class="mt-2 inline-flex items-center gap-1 font-medium" style="color: var(--primary);" href={`${base}/profile`}>
							<Wallet size={12} /> {tr("Buka Profil")}
						</a>
					</div>
				{/if}
			</div>

			<!-- Footer aksi -->
			<div
				class="flex items-center justify-end gap-2.5 border-t px-5 py-3.5 sm:px-6"
				style="border-color: var(--border); background-color: #fff; padding-bottom: calc(0.875rem + env(safe-area-inset-bottom));"
			>
				<button class="app-btn app-btn-soft min-h-11 rounded-[9px] px-5" onclick={() => (showWithdraw = false)}>{tr("Batal")}</button>
				<button
					class="app-btn app-btn-primary min-h-11 rounded-[9px] px-5"
					disabled={busy || !hasPayoutAccount || withdrawBlocked || Boolean(partialError)}
					onclick={withdraw}
				>
					{busy ? tr('Mengirim…') : tr('Ajukan Penarikan')}
				</button>
			</div>
		</dialog>
	</div>
{/if}

<style>
	/* Tabel Earnings Details: di bawah 1400px area konten menyempit karena
	   sidebar, sehingga enam kolom tidak muat nyaman (di 768px tinggi header
	   kolom sampai 100px) dan di 1280px tabel terpotong overflow-hidden kartu.
	   Solusinya: lebar minimum kolom + gulir lokal pada wrapper tabel.
	   `!important` diperlukan karena aturan global `main table` di app.css
	   memaksa min-width:0 pada semua tabel di bawah 1280px. */
	@media (min-width: 768px) and (max-width: 1399px) {
		.earn-table-wrap {
			min-width: 0;
			max-width: 100%;
			overflow-x: auto;
			-webkit-overflow-scrolling: touch;
		}
		.earn-table-wrap > table {
			/* Lima kolom: cukup agar judul kolom satu baris dan kolom angka tidak
			   berdesakan; di area lebih sempit tabel digulir lokal di dalam kartu. */
			min-width: 860px !important;
		}
	}
</style>
