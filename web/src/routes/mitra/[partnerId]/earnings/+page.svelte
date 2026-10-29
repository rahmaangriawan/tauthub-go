<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Penghasilan mitra — port section "Penghasilan & Saldo": ringkasan saldo,
	// daftar earning per publikasi dengan paginasi offset (limit/offset/total
	// dari API), penarikan dana dengan metode terkunci dari profil mitra
	// (payout_type) + biaya transfer dari tc_config, dan guard rekening.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banknote, Calendar, Clock, Coins, FileText, Landmark, Search, Wallet, X } from '@lucide/svelte';
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
			showWithdraw = false;
			await load();
		} finally {
			busy = false;
		}
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-');
	// $derived, bukan konstanta: tr() membaca locale yang baru tersedia setelah
	// initLocaleFromStorage() di root layout, jadi konstanta biasa akan membeku
	// di bahasa awal dan badge tidak ikut berganti ke Inggris.
	const statusLabel = $derived<Record<string, string>>({
		pending: tr('Menunggu cair'), settled: tr('Siap ditarik'), voided: tr('Dibatalkan')
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
			<h1 class="text-2xl font-semibold tracking-tight">{tr("Penghasilan")}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Saldo mitra bertambah saat artikel terbit di media Anda.")}</p>
		</div>
		<!-- Cadangan aksi penarikan saat ringkasan saldo gagal dimuat. -->
		{#if !loading && !data}
			<button class="app-btn app-btn-primary min-h-11 shrink-0" disabled={!hasPayoutAccount} onclick={() => (showWithdraw = true)}>
				<Banknote size={16} />{tr("Tarik Saldo")}
			</button>
		{/if}
	</header>

	{#if notice}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{notice}</div>
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
				</div>

				<div class="hidden h-12 w-px sm:block" style="background: var(--border);"></div>

				<div class="flex items-center gap-5">
					<div class="flex items-center gap-2.5">
						<span class="flex h-10 w-10 items-center justify-center rounded-lg" style="background: var(--warning-soft);">
							<Clock size={18} style="color: var(--warning);" />
						</span>
						<div>
							<p class="text-xs text-muted-foreground">{tr("Pending payout")}</p>
							<p class="text-sm font-semibold tabular-nums">{rp(data.summary?.pending)}</p>
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

				<button class="app-btn app-btn-primary md:ml-auto min-h-11 shrink-0 px-5" disabled={!hasPayoutAccount} onclick={() => (showWithdraw = true)}>
					<Banknote size={16} />{tr("Tarik Saldo")}
				</button>
			</div>
		</div>

		<!-- Info bar: ketentuan biaya penarikan (nilai dari konfigurasi admin) -->
		<div class="app-card mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
			<span class="flex h-9 w-9 flex-none items-center justify-center rounded-lg" style="background: var(--accent);">
				<Landmark size={16} style="color: var(--primary);" />
			</span>
			<p class="text-sm">
				<span class="font-semibold">{tr("Biaya transfer")}</span>
				<span class="text-muted-foreground">
					{tr("Bank")} {rp(withdrawFee)} · {tr("E-Wallet")} {ewalletFeePct}%
				</span>
			</p>
			<p class="text-sm font-medium" style="color: var(--primary);">{tr("Biaya ditanggung mitra.")}</p>
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
		<span class="min-w-0 text-muted-foreground" title={e.media_name ?? ''}>{e.media_name ?? '-'}</span>
	</div>
{/snippet}

{#snippet earnStatusBadge(e: any)}
	<span class="inline-flex w-fit items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium" style={statusBadge(e.status)}>
		<span class="h-1.5 w-1.5 rounded-full" style="background: currentColor;"></span>
		{statusLabel[e.status] ?? e.status}
	</span>
{/snippet}

{#if showWithdraw}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={(event) => event.currentTarget === event.target && (showWithdraw = false)} onkeydown={(e) => { if (e.key === 'Escape') showWithdraw = false; }}>
		<dialog open class="app-card my-auto w-full max-w-md p-5" aria-label={tr("Tarik Saldo")}>
			<div class="flex items-start justify-between gap-4">
				<h2 class="text-lg font-semibold">{tr("Tarik Saldo")}</h2>
				<button class="app-btn app-btn-soft !min-h-9 !w-9 !p-0" aria-label={tr("Tutup")} onclick={() => (showWithdraw = false)}><X size={16} /></button>
			</div>
			<p class="mt-1 text-xs leading-relaxed text-muted-foreground">
				{tr("Metode pencairan dan rekening tujuan diambil otomatis dari Profil (terkunci, tidak bisa diubah di sini).")}
				{tr("Biaya transfer ditanggung mitra:")} {tr("Bank")} {rp(withdrawFee)} / {tr("E-Wallet")} {ewalletFeePct}% {tr("dari nominal, terpotong otomatis.")}
				{tr("Saldo siap ditarik")} {rp(settledBalance)}.
			</p>
			{#if error}
				<div class="mt-3 rounded-lg border px-3 py-2 text-sm" style="border-color: var(--destructive); background-color: rgba(197,43,54,.08); color: var(--destructive);">{error}</div>
			{/if}
			<div class="mt-4 space-y-4">
				<div>
					<p class="text-xs font-semibold text-muted-foreground">{tr("Jumlah Penarikan")}</p>
					<div class="mt-2 grid gap-2">
						<label class="flex cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2.5 text-sm" style="border-color: {mode === 'all' ? 'var(--primary)' : 'var(--border)'}; background-color: {mode === 'all' ? 'var(--accent)' : 'transparent'};">
							<input type="radio" value="all" bind:group={mode} />
							<span class="min-w-0 break-words">{tr("Tarik seluruh saldo")} <strong class="tabular-nums">({rp(settledBalance)})</strong></span>
						</label>
						<label class="flex cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2.5 text-sm" style="border-color: {mode === 'partial' ? 'var(--primary)' : 'var(--border)'}; background-color: {mode === 'partial' ? 'var(--accent)' : 'transparent'};">
							<input type="radio" value="partial" bind:group={mode} />
							<span>{tr("Tarik sebagian")}</span>
						</label>
					</div>
				</div>
				{#if mode === 'partial'}
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr("Nominal (IDR)")} *</span>
						<input class="app-input mt-1 w-full" type="number" min="0" bind:value={amount} disabled={busy} />
						<span class="mt-1 block text-xs text-muted-foreground">
							{tr("Minimal")} {rp(minimumWithdraw)} · {tr("maksimal")} {rp(settledBalance)}
						</span>
					</label>
				{/if}
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr("Catatan (opsional)")}</span>
					<input class="app-input mt-1 w-full" bind:value={notes} disabled={busy} />
				</label>
				<div class="rounded-lg border p-3 text-xs" style="background-color: var(--muted);">
					<div class="flex justify-between"><span>{tr("Bruto")}</span><span class="tabular-nums">{rp(mode === 'partial' ? Number(amount) || 0 : settledBalance)}</span></div>
					<div class="mt-1 flex justify-between"><span>{tr("Biaya")} ({payoutType === 'ewallet' ? tr('E-Wallet') : tr('Bank')})</span><span class="tabular-nums">{rp(effectiveFee)}</span></div>
					<div class="mt-2 flex justify-between border-t pt-2 font-semibold" style="color: var(--foreground);"><span>{tr("Diterima (net)")}</span><span class="tabular-nums">{rp(netWithdraw)}</span></div>
				</div>
				{#if hasPayoutAccount}
					<div class="rounded-lg border p-3 text-xs" style="background-color: var(--accent); border-color: var(--border);">
						<div class="flex items-center justify-between gap-2">
							<p class="font-semibold" style="color: var(--accent-foreground);">{tr("Tujuan Pencairan dari Profil")}</p>
							<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: var(--accent-foreground);">{payoutType === 'ewallet' ? tr('E-Wallet') : tr('Bank')}</span>
						</div>
						<p class="mt-1 font-medium" style="color: var(--accent-foreground);">{payoutAccountLabel}</p>
						<p class="mt-1 text-muted-foreground">{tr("Untuk mengubah tujuan dana, buka Profil.")}</p>
					</div>
				{:else}
					<div class="rounded-lg border p-3 text-xs" style="background-color: var(--warning-soft); border-color: var(--warning);">
						<p class="font-medium" style="color: var(--warning);">{tr("Data rekening atau e-wallet belum diisi.")}</p>
						<p class="mt-1 text-muted-foreground">{tr("Isi tujuan penerimaan dana di profil sebelum mengajukan penarikan.")}</p>
						<a class="mt-2 inline-flex items-center gap-1 font-medium" style="color: var(--primary);" href={`${base}/profile`}>
							<Wallet size={12} /> {tr("Buka Profil")}
						</a>
					</div>
				{/if}
			</div>
			<div class="mt-5 flex justify-end gap-2">
				<button class="app-btn app-btn-soft min-h-11" onclick={() => (showWithdraw = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary min-h-11" disabled={busy || !hasPayoutAccount || withdrawBlocked} onclick={withdraw}>{busy ? tr('Mengirim…') : tr('Ajukan Penarikan')}</button>
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
