<script lang="ts">
	// Monitoring VIP — port app/admin/vip-partners/page.tsx: monitoring mitra
	// VVIP (Brand Ambassador) & VIP Reguler: kontrak, domain referral yang join,
	// domain yang dapat orderan, kuota tier, rate, fee dibayar, dan komisi.
	// Drill-down per mitra menampilkan daftar domain referral-nya.
	import { onMount } from 'svelte';
	import { Crown, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type VipPartner = {
		partner_id: string; name: string | null; email: string | null; tier: string;
		is_active: boolean; vip_enabled_at: string | null; vip_expires_at: string | null;
		days_remaining: number | null; contract_days: number | null; contract_months: number | null;
		commission_rate_bp: number; cpa_rate: number; cpa_tier_threshold: number;
		cpa_tier1_rate: number; cpa_tier2_rate: number;
		domains_total: number; domains_approved: number; domains_paid: number;
		domains_with_orders: number; orders_published_total: number; gmv_published_total: number;
		quota_used: number; fee_paid_total: number; tx_commission_total: number;
		referral_commission_total: number;
	};
	type DomainDetail = {
		id: string; domain_name: string; owner_email: string | null; owner_name: string | null;
		media_name: string | null; media_website_url: string | null; status: string;
		rejection_reason: string | null; cpa_paid: boolean; cpa_amount: number | null;
		orders_total: number; orders_published: number; gmv_published: number;
		created_at: string | null; last_order_at: string | null;
	};
	type Summary = {
		total: number; active: number; vvip: number; vip_reguler: number;
		fee_paid_total: number; commission_total: number; domains_total: number;
		domains_with_orders: number; orders_published_total: number; gmv_published_total: number;
	};

	const DOMAIN_PAGE_SIZE = 20;
	const REFERRAL_STATUS_LABEL: Record<string, string> = {
		pending: 'Menunggu', approved: 'Disetujui', rejected: 'Ditolak'
	};
	const REFERRAL_STATUS_TONE: Record<string, 'warning' | 'success' | 'danger' | 'neutral'> = {
		pending: 'warning', approved: 'success', rejected: 'danger'
	};

	let items = $state<VipPartner[]>([]);
	let summary = $state<Summary | null>(null);
	let error = $state('');
	let loaded = $state(false);

	let detailTarget = $state<VipPartner | null>(null);
	let domains = $state<DomainDetail[]>([]);
	let domainsTotal = $state(0);
	let domainsOffset = $state(0);
	let domainsLoading = $state(false);
	let domainsError = $state('');
	let orderFilter = $state<'' | 'with' | 'without'>('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loaded = true;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const res = await apiRequest('/v1/platform/vip-partners');
		if (res.ok) {
			const data = await res.json();
			items = data.items ?? [];
			summary = data.summary ?? null;
		} else {
			error = await platformError(res);
		}
	}

	async function loadDomains(partnerId: string, offset: number, filter: '' | 'with' | 'without') {
		domainsLoading = true;
		domainsError = '';
		const params = new URLSearchParams({ limit: String(DOMAIN_PAGE_SIZE), offset: String(offset) });
		if (filter) params.set('orders', filter);
		const res = await apiRequest(`/v1/platform/vip-partners/${partnerId}/domains?${params.toString()}`);
		if (res.ok) {
			const data = await res.json();
			domains = data.domains ?? [];
			domainsTotal = data.total ?? 0;
		} else {
			domainsError = await platformError(res);
		}
		domainsLoading = false;
	}

	function openDetail(p: VipPartner) {
		detailTarget = p;
		domainsOffset = 0;
		orderFilter = '';
		domains = [];
		domainsTotal = 0;
		void loadDomains(p.partner_id, 0, '');
	}

	const tierLabel = (tier: string) =>
		tier === 'brand_ambassador' ? 'VVIP' : tier === 'vip_reguler' ? 'VIP Reguler' : 'VIP';
	const hasAksesError = $derived(Boolean(items.length === 0 && error.includes('Akses platform')));
	const detailCurrentPage = $derived(Math.floor(domainsOffset / DOMAIN_PAGE_SIZE) + 1);
	const detailTotalPages = $derived(Math.max(1, Math.ceil(domainsTotal / DOMAIN_PAGE_SIZE)));
</script>

<section>
	<PageHeader
		title="Monitoring VIP"
		description="Tracking & monitoring mitra VVIP dan VIP Reguler: domain referral yang join, domain yang dapat orderan, kuota, kontrak, dan total pencairan."
	/>

	<div class="app-card mt-3 flex items-start gap-2 px-4 py-3 text-xs" style="color: var(--warning);">
		<TriangleAlert size={16} class="mt-0.5 shrink-0" />
		<span><strong>Beda konsep:</strong> VVIP (Brand Ambassador) dapat komisi transaksi lebih besar + akses Client Workspace. VIP Reguler hanya fee per-domain bertingkat — tanpa akses Client Workspace.</span>
	</div>

	{#if !loaded}
		<p class="mt-6 text-sm text-muted-foreground">Memuat monitoring VIP…</p>
	{:else if hasAksesError}
		<div class="app-card mt-3 px-4 py-3 text-sm text-muted-foreground">Akun ini tidak memiliki akses ke monitoring VIP.</div>
	{:else}
		{#if error}
			<div class="app-card mt-3 px-4 py-3 text-sm" style="color: var(--destructive);">{error}</div>
		{/if}

		<div class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">Total Mitra VIP</p>
				<p class="mt-1 text-2xl font-bold">{fmtNum(summary?.total ?? 0)}</p>
				<p class="text-xs text-muted-foreground">{fmtNum(summary?.active ?? 0)} aktif</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">VVIP</p>
				<p class="mt-1 text-2xl font-bold" style="color: var(--primary);">{fmtNum(summary?.vvip ?? 0)}</p>
				<p class="text-xs text-muted-foreground">Brand Ambassador</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">VIP Reguler</p>
				<p class="mt-1 text-2xl font-bold">{fmtNum(summary?.vip_reguler ?? 0)}</p>
				<p class="text-xs text-muted-foreground">Fee per-domain</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">Domain Join</p>
				<p class="mt-1 text-2xl font-bold">{fmtNum(summary?.domains_total ?? 0)}</p>
				<p class="text-xs text-muted-foreground">domain referral masuk</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">Domain Dapat Orderan</p>
				<p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtNum(summary?.domains_with_orders ?? 0)}</p>
				<p class="text-xs text-muted-foreground">{fmtNum(summary?.orders_published_total ?? 0)} order terbit</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">GMV Order Terbit</p>
				<p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtRp(summary?.gmv_published_total ?? 0)}</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">Total Fee Domain</p>
				<p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtRp(summary?.fee_paid_total ?? 0)}</p>
			</div>
			<div class="app-card min-h-[92px] p-4">
				<p class="text-xs text-muted-foreground">Total Komisi</p>
				<p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtRp(summary?.commission_total ?? 0)}</p>
			</div>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			{#if items.length === 0}
				<p class="px-4 py-4 text-sm text-muted-foreground">Belum ada mitra VVIP atau VIP Reguler.</p>
			{:else}
				<p class="px-4 pt-3 text-xs text-muted-foreground">
					Klik baris mitra untuk melihat daftar domain referral-nya (domain ini referal dari akun siapa &amp; sudah dapat orderan atau belum).
				</p>
				<table class="mt-2 w-full text-sm" style="min-width: 1320px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">Mitra</th>
							<th class="px-4 py-2.5 font-medium">Tier</th>
							<th class="px-4 py-2.5 font-medium">Status Kontrak</th>
							<th class="px-4 py-2.5 font-medium">Domain Join</th>
							<th class="px-4 py-2.5 font-medium">Domain Order</th>
							<th class="px-4 py-2.5 font-medium">Kuota Tier</th>
							<th class="px-4 py-2.5 font-medium">Rate</th>
							<th class="px-4 py-2.5 font-medium">Fee Dibayar</th>
							<th class="px-4 py-2.5 font-medium">Komisi</th>
							<th class="px-4 py-2.5"></th>
						</tr>
					</thead>
					<tbody>
						{#each items as p (p.partner_id)}
							{@const isReguler = p.tier === 'vip_reguler'}
							{@const quotaPct = isReguler && p.cpa_tier_threshold > 0 ? Math.min(100, Math.round((p.quota_used / p.cpa_tier_threshold) * 100)) : null}
							<tr class="cursor-pointer border-t hover:bg-secondary" onclick={() => openDetail(p)}>
								<td class="px-4 py-2.5">
									<p class="font-medium">{p.name ?? '—'}</p>
									<p class="text-xs text-muted-foreground">{p.email ?? '—'}</p>
								</td>
								<td class="px-4 py-2.5">
									<span
										class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
										style="background-color: var(--accent); color: var(--accent-foreground);"
									>
										{#if !isReguler}<Crown size={11} />{/if} {tierLabel(p.tier)}
									</span>
								</td>
								<td class="px-4 py-2.5">
									{#if p.is_active}
										<Badge label="Aktif" tone="success" />
										<p class="mt-1 text-xs text-muted-foreground">
											{p.days_remaining != null
												? `${Math.max(0, p.days_remaining)} hari lagi`
												: p.contract_months != null
													? `${p.contract_months} bulan`
													: 'Tanpa batas'}
										</p>
										{#if p.vip_expires_at}<p class="text-xs text-muted-foreground">s/d {fmtDate(p.vip_expires_at)}</p>{/if}
									{:else}
										<Badge label="Berakhir" tone="neutral" />
										{#if p.vip_expires_at}<p class="mt-1 text-xs text-muted-foreground">s/d {fmtDate(p.vip_expires_at)}</p>{/if}
									{/if}
								</td>
								<td class="px-4 py-2.5">
									<p class="font-medium">{fmtNum(p.domains_total)}</p>
									<p class="text-xs text-muted-foreground">{fmtNum(p.domains_approved)} disetujui</p>
									<p class="text-xs text-muted-foreground">{fmtNum(p.domains_paid)} fee dibayar</p>
								</td>
								<td class="px-4 py-2.5">
									<p class="font-medium" style={p.domains_with_orders > 0 ? 'color: var(--success);' : undefined}>{fmtNum(p.domains_with_orders)} dapat orderan</p>
									<p class="text-xs text-muted-foreground">{fmtNum(p.orders_published_total)} order terbit</p>
									<p class="text-xs text-muted-foreground">GMV {fmtRp(p.gmv_published_total)}</p>
								</td>
								<td class="px-4 py-2.5">
									{#if isReguler}
										<p class="text-sm">{fmtNum(p.quota_used)} / {fmtNum(p.cpa_tier_threshold)}</p>
										{#if quotaPct != null}
											<p class="mt-1 text-xs" style="color: {quotaPct >= 100 ? 'var(--warning)' : 'var(--muted-foreground)'};">
												{quotaPct >= 100 ? 'Kuota habis → rate tier 2' : `${quotaPct}% terpakai`}
											</p>
										{/if}
									{:else}
										<span class="text-xs text-muted-foreground">Tidak berlaku</span>
									{/if}
								</td>
								<td class="px-4 py-2.5">
									{#if isReguler}
										<p class="text-xs" style="color: var(--success);">{fmtRp(p.cpa_tier1_rate)} → {fmtRp(p.cpa_tier2_rate)}</p>
										<p class="mt-1 text-xs text-muted-foreground">per domain</p>
									{:else}
										<p class="text-xs" style="color: var(--success);">{fmtRp(p.cpa_rate)}/domain</p>
										<p class="mt-1 text-xs text-muted-foreground">komisi {(p.commission_rate_bp / 100).toFixed(p.commission_rate_bp % 100 === 0 ? 0 : 1)}%</p>
									{/if}
								</td>
								<td class="px-4 py-2.5 font-medium" style="color: var(--success);">{fmtRp(p.fee_paid_total)}</td>
								<td class="px-4 py-2.5">
									<p class="text-sm">{fmtRp(p.tx_commission_total + p.referral_commission_total)}</p>
									<p class="text-xs text-muted-foreground">
										{fmtRp(p.referral_commission_total)} referral{p.tx_commission_total > 0 ? ` + ${fmtRp(p.tx_commission_total)} transaksi` : ''}
									</p>
								</td>
								<td class="px-4 py-2.5 text-muted-foreground">›</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>

{#if detailTarget}
	<Modal
		title={`Domain Referral — ${detailTarget.name ?? detailTarget.email ?? 'Mitra'}`}
		label="Domain referral mitra VIP"
		maxWidth="2xl"
		onClose={() => { detailTarget = null; domains = []; domainsError = ''; }}
	>
		<p class="text-xs text-muted-foreground">
			Daftar domain yang direferensikan akun ini, lengkap pemilik, media, jumlah order terbit, dan fee yang sudah dibayar.
		</p>

		<div class="mt-3 flex flex-wrap gap-2">
			{#each [['', 'Semua Domain'], ['with', 'Sudah Dapat Orderan'], ['without', 'Belum Ada Orderan']] as [key, label] (key || 'all')}
				<button
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={orderFilter === key}
					class:app-btn-soft={orderFilter !== key}
					type="button"
					onclick={() => { orderFilter = key as '' | 'with' | 'without'; domainsOffset = 0; void loadDomains(detailTarget!.partner_id, 0, orderFilter); }}
				>{label}</button>
			{/each}
		</div>

		{#if domainsError}
			<div class="mt-3 text-sm" style="color: var(--destructive);">{domainsError}</div>
		{/if}

		{#if domainsLoading}
			<p class="mt-3 text-sm text-muted-foreground">Memuat domain…</p>
		{:else if domains.length === 0}
			<p class="mt-3 text-sm text-muted-foreground">
				{orderFilter === 'with'
					? 'Belum ada domain yang mendapat orderan terbit.'
					: orderFilter === 'without'
						? 'Semua domain sudah mendapat orderan terbit.'
						: 'Belum ada domain referral untuk mitra ini.'}
			</p>
		{:else}
			<div class="mt-3 max-h-[420px] overflow-auto">
				<table class="w-full text-sm" style="min-width: 1080px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="py-2 pr-3 font-medium">Domain</th>
							<th class="py-2 pr-3 font-medium">Pemilik (Akun)</th>
							<th class="py-2 pr-3 font-medium">Media</th>
							<th class="py-2 pr-3 font-medium">Status Referal</th>
							<th class="py-2 pr-3 font-medium">Order</th>
							<th class="py-2 pr-3 font-medium">Fee</th>
							<th class="py-2 pr-3 font-medium">Join</th>
							<th class="py-2 font-medium">Order Terakhir</th>
						</tr>
					</thead>
					<tbody>
						{#each domains as d (d.id)}
							<tr class="border-t">
								<td class="py-2 pr-3 font-medium">{d.domain_name}</td>
								<td class="py-2 pr-3">
									<p class="text-sm">{d.owner_name ?? '—'}</p>
									<p class="text-xs text-muted-foreground">{d.owner_email ?? '—'}</p>
								</td>
								<td class="py-2 pr-3">
									<p class="text-sm">{d.media_name ?? '—'}</p>
									{#if d.media_website_url}
										<p class="max-w-[180px] break-all text-xs text-muted-foreground">{d.media_website_url}</p>
									{/if}
								</td>
								<td class="py-2 pr-3">
									<Badge label={REFERRAL_STATUS_LABEL[d.status] ?? d.status} tone={REFERRAL_STATUS_TONE[d.status] ?? 'neutral'} />
									{#if d.status === 'rejected' && d.rejection_reason}
										<p class="mt-1 max-w-[180px] text-xs text-muted-foreground">{d.rejection_reason}</p>
									{/if}
								</td>
								<td class="py-2 pr-3">
									{#if d.orders_published > 0}
										<p class="text-sm font-medium" style="color: var(--success);">{fmtNum(d.orders_published)} terbit</p>
										<p class="text-xs text-muted-foreground">dari {fmtNum(d.orders_total)} order</p>
										<p class="text-xs text-muted-foreground">GMV {fmtRp(d.gmv_published)}</p>
									{:else}
										<p class="text-sm text-muted-foreground">Belum ada</p>
										{#if d.orders_total > 0}<p class="text-xs text-muted-foreground">{fmtNum(d.orders_total)} order belum terbit</p>{/if}
									{/if}
								</td>
								<td class="py-2 pr-3">
									{#if d.cpa_paid}
										<Badge label="Dibayar" tone="success" />
										{#if d.cpa_amount != null}<p class="mt-1 text-xs text-muted-foreground">{fmtRp(d.cpa_amount)}</p>{/if}
									{:else}
										<Badge label="Belum dibayar" tone="neutral" />
									{/if}
								</td>
								<td class="py-2 pr-3 text-xs text-muted-foreground">{d.created_at ? fmtDate(d.created_at) : '—'}</td>
								<td class="py-2 text-xs text-muted-foreground">{d.last_order_at ? fmtDate(d.last_order_at) : '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
				<span>Menampilkan {domainsOffset + 1}–{Math.min(domainsOffset + DOMAIN_PAGE_SIZE, domainsTotal)} dari {fmtNum(domainsTotal)} domain</span>
				<div class="flex items-center gap-2">
					<button
						class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
						type="button"
						disabled={detailCurrentPage <= 1 || domainsLoading}
						onclick={() => { const next = Math.max(0, domainsOffset - DOMAIN_PAGE_SIZE); domainsOffset = next; void loadDomains(detailTarget!.partner_id, next, orderFilter); }}
					>‹ Sebelumnya</button>
					<span>{detailCurrentPage} / {detailTotalPages}</span>
					<button
						class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
						type="button"
						disabled={detailCurrentPage >= detailTotalPages || domainsLoading}
						onclick={() => { const next = domainsOffset + DOMAIN_PAGE_SIZE; domainsOffset = next; void loadDomains(detailTarget!.partner_id, next, orderFilter); }}
					>Berikutnya ›</button>
				</div>
			</div>
		{/if}

		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => { detailTarget = null; domains = []; domainsError = ''; }}>Tutup</button>
		{/snippet}
	</Modal>
{/if}
