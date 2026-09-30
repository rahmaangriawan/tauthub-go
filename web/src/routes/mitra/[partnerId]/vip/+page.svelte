<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// VIP Dashboard mitra — port section "VIP Dashboard": status keanggotaan,
	// entitlement, rate komisi/CPA, dompet VIP (CPA, komisi transaksi, referral),
	// dan daftar domain rujukan.
	import { onMount } from 'svelte';
	import { Award, Globe } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	let loading = $state(true);
	let vip = $state<any>(null);
	let wallet = $state<any>(null);
	let domains = $state<any>(null);
	let notice = $state('');
	let error = $state('');
	let busy = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadWallet(), loadDomains()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const res = await apiRequest('/v1/partner/me/vip');
		if (res.ok) vip = await res.json();
	}
	async function loadWallet() {
		const res = await apiRequest('/v1/partner/vip/wallet');
		if (res.ok) wallet = await res.json();
	}
	async function loadDomains() {
		const res = await apiRequest('/v1/partner/vip/domains');
		if (res.ok) domains = await res.json();
	}

	async function createBrand() {
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/vip/brand', { method: 'POST' });
			const data = await res.json();
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Brand VIP tersedia. Membuka area klien…';
			window.location.href = `/klien/${data.public_id ?? data.id}`;
		} finally {
			busy = false;
		}
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const entLabel: Record<string, string> = {
		client_workspace: 'Ruang kerja klien', ai_premium: 'AI Premium',
		seo_tools: 'Alat SEO', keyword_research: 'Riset kata kunci'
	};
</script>

<PageTitle title="VIP Dashboard" />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">VIP Dashboard</h1>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Status keanggotaan, komisi, dan penghasilan tambahan mitra VIP.")}</p>
		</div>
		{#if vip?.isVip}
			<button class="app-btn app-btn-primary" disabled={busy} onclick={createBrand}>{tr("Buat / Buka Brand VIP")}</button>
		{/if}
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat data VIP…")}</p>
	{:else if vip}
		<div class="app-card mt-4 p-4">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="flex items-center gap-3">
					<Award size={22} style="color: {vip.isVip ? 'var(--primary)' : 'var(--muted-foreground)'};" />
					<div>
						<p class="text-sm font-semibold">
							{vip.isVip ? tr('Mitra VIP Aktif') : vip.hasVip ? tr('Keanggotaan Berakhir') : tr('Mitra Reguler')}
						</p>
						<p class="text-xs text-muted-foreground">Tier: {vip.tier}{#if vip.vipExpiresAt} · {tr('berlaku sampai')} {dt(vip.vipExpiresAt)}{/if}</p>
					</div>
				</div>
				<dl class="flex gap-6 text-sm">
					<div><dt class="text-xs text-muted-foreground">{tr('Komisi')}</dt><dd class="font-semibold tabular-nums">{(vip.commissionRateBp / 100).toFixed(2)}%</dd></div>
					<div><dt class="text-xs text-muted-foreground">CPA / domain</dt><dd class="font-semibold tabular-nums">{rp(vip.cpaRate)}</dd></div>
				</dl>
			</div>

			<div class="mt-3 flex flex-wrap gap-2">
				{#each Object.entries(vip.entitlements ?? {}) as [key, enabled] (key)}
					<span
						class="rounded-full px-2.5 py-0.5 text-[11px] font-medium"
						style="background-color: {enabled ? 'var(--accent)' : 'var(--secondary)'}; color: {enabled ? 'var(--accent-foreground)' : 'var(--muted-foreground)'};"
					>{entLabel[key] ?? key}{enabled ? '' : ` ${tr('nonaktif')}`}</span>
				{/each}
			</div>
		</div>

		{#if wallet}
			<div class="mt-4 grid gap-3 sm:grid-cols-4">
				<div class="app-card p-4"><p class="text-xs font-semibold tracking-wide text-muted-foreground">CPA DOMAIN</p><p class="mt-2 text-xl font-bold tabular-nums">{rp(wallet.cpa)}</p></div>
				<div class="app-card p-4"><p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr('KOMISI TRANSAKSI')}</p><p class="mt-2 text-xl font-bold tabular-nums">{rp(wallet.transaction_commission)}</p></div>
				<div class="app-card p-4"><p class="text-xs font-semibold tracking-wide text-muted-foreground">REFERRAL</p><p class="mt-2 text-xl font-bold tabular-nums">{rp(wallet.referral)}</p></div>
				<div class="app-card p-4"><p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr('PUBLIKASI')}</p><p class="mt-2 text-xl font-bold tabular-nums">{rp(wallet.publication)}</p></div>
			</div>

			{#if (wallet.history ?? []).length > 0}
				<div class="app-card mt-4 overflow-x-auto">
					<div class="border-b px-4 py-3"><h2 class="text-sm font-semibold">{tr("Riwayat Penghasilan VIP")}</h2></div>
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2 font-medium">{tr('Jenis')}</th>
								<th class="px-4 py-2 text-right font-medium">{tr('Nilai')}</th>
								<th class="px-4 py-2 font-medium">Status</th>
								<th class="px-4 py-2 font-medium">{tr("Tanggal")}</th>
							</tr>
						</thead>
						<tbody>
							{#each wallet.history as h (h.id)}
								<tr class="border-t">
									<td class="px-4 py-2">{h.kind}</td>
									<td class="px-4 py-2 text-right tabular-nums">{rp(h.amount)}</td>
									<td class="px-4 py-2 text-muted-foreground">{h.status}</td>
									<td class="px-4 py-2 text-xs text-muted-foreground">{dt(h.created_at)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		{/if}

		<div class="app-card mt-4 overflow-x-auto">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<div class="flex items-center gap-2">
					<Globe size={16} />
					<h2 class="text-sm font-semibold">{tr('Domain Rujukan')}</h2>
				</div>
				<span class="text-xs text-muted-foreground">{domains?.clicks ?? 0} {tr('klik')} · {domains?.stats?.approved ?? 0} {tr('disetujui')}</span>
			</div>
			{#if (domains?.items ?? []).length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Belum ada domain rujukan.")}</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2 font-medium">Domain</th>
							<th class="px-4 py-2 font-medium">{tr("Media")}</th>
							<th class="px-4 py-2 font-medium">Status</th>
							<th class="px-4 py-2 text-right font-medium">CPA</th>
							<th class="px-4 py-2 font-medium">{tr("Tanggal")}</th>
						</tr>
					</thead>
					<tbody>
						{#each domains.items as d (d.id)}
							<tr class="border-t">
								<td class="px-4 py-2">{d.domain_name}</td>
								<td class="px-4 py-2 text-muted-foreground">{d.media_name ?? '—'}</td>
								<td class="px-4 py-2">
									{#if d.status === 'approved'}<span style="color: var(--success);">{tr("Disetujui")}</span>
									{:else if d.status === 'rejected'}<span style="color: var(--destructive);">{tr("Ditolak")}</span>
									{:else}<span class="text-muted-foreground">{tr("Menunggu")}</span>{/if}
								</td>
								<td class="px-4 py-2 text-right tabular-nums">{d.cpa_paid ? rp(d.cpa_amount ?? 0) : '—'}</td>
								<td class="px-4 py-2 text-xs text-muted-foreground">{dt(d.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>
