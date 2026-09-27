<script lang="ts">
	// VIP Dashboard mitra — port penuh section "VIP Dashboard" (vip-dashboard.tsx,
	// blueprint Mas Aqil): status keanggotaan, dual-wallet (CPA, komisi transaksi,
	// referral), Performance + Statistik Affiliate, riwayat penghasilan per domain
	// sukses, dan riwayat earning dengan sparkline kumulatif.
	import { onMount } from 'svelte';
	import {
		CircleCheck, CircleX, Clock, Coins, Crown, Globe, HandCoins,
		MousePointerClick, Users, Wallet
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type WalletHistory = {
		id: string; kind: string; amount: number; client_price: number; status: string;
		created_at: string; source_referred_domain_id: string | null;
	};
	type WalletData = { cpa: number; transaction_commission: number; publication: number; referral: number; history: WalletHistory[] };
	type DomainsData = {
		items: Array<{
			id: string; domain_name: string; status: string; rejection_reason: string | null;
			cpa_paid: boolean; cpa_amount: number | null; is_tld: boolean; created_at: string;
			owner_email: string | null; media_name: string | null;
		}>;
		stats: { total: number; approved: number; pending: number; rejected: number };
		clicks: number;
	};
	type VipInfo = {
		isVip: boolean; hasVip: boolean; tier: string; isExpired: boolean;
		vipExpiresAt: string | null; vipContractMonths: number | null; vipContractDays: number | null;
		commissionRateBp: number; cpaRate: number; cpaTierThreshold: number;
		cpaTier1Rate: number; cpaTier2Rate: number;
	};
	type ReferralData = {
		totalReferrals: number; referral_earning: number;
		clicks?: { total: number; unique: number };
	};

	let wallet = $state<WalletData | null>(null);
	let domains = $state<DomainsData | null>(null);
	let vip = $state<VipInfo | null>(null);
	let referral = $state<ReferralData | null>(null);
	let error = $state('');
	let loading = $state(true);

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
		const [vipRes, walletRes, domainsRes, refRes] = await Promise.all([
			apiRequest('/v1/partner/me/vip'),
			apiRequest('/v1/partner/vip/wallet'),
			apiRequest('/v1/partner/vip/domains'),
			apiRequest('/v1/partner/referrals')
		]);
		if (vipRes.ok) vip = await vipRes.json();
		if (walletRes.ok) wallet = await walletRes.json();
		else error = (await walletRes.json().catch(() => null))?.detail ?? 'Gagal memuat dompet VIP.';
		if (domainsRes.ok) domains = await domainsRes.json();
		if (refRes.ok) referral = await refRes.json();
	}

	/**
	 * Bangun cumulative time series untuk sparkline Saldo dari wallet.history.
	 * - History kosong → flat series di nilai current.
	 * - History ada → kumulatif (ASC by created_at), disample ke maxPoints, titik akhir di-anchor ke current.
	 */
	function buildWalletSparkSeries(history: WalletHistory[] | undefined, kind: 'cpa' | 'transaction_commission' | 'all', currentValue: number, maxPoints = 10): number[] {
		const flat = (v: number) => new Array(maxPoints).fill(v);
		if (!history || history.length === 0) return flat(currentValue > 0 ? currentValue : 0);
		const filtered = kind === 'all' ? history : history.filter((h) => h.kind === kind);
		if (filtered.length === 0) return flat(currentValue > 0 ? currentValue : 0);
		const sorted = [...filtered].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
		const cum: number[] = [];
		let acc = 0;
		for (const h of sorted) {
			acc += Number(h.amount) || 0;
			cum.push(acc);
		}
		let series = cum;
		if (series.length > maxPoints) {
			const step = (series.length - 1) / (maxPoints - 1);
			series = Array.from({ length: maxPoints }, (_, i) => series[Math.round(i * step)]);
		}
		if (series.length > 0) series[series.length - 1] = currentValue;
		if (currentValue <= 0) return flat(0);
		return series;
	}

	/** Bangun path SVG halus (midpoint-QT) untuk line + area sparkline. */
	function buildSparkPath(values: number[]) {
		const w = 100;
		const h = 40;
		const pad = 5;
		const min = Math.min(...values);
		const max = Math.max(...values);
		const range = max - min || 1;
		const pts = values.map((v, i) => ({
			x: (i / (values.length - 1)) * w,
			y: h - pad - ((v - min) / range) * (h - pad * 2)
		}));
		let line = `M${pts[0].x.toFixed(2)},${pts[0].y.toFixed(2)}`;
		for (let i = 0; i < pts.length - 1; i++) {
			const midX = (pts[i].x + pts[i + 1].x) / 2;
			const midY = (pts[i].y + pts[i + 1].y) / 2;
			if (i === 0) line += ` Q${pts[i].x.toFixed(2)},${pts[i].y.toFixed(2)} ${midX.toFixed(2)},${midY.toFixed(2)}`;
			else line += ` T${midX.toFixed(2)},${midY.toFixed(2)}`;
		}
		line += ` T${pts[pts.length - 1].x.toFixed(2)},${pts[pts.length - 1].y.toFixed(2)}`;
		const area = `${line} L${pts[pts.length - 1].x.toFixed(2)},${h} L${pts[0].x.toFixed(2)},${h} Z`;
		return { line, area };
	}

	const idr = (n: number) => 'Rp' + Number(n || 0).toLocaleString('id-ID');
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '—');
	const KIND_LABEL: Record<string, string> = { cpa: 'CPA', transaction_commission: 'Komisi Transaksi', referral: 'Referral', publication: 'Publikasi' };
	const STATUS_LABEL: Record<string, string> = { pending: 'Menunggu', approved: 'Disetujui', rejected: 'Ditolak', available: 'Tersedia', paid: 'Dibayar', reversed: 'Dibalik', settled: 'Selesai', voided: 'Dibatalkan' };
	const statusColor = (s: string) =>
		s === 'approved' || s === 'paid' || s === 'settled' ? 'var(--success)'
		: s === 'rejected' || s === 'reversed' ? 'var(--destructive)'
		: s === 'available' ? 'var(--primary)'
		: s === 'voided' ? 'var(--muted-foreground)'
		: 'var(--warning)';
	const tierLabel = $derived(vip?.tier === 'brand_ambassador' || vip?.tier === 'vvip' ? 'VVIP' : vip?.tier === 'vip_reguler' ? 'VIP Reguler' : 'VIP');

	const vipActive = $derived(Boolean(vip?.isVip) && !vip?.isExpired);
	const showCpa = $derived(vipActive);
	const approvedDomains = $derived((domains?.items ?? []).filter((d) => d.status === 'approved'));
	const totalCpaPaid = $derived(approvedDomains.reduce((sum, d) => sum + (Number(d.cpa_amount) || 0), 0));

	const cpaSeries = $derived(buildWalletSparkSeries(wallet?.history, 'cpa', wallet?.cpa ?? 0));
	const komisiSeries = $derived(buildWalletSparkSeries(wallet?.history, 'transaction_commission', wallet?.transaction_commission ?? 0));
	const totalSeries = $derived(buildWalletSparkSeries(wallet?.history, 'cpa', totalCpaPaid));
	const cpaPath = $derived(buildSparkPath(cpaSeries));
	const komisiPath = $derived(buildSparkPath(komisiSeries));
	const totalPath = $derived(buildSparkPath(totalSeries));

	const performanceRows = $derived([
		{ label: 'Total Klik', value: String(referral?.clicks?.total ?? domains?.clicks ?? 0), icon: MousePointerClick, color: 'var(--foreground)' },
		{ label: 'Total Referral', value: String(referral?.totalReferrals ?? 0), icon: Users, color: 'var(--foreground)' },
		{ label: 'Komisi Referral', value: idr(referral?.referral_earning ?? 0), icon: Wallet, color: 'var(--success)' },
		{ label: 'Komisi Transaksi', value: idr(wallet?.transaction_commission ?? 0), icon: HandCoins, color: 'var(--foreground)' }
	]);

	const affiliateRows = $derived([
		{ label: 'Total Klik', value: String(domains?.clicks ?? 0), icon: MousePointerClick, color: 'var(--foreground)' },
		{ label: 'Domain Terdaftar', value: String(domains?.stats?.total ?? 0), icon: Globe, color: 'var(--primary)' },
		{ label: 'Approved', value: String(domains?.stats?.approved ?? 0), icon: CircleCheck, color: 'var(--success)' },
		{ label: 'Pending', value: String(domains?.stats?.pending ?? 0), icon: Clock, color: 'var(--warning)' },
		{ label: 'Ditolak', value: String(domains?.stats?.rejected ?? 0), icon: CircleX, color: 'var(--destructive)' },
		{ label: 'Estimasi CPA', value: idr((domains?.stats?.approved ?? 0) * (vip?.cpaRate ?? 15000)), icon: Coins, color: 'var(--primary)' }
	]);
</script>

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<p class="text-xs font-semibold tracking-wide text-muted-foreground">VIP MITRA</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">VIP Dashboard</h1>
			<p class="mt-1 text-sm text-muted-foreground">Ringkasan affiliate, dual-wallet, dan domain referral Anda.</p>
		</div>
		<div class="flex items-center gap-2">
			{#if vip}
				<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
					<Crown size={12} /> {tierLabel}
				</span>
			{:else}
				<span class="rounded-full px-2.5 py-1 text-xs font-medium text-muted-foreground" style="background-color: var(--secondary);">Memuat...</span>
			{/if}
		</div>
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat VIP Dashboard…</p>
	{:else}
		{#if vip && !vipActive}
			<div class="app-card mt-3 flex items-start gap-2 px-4 py-3 text-sm" style="border-color: var(--warning); color: var(--warning);">
				<Clock size={16} class="mt-0.5 shrink-0" />
				<p>
					{#if vip.isExpired || vip.vipExpiresAt}
						Masa kontrak VIP telah berakhir{#if vip.vipContractMonths != null} ({vip.vipContractMonths} bulan, berakhir {vip.vipExpiresAt ? dt(vip.vipExpiresAt) : '—'}){/if}. CPA Balance nonaktif. Hubungi Admin untuk perpanjangan.
					{:else}
						Status VIP tidak aktif saat ini.
					{/if}
				</p>
			</div>
		{/if}

		<!-- ── Saldo ── -->
		<div class="mt-4 rounded-xl border p-4">
			<h2 class="text-sm font-semibold">Saldo</h2>
			<div class="mt-3 grid gap-3" style="grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
				{#if showCpa}
					<div class="relative overflow-hidden rounded-lg border px-3.5 py-3">
						<div class="absolute inset-y-0 left-0 w-[3px]" style="background-color: var(--success);"></div>
						<div class="flex min-h-4 items-center justify-between gap-2">
							<span class="text-[11px] font-bold uppercase tracking-wide text-muted-foreground">CPA Balance</span>
							{#if vip?.vipExpiresAt}
								<span class="shrink-0 whitespace-nowrap rounded-full px-2 py-0.5 text-[10.5px] font-semibold uppercase tracking-wide" style="background-color: var(--success-soft); color: var(--success);">Berakhir {dt(vip.vipExpiresAt)}</span>
							{/if}
						</div>
						<p class="mt-1 text-xl font-bold tabular-nums" style="color: var(--success);">{idr(wallet?.cpa ?? 0)}</p>
						<p class="mt-1 text-xs text-muted-foreground">
							{#if vip?.tier === 'vip_reguler'}
								{idr(vip.cpaTier1Rate)}/domain untuk {vip.cpaTierThreshold.toLocaleString('id-ID')} domain pertama, lalu {idr(vip.cpaTier2Rate)}/domain
							{:else}
								{idr(vip?.cpaRate ?? 0)} per domain
							{/if}
						</p>
						<svg viewBox="0 0 100 40" preserveAspectRatio="none" width="100%" height="26" aria-hidden="true" class="mt-1.5 block">
							<path d={cpaPath.area} fill="var(--success)" fill-opacity="0.1" />
							<path d={cpaPath.line} fill="none" stroke="var(--success)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
						</svg>
					</div>
				{/if}
				<div class="relative overflow-hidden rounded-lg border px-3.5 py-3">
					<div class="absolute inset-y-0 left-0 w-[3px]" style="background-color: var(--primary);"></div>
					<span class="text-[11px] font-bold uppercase tracking-wide text-muted-foreground">Komisi Transaksi</span>
					<p class="mt-1 text-xl font-bold tabular-nums" style="color: var(--primary);">{idr(wallet?.transaction_commission ?? 0)}</p>
					{#if vip}<p class="mt-1 text-xs text-muted-foreground">{(vip.commissionRateBp / 100).toFixed(1)}% dari transaksi</p>{/if}
					<svg viewBox="0 0 100 40" preserveAspectRatio="none" width="100%" height="26" aria-hidden="true" class="mt-1.5 block">
						<path d={komisiPath.area} fill="var(--primary)" fill-opacity="0.1" />
						<path d={komisiPath.line} fill="none" stroke="var(--primary)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
					</svg>
				</div>
				<div class="relative overflow-hidden rounded-lg border px-3.5 py-3">
					<div class="absolute inset-y-0 left-0 w-[3px]" style="background-color: var(--muted-foreground);"></div>
					<span class="text-[11px] font-bold uppercase tracking-wide text-muted-foreground">Total</span>
					<p class="mt-1 text-xl font-bold tabular-nums">{idr(totalCpaPaid)}</p>
					<p class="mt-1 text-xs text-muted-foreground">Akumulasi CPA · {approvedDomains.length} domain disetujui × {idr(vip?.cpaRate ?? 15000)}</p>
					<svg viewBox="0 0 100 40" preserveAspectRatio="none" width="100%" height="26" aria-hidden="true" class="mt-1.5 block">
						<path d={totalPath.area} fill="var(--muted-foreground)" fill-opacity="0.1" />
						<path d={totalPath.line} fill="none" stroke="var(--muted-foreground)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
					</svg>
				</div>
			</div>
		</div>

		<!-- ── Performance + Statistik Affiliate ── -->
		<div class="mt-4 grid gap-3 md:grid-cols-2">
			<div class="overflow-hidden rounded-xl border">
				<div class="border-b px-4 py-3"><h2 class="text-sm font-semibold">Performance</h2></div>
				<div>
					{#each performanceRows as row (row.label)}
						<div class="flex items-center gap-3 border-b px-4 py-2.5 last:border-b-0">
							<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md" style="background-color: var(--secondary);"><row.icon size={15} /></span>
							<span class="flex-1 text-sm">{row.label}</span>
							<span class="whitespace-nowrap text-sm font-semibold tabular-nums" style="color: {row.color};">{row.value}</span>
						</div>
					{/each}
				</div>
			</div>

			<div class="overflow-hidden rounded-xl border">
				<div class="border-b px-4 py-3"><h2 class="text-sm font-semibold">Statistik Affiliate</h2></div>
				<div>
					{#each affiliateRows as row (row.label)}
						<div class="flex items-center gap-3 border-b px-4 py-2.5 last:border-b-0">
							<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md" style="background-color: var(--secondary);"><row.icon size={15} /></span>
							<span class="flex-1 text-sm">{row.label}</span>
							<span class="whitespace-nowrap text-sm font-semibold tabular-nums" style="color: {row.color};">{row.value}</span>
						</div>
					{/each}
				</div>
			</div>
		</div>

		<!-- ── Riwayat Penghasilan Affiliate (per domain sukses) ── -->
		<div class="app-card mt-3 overflow-x-auto">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
				<h2 class="text-sm font-semibold">Riwayat Penghasilan Affiliate</h2>
				<div class="flex items-center gap-2">
					<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">{approvedDomains.length} domain sukses</span>
					<span class="rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground" style="background-color: var(--secondary);">{domains?.items?.length ?? 0} domain</span>
				</div>
			</div>
			{#if approvedDomains.length > 0}
				<div class="border-b px-4 py-2.5 text-xs" style="color: var(--success);">
					Total penghasilan CPA dari {approvedDomains.length} domain approved: <strong>{idr(totalCpaPaid)}</strong> (per domain sukses × {idr(vip?.cpaRate ?? 15000)}).
				</div>
			{/if}
			{#if !domains || domains.items.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada domain referral. Bagikan link referral Anda untuk memulai.</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">Domain</th>
							<th class="px-4 py-2.5 font-medium">Owner</th>
							<th class="px-4 py-2.5 font-medium">Status</th>
							<th class="px-4 py-2.5 font-medium">CPA</th>
							<th class="px-4 py-2.5 font-medium">Tanggal</th>
						</tr>
					</thead>
					<tbody>
						{#each domains.items as d (d.id)}
							<tr class="border-t" style={d.status === 'approved' ? 'background-color: var(--success-soft);' : ''}>
								<td class="px-4 py-2.5">
									<div class="flex items-center gap-2">
										<Globe size={14} />
										<div>
											<p class="font-medium">{d.domain_name}</p>
											<p class="text-xs text-muted-foreground">{d.media_name ?? '—'}</p>
										</div>
									</div>
								</td>
								<td class="px-4 py-2.5 text-muted-foreground">{d.owner_email ?? '—'}</td>
								<td class="px-4 py-2.5">
									<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--secondary); color: {statusColor(d.status)};">{STATUS_LABEL[d.status] ?? d.status}</span>
									{#if d.rejection_reason}<p class="mt-1 text-xs" style="color: var(--destructive);">{d.rejection_reason}</p>{/if}
								</td>
								<td class="px-4 py-2.5">
									{#if d.cpa_paid}
										<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">{idr(d.cpa_amount ?? 0)}</span>
									{:else}—{/if}
								</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(d.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>

		<!-- ── Riwayat Earning ── -->
		<div class="app-card mt-3 overflow-x-auto">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
				<h2 class="text-sm font-semibold">Riwayat Earning</h2>
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground" style="background-color: var(--secondary);">{wallet?.history?.length ?? 0} transaksi</span>
			</div>
			{#if !wallet || wallet.history.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada riwayat earning.</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">Jenis</th>
							<th class="px-4 py-2.5 font-medium">Jumlah</th>
							<th class="px-4 py-2.5 font-medium">Status</th>
							<th class="px-4 py-2.5 font-medium">Tanggal</th>
						</tr>
					</thead>
					<tbody>
						{#each wallet.history as h (h.id)}
							<tr class="border-t">
								<td class="px-4 py-2.5">
									<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--secondary); color: {h.kind === 'cpa' ? 'var(--success)' : h.kind === 'transaction_commission' ? 'var(--primary)' : h.kind === 'referral' ? 'var(--accent-foreground)' : 'var(--warning)'};">{KIND_LABEL[h.kind] ?? h.kind}</span>
								</td>
								<td class="px-4 py-2.5 font-medium tabular-nums">{idr(h.amount)}</td>
								<td class="px-4 py-2.5">
									<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--secondary); color: {statusColor(h.status)};">{STATUS_LABEL[h.status] ?? h.status}</span>
								</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(h.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>
