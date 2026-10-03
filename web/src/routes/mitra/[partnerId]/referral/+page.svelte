<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Referral mitra: port section "Referral" (referral-section.tsx), header
	// dengan hint komisi + badge tier VIP/jumlah referral, kartu VIP (link
	// Dashboard Klien hanya untuk brand_ambassador), kartu link referral
	// mitra/klien dengan tombol salin, grid Performance, dan tabel Direct
	// Referral. Adaptasi kontrak Go API: atribusi referral hanya menyimpan
	// email, kode, dan tanggal, jadi kolom status/earning tidak ada.
	import { onMount } from 'svelte';
	import { Check, Copy, Crown, TriangleAlert } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, readApiError, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import { vipTierBadge } from '$lib/partner/vip-tier';

	type ReferralItem = {
		id: string;
		code: string;
		referee_user_id: string;
		referee_email: string | null;
		created_at: string | null;
	};
	type ReferralData = {
		link: string;
		link_mitra?: string;
		link_klien?: string;
		code: string;
		rate_bps: number;
		totalReferrals?: number;
		referral_earning: number;
		referrals: ReferralItem[];
		clicks?: { total: number; unique: number };
	};
	type VipInfo = { isVip: boolean; hasVip: boolean; tier: string };

	let loading = $state(true);
	let data = $state<ReferralData | null>(null);
	let vip = $state<VipInfo | null>(null);
	let error = $state('');
	let vipError = $state('');
	let copied = $state<'' | 'mitra' | 'klien'>('');

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
		// Status VIP non-fatal: gagal tetap menampilkan halaman + callout abu-abu.
		try {
			const vipRes = await apiRequest('/v1/partner/me/vip');
			if (vipRes.ok) vip = await vipRes.json();
			else vipError = await readApiError(vipRes, tr('Gagal memuat status VIP.'));
		} catch {
			vipError = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		}
		try {
			const res = await apiRequest('/v1/partner/referrals');
			if (res.ok) data = await res.json();
			else error = await readApiError(res, tr('Gagal memuat data referral. Coba lagi nanti.'));
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		}
	}

	async function copy(label: 'mitra' | 'klien', value: string) {
		try {
			await navigator.clipboard.writeText(value);
			copied = label;
			setTimeout(() => (copied = ''), 1600);
		} catch {
			copied = '';
		}
	}

	const idr = (n: number) => 'Rp' + Number(n || 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' });
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '-');

	const ratePct = $derived(((data?.rate_bps ?? 100) / 100).toFixed(1));
	const isVip = $derived(Boolean(vip?.hasVip ?? vip?.isVip));
	const tierLabel = $derived(
		vip?.tier === 'brand_ambassador' ? 'VVIP' : vip?.tier === 'vip_reguler' ? 'VIP Reguler' : 'VIP'
	);
	const totalReferrals = $derived(data?.totalReferrals ?? (data?.referrals?.length ?? 0));
	const linkMitra = $derived(data?.link_mitra ?? data?.link ?? '');
	const linkKlien = $derived(data?.link_klien ?? '');
	const hint = $derived(
		`${tr('Komisi Mitra')} ${ratePct}% · ${tr('Komisi Klien')} 5% - ${tr('hanya 1 level.')}`
	);
	const linkDesc = $derived(
		`${tr('Bagikan link sesuai pendaftar: Mitra → calon mitra, Klien → calon klien. Pendaftar via link menjadi direct referral Anda. Komisi')} ${ratePct}% ${tr('× earning mitra (settled) · Komisi klien 5% (segera aktif).')}`
	);
</script>

<PageTitle title={tr('Referral')} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr('Referral')}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{hint}</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			{#if isVip}
				{@const b = vipTierBadge(vip?.tier)}
				<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold shadow-xs" style="background-color: {b.bg}; color: {b.fg}; border: 1px solid {b.border};">
					<Crown size={12} /> {tierLabel}
				</span>
			{:else}
				<span class="rounded-full px-2.5 py-1 text-xs font-medium text-muted-foreground" style="background-color: var(--secondary);">{tr('Regular')}</span>
			{/if}
			<span class="rounded-full px-2.5 py-1 text-xs font-medium" style="background-color: {data ? 'var(--success-soft)' : 'var(--secondary)'}; color: {data ? 'var(--success)' : 'var(--muted-foreground)'};">{data ? `${totalReferrals} referral` : '-'}</span>
		</div>
	</header>

	{#if vipError}
		<div class="app-card mt-3 px-4 py-2.5 text-sm text-muted-foreground">{vipError}</div>
	{/if}

	{#if isVip}
		{@const b = vipTierBadge(vip?.tier)}
		<div class="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3.5" style="border-color: {b.border}; background-color: {b.bg};">
			<div>
				<div class="flex items-center gap-2">
					<Crown size={16} style="color: {b.fg};" />
					<span class="font-bold" style="color: {b.fg};">{tr('VIP Mitra')}</span>
					<span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" style="background-color: {b.fg}; color: #ffffff;">{tierLabel}</span>
				</div>
				<p class="mt-1 text-sm text-muted-foreground">
					{vip?.tier === 'vip_reguler'
						? tr('Fee affiliate Rp10.000/domain untuk 1000 domain pertama, lalu Rp5.000/domain.')
						: tr('Akses Client Workspace & AI premium aktif.')}
				</p>
			</div>
			<!-- Hanya VVIP (brand_ambassador) yang boleh masuk Client Workspace.
				VIP Reguler murni fee affiliate, jadi tombolnya tidak ditampilkan. -->
			{#if vip?.tier === 'brand_ambassador'}
				<a class="app-btn app-btn-primary shrink-0" href="/klien">{tr('Dashboard Klien')} →</a>
			{/if}
		</div>
	{/if}

	{#if error}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-3 text-sm" style="border-color: var(--destructive); color: var(--destructive);">
			<TriangleAlert size={16} class="mt-0.5 shrink-0" />
			<p>{error}</p>
		</div>
	{/if}

	{#if loading && !data && !error}
		<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat referral…')}</p>
	{/if}

	{#if data}
		<div class="app-card mt-3 p-4">
			<h2 class="text-sm font-semibold">{tr('Link Referral Anda')}</h2>
			<p class="mb-3 mt-1 text-sm text-muted-foreground">{linkDesc}</p>
			<div class="mb-2 flex flex-wrap items-center gap-2">
				<span class="w-[110px] shrink-0 text-xs text-muted-foreground">{tr('Pendaftar Mitra')}</span>
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground" style="background-color: var(--secondary);">{tr('Komisi')} {ratePct}%</span>
				<input class="app-input min-w-[240px] flex-1 font-mono text-xs" readonly value={linkMitra} />
				<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('mitra', linkMitra)}>
					{#if copied === 'mitra'}<Check size={14} /> {tr('Tersalin')}{:else}<Copy size={14} /> {tr('Salin')}{/if}
				</button>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<span class="w-[110px] shrink-0 text-xs text-muted-foreground">{tr('Pendaftar Klien')}</span>
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{tr('Komisi')} 5%</span>
				<input class="app-input min-w-[240px] flex-1 font-mono text-xs" readonly value={linkKlien} />
				<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('klien', linkKlien)}>
					{#if copied === 'klien'}<Check size={14} /> {tr('Tersalin')}{:else}<Copy size={14} /> {tr('Salin')}{/if}
				</button>
			</div>
			<div class="mt-3 flex flex-wrap items-center gap-2">
				<span class="text-xs text-muted-foreground">{tr('Kode referral:')}</span>
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{data.code}</span>
			</div>
		</div>

		<div class="app-card mt-3 p-4">
			<h2 class="mb-2 text-sm font-semibold">{tr('Performance')}</h2>
			<div class="grid gap-3" style="grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));">
				<div>
					<p class="text-xs text-muted-foreground">{tr('Total Klik')}</p>
					<p class="text-3xl font-bold tabular-nums">{data.clicks?.total ?? 0}</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">{tr('Total Referral')}</p>
					<p class="text-3xl font-bold tabular-nums">{totalReferrals}</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">{tr('Komisi Referral')}</p>
					<p class="text-3xl font-bold tabular-nums" style="color: var(--success);">{idr(data.referral_earning)}</p>
				</div>
			</div>
		</div>

		<div class="app-card mt-3 overflow-x-auto">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
				<h2 class="text-sm font-semibold">{tr('Direct Referral')}</h2>
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground" style="background-color: var(--secondary);">{data.referrals.length} {tr('akun')}</span>
			</div>
			{#if data.referrals.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr('Belum ada referral. Bagikan link di atas.')}</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">{tr('Email')}</th>
							<th class="px-4 py-2.5 font-medium">{tr('Kode')}</th>
							<th class="px-4 py-2.5 font-medium">{tr('Bergabung')}</th>
						</tr>
					</thead>
					<tbody>
						{#each data.referrals as r (r.id)}
							<tr class="border-t">
								<td class="px-4 py-2.5">{r.referee_email ?? '-'}</td>
								<td class="px-4 py-2.5 font-mono text-xs text-muted-foreground">{r.code}</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(r.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>
