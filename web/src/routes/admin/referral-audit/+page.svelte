<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Monitoring non-VIP & audit referral — port app/admin/referral-audit/page.tsx:
	// funnel referral mitra non-VIP (klik, pendaftaran, domain join, domain dapat
	// orderan, GMV, komisi) + tab Domain Referral (approve/reject) + tab
	// Pendaftaran (attribution). Halaman /admin/referred-domains lama me-redirect
	// ke sini, jadi route itu dibuat sebagai halaman redirect.
	import { onMount } from 'svelte';
	import { Globe, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtDateTime, fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type Referrer = {
		partner_id: string; name: string; email: string; referral_code: string | null;
		vip_tier: string | null; is_vip: boolean; clicks: number; signups: number;
		mitra_count: number; klien_count: number; commission_total: number;
		domains_joined: number; domains_with_orders: number;
		orders_published_total: number; gmv_published_total: number;
	};
	type Attribution = {
		id: string; code: string; referrer_name: string; referrer_partner_id: string;
		referee_email: string | null; referee_name: string | null; referee_role: string | null;
		ip: string | null; fraud_note: string | null; created_at: string;
	};
	type DomainRow = {
		id: string; domain_name: string; status: 'pending' | 'approved' | 'rejected';
		rejection_reason: string | null; is_tld: boolean; cpa_paid: boolean;
		cpa_amount: number | null; created_at: string; referrer_name: string | null;
		referrer_partner_id: string | null; referrer_vip_tier: string | null;
		referrer_is_vip: boolean | null; owner_email: string | null; owner_name: string | null;
		media_name: string | null; media_website_url: string | null;
		orders_total: number; orders_published: number; gmv_published: number;
		last_order_at: string | null;
	};

	const ROLE_LABEL: Record<string, string> = $derived({ partner: tr('Mitra'), client: tr('Klien'), admin: 'Admin' });
	const STATUS_LABEL: Record<string, string> = $derived({ pending: tr('Menunggu'), approved: tr('Disetujui'), rejected: tr('Ditolak') });
	const STATUS_TONE: Record<string, 'warning' | 'success' | 'danger'> = {
		pending: 'warning', approved: 'success', rejected: 'danger'
	};

	let referrers = $state<Referrer[]>([]);
	let attributions = $state<Attribution[]>([]);
	let domains = $state<DomainRow[]>([]);
	let domainsTotal = $state(0);
	let error = $state('');
	let loaded = $state(false);
	let tab = $state<'referrers' | 'domains' | 'attributions'>('referrers');

	let orderFilter = $state<'' | 'with' | 'without'>('');
	let domainStatus = $state<'' | 'pending' | 'approved' | 'rejected'>('');
	let tierFilter = $state<'' | 'vip' | 'nonvip'>('');

	let busy = $state(false);
	let rejecting = $state<DomainRow | null>(null);
	let rejectionReason = $state('');
	let rejectionError = $state('');

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
		const params = new URLSearchParams();
		if (orderFilter) params.set('orders', orderFilter);
		if (domainStatus) params.set('domain_status', domainStatus);
		const res = await apiRequest(`/v1/platform/referral-audit${params.toString() ? `?${params.toString()}` : ''}`);
		if (res.ok) {
			const data = await res.json();
			referrers = data.referrers ?? [];
			attributions = data.attributions ?? [];
			domains = data.domains ?? [];
			domainsTotal = data.domains_total ?? 0;
		} else {
			error = await platformError(res);
		}
	}

	function setOrderFilter(key: '' | 'with' | 'without') {
		orderFilter = key;
		void load();
	}

	function setDomainStatus(key: '' | 'pending' | 'approved' | 'rejected') {
		domainStatus = key;
		void load();
	}

	async function approve(rd: DomainRow) {
		busy = true;
		error = '';
		const res = await apiRequest('/v1/platform/referred-domains', {
			method: 'PUT',
			body: JSON.stringify({ id: rd.id, status: 'approved' })
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		busy = false;
		await load();
	}

	async function reject() {
		if (!rejecting) return;
		if (rejectionReason.trim().length < 5) {
			rejectionError = 'Alasan penolakan minimal 5 karakter.';
			return;
		}
		busy = true;
		rejectionError = '';
		const res = await apiRequest('/v1/platform/referred-domains', {
			method: 'PUT',
			body: JSON.stringify({ id: rejecting.id, status: 'rejected', rejection_reason: rejectionReason.trim() })
		});
		if (!res.ok) {
			rejectionError = await platformError(res);
			busy = false;
			return;
		}
		rejecting = null;
		rejectionReason = '';
		busy = false;
		await load();
	}

	function referrerTierLabel(tier: string | null, isVip: boolean | null) {
		if (!isVip) return 'Non VIP';
		if (tier === 'brand_ambassador') return 'VVIP';
		if (tier === 'vip_reguler') return 'VIP Reguler';
		return 'VIP';
	}

	const visibleDomains = $derived(
		tierFilter === ''
			? domains
			: domains.filter((d) => (tierFilter === 'vip' ? Boolean(d.referrer_is_vip) : !d.referrer_is_vip))
	);

	const totalReferrers = $derived(referrers.length);
	const totalClicks = $derived(referrers.reduce((s, r) => s + Number(r.clicks || 0), 0));
	const totalSignups = $derived(referrers.reduce((s, r) => s + Number(r.signups || 0), 0));
	const totalKomisi = $derived(referrers.reduce((s, r) => s + Number(r.commission_total || 0), 0));
	const totalDomainsJoined = $derived(referrers.reduce((s, r) => s + Number(r.domains_joined || 0), 0));
	const totalDomainsWithOrders = $derived(referrers.reduce((s, r) => s + Number(r.domains_with_orders || 0), 0));
	const totalGmv = $derived(referrers.reduce((s, r) => s + Number(r.gmv_published_total || 0), 0));
</script>

<section>
	<PageHeader
		title={tr('Monitoring Non VIP')}
		description={tr('Referral mitra non-VIP: klik, pendaftaran, domain yang ikut, dan order yang terbit.')}
	/>

	<Feedback loading={!loaded} {error} loadingText="Memuat monitoring non VIP…" />

	{#if loaded}
		<!-- Funnel: satu deret angka, bukan tujuh kartu terpisah -->
		<div class="funnel mt-5">
			<div class="funnel-step">
				<p class="funnel-label">Referrer</p>
				<p class="funnel-value" data-zero={totalReferrers === 0}>{fmtNum(totalReferrers)}</p>
			</div>
			<div class="funnel-step">
				<p class="funnel-label">{tr('Klik')}</p>
				<p class="funnel-value" data-zero={totalClicks === 0}>{fmtNum(totalClicks)}</p>
			</div>
			<div class="funnel-step">
				<p class="funnel-label">{tr('Pendaftaran')}</p>
				<p class="funnel-value" data-zero={totalSignups === 0}>{fmtNum(totalSignups)}</p>
			</div>
			<div class="funnel-step">
				<p class="funnel-label">{tr('Domain join')}</p>
				<p class="funnel-value" data-zero={totalDomainsJoined === 0}>{fmtNum(totalDomainsJoined)}</p>
			</div>
			<div class="funnel-step">
				<p class="funnel-label">{tr('Domain order')}</p>
				<p class="funnel-value" data-zero={totalDomainsWithOrders === 0}>{fmtNum(totalDomainsWithOrders)}</p>
			</div>
			<div class="funnel-step">
				<p class="funnel-label">{tr('GMV terbit')}</p>
				<p class="funnel-value" data-zero={totalGmv === 0}>{fmtRp(totalGmv)}</p>
			</div>
			<div class="funnel-step">
				<p class="funnel-label">{tr('Komisi')}</p>
				<p class="funnel-value" data-zero={totalKomisi === 0}>{fmtRp(totalKomisi)}</p>
			</div>
		</div>

		<div class="mt-6 flex gap-1" role="tablist">
			{#each [
				['referrers', tr('Per referrer'), referrers.length],
				['domains', tr('Domain referral'), domainsTotal],
				['attributions', tr('Pendaftaran'), attributions.length]
			] as [key, label, count] (key)}
				<button
					class="tab"
					data-active={tab === key}
					role="tab"
					aria-selected={tab === key}
					type="button"
					onclick={() => (tab = key as typeof tab)}
				>
					{label}
					<span class="tab-count tabular-nums">{count}</span>
				</button>
			{/each}
		</div>

		{#if tab === 'referrers'}
			<div class="app-card mt-3 overflow-x-auto">
				{#if referrers.length === 0}
					<p class="px-4 py-10 text-sm text-muted-foreground">{tr("Belum ada referrer non-VIP.")}</p>
				{:else}
					<table class="data">
						<thead>
							<tr>
								<th class="c-ref">{tr('Referrer')}</th>
								<th class="c-code">{tr('Kode')}</th>
								<th class="c-n">{tr('Klik')}</th>
								<th class="c-n">{tr('Daftar')}</th>
								<th class="c-n">{tr('Mitra')}</th>
								<th class="c-n">{tr('Klien')}</th>
								<th class="c-n">{tr('Domain')}</th>
								<th class="c-order">{tr('Order domain')}</th>
								<th class="c-n c-komisi">{tr('Komisi')}</th>
							</tr>
						</thead>
						<tbody>
							{#each referrers as r (r.partner_id)}
								<tr>
									<td class="c-ref">
										<span class="block truncate text-sm font-medium">{r.name}</span>
										<span class="block truncate text-xs text-muted-foreground">{r.email}</span>
									</td>
									<td class="c-code">
										{#if r.referral_code}
											<span class="code-chip">{r.referral_code}</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="c-n tabular-nums" data-dim={Number(r.clicks || 0) === 0}>{fmtNum(r.clicks)}</td>
									<td class="c-n tabular-nums" data-dim={Number(r.signups || 0) === 0}>{fmtNum(r.signups)}</td>
									<td class="c-n tabular-nums" data-dim={Number(r.mitra_count || 0) === 0}>{fmtNum(r.mitra_count)}</td>
									<td class="c-n tabular-nums" data-dim={Number(r.klien_count || 0) === 0}>{fmtNum(r.klien_count)}</td>
									<td class="c-n tabular-nums" data-dim={Number(r.domains_joined || 0) === 0}>{fmtNum(r.domains_joined)}</td>
									<td class="c-order">
										<span class="ord-line">
											<span class="ord-strong" data-on={Number(r.domains_with_orders || 0) > 0}>{fmtNum(r.domains_with_orders)}</span>
											<span class="text-xs text-muted-foreground">{tr('domain')}</span>
											<span class="text-xs text-muted-foreground" aria-hidden="true">·</span>
											<span class="text-xs text-muted-foreground tabular-nums">{fmtNum(r.orders_published_total)} {tr('terbit')}</span>
										</span>
										<span class="block text-right text-xs text-muted-foreground tabular-nums">{fmtRp(r.gmv_published_total)}</span>
									</td>
									<td class="c-n c-komisi tabular-nums" style="color: var(--success);">{fmtRp(r.commission_total)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{:else if tab === 'domains'}
			<div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
				<div class="filter-group">
					<span class="filter-label">{tr('Order')}</span>
					{#each [['', tr('Semua')], ['with', tr('Ada orderan')], ['without', tr('Belum ada')]] as [key, label] (key || 'all')}
						<button class="filter-chip" data-active={orderFilter === key} type="button" onclick={() => setOrderFilter(key as '' | 'with' | 'without')}>{label}</button>
					{/each}
				</div>
				<div class="filter-group">
					<span class="filter-label">{tr('Tier')}</span>
					{#each [['', tr('Semua')], ['vip', 'VIP'], ['nonvip', 'Non VIP']] as [key, label] (key || 'alltier')}
						<button class="filter-chip" data-active={tierFilter === key} type="button" onclick={() => (tierFilter = key as '' | 'vip' | 'nonvip')}>{label}</button>
					{/each}
				</div>
				<div class="filter-group">
					<span class="filter-label">{tr('Status')}</span>
					{#each [['', tr('Semua')], ['pending', tr('Menunggu')], ['approved', tr('Disetujui')], ['rejected', tr('Ditolak')]] as [key, label] (key || 'allstatus')}
						<button class="filter-chip" data-active={domainStatus === key} type="button" onclick={() => setDomainStatus(key as '' | 'pending' | 'approved' | 'rejected')}>{label}</button>
					{/each}
				</div>
			</div>

			<div class="app-card mt-3 overflow-x-auto">
				{#if visibleDomains.length === 0}
					<div class="flex flex-col items-center px-4 py-12 text-center">
						<Globe size={18} strokeWidth={1.5} class="text-muted-foreground" />
						<p class="mt-2 text-sm text-muted-foreground">{tr("Belum ada domain referral yang cocok dengan filter.")}</p>
					</div>
				{:else}
					<table class="data">
						<thead>
							<tr>
								<th class="c-dom">{tr('Domain')}</th>
								<th class="c-ref2">{tr('Referrer')}</th>
								<th class="c-tier">{tr('Tier')}</th>
								<th class="c-owner">{tr('Pemilik')}</th>
								<th class="c-order">{tr('Order')}</th>
								<th class="c-fee">{tr('Fee')}</th>
								<th class="c-status">{tr('Status')}</th>
								<th class="c-join">{tr('Join')}</th>
								<th class="c-act"></th>
							</tr>
						</thead>
						<tbody>
							{#each visibleDomains as rd (rd.id)}
								<tr>
									<td class="c-dom">
										<span class="block truncate text-sm font-medium">{rd.domain_name}</span>
										<span class="block text-xs text-muted-foreground">{rd.is_tld ? 'TLD' : 'Subdomain'}</span>
									</td>
									<td class="c-ref2">
										<span class="block truncate text-sm">{rd.referrer_name ?? '—'}</span>
										<span class="block text-xs text-muted-foreground">{rd.referrer_partner_id ? `ID ${rd.referrer_partner_id.slice(0, 8)}` : '—'}</span>
									</td>
									<td class="c-tier">
										{#if rd.referrer_is_vip}
											<span class="tier-chip" data-vip="true">{referrerTierLabel(rd.referrer_vip_tier, rd.referrer_is_vip)}</span>
										{:else}
											<span class="text-xs text-muted-foreground">Non VIP</span>
										{/if}
									</td>
									<td class="c-owner">
										<span class="block truncate text-sm">{rd.owner_name ?? '—'}</span>
										<span class="block truncate text-xs text-muted-foreground">{rd.owner_email ?? '—'}</span>
									</td>
									<td class="c-order">
										{#if rd.orders_published > 0}
											<span class="block text-sm font-medium" style="color: var(--success);">{fmtNum(rd.orders_published)} {tr('terbit')}</span>
											<span class="block text-xs text-muted-foreground tabular-nums">{tr('dari')} {fmtNum(rd.orders_total)} · {fmtRp(rd.gmv_published)}</span>
										{:else}
											<span class="block text-sm text-muted-foreground">{tr('Belum ada')}</span>
											{#if rd.orders_total > 0}<span class="block text-xs text-muted-foreground">{fmtNum(rd.orders_total)} {tr('belum terbit')}</span>{/if}
										{/if}
									</td>
									<td class="c-fee">
										{#if rd.cpa_paid}
											<span class="status-pill" data-tone="success">{tr('Dibayar')}</span>
											{#if rd.cpa_amount != null}<span class="mt-1 block text-xs text-muted-foreground tabular-nums">{fmtRp(rd.cpa_amount)}</span>{/if}
										{:else}
											<span class="text-xs text-muted-foreground">{tr('Belum')}</span>
										{/if}
									</td>
									<td class="c-status">
										<span class="status-pill" data-tone={STATUS_TONE[rd.status]}>{STATUS_LABEL[rd.status]}</span>
										{#if rd.status === 'rejected' && rd.rejection_reason}
											<span class="mt-1 block text-xs" style="color: var(--destructive);">{rd.rejection_reason}</span>
										{/if}
									</td>
									<td class="c-join text-xs text-muted-foreground">{fmtDate(rd.created_at)}</td>
									<td class="c-act">
										{#if rd.status === 'pending'}
											<div class="flex justify-end gap-2">
												<button class="btn-approve" type="button" disabled={busy} onclick={() => void approve(rd)}>{tr('Setujui')}</button>
												<button class="btn-reject" type="button" disabled={busy} onclick={() => { rejecting = rd; rejectionReason = ''; rejectionError = ''; }}>{tr('Tolak')}</button>
											</div>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{:else}
			<div class="app-card mt-3 overflow-x-auto">
				{#if attributions.length === 0}
					<p class="px-4 py-10 text-sm text-muted-foreground">{tr("Belum ada pendaftaran via referral.")}</p>
				{:else}
					<table class="data">
						<thead>
							<tr>
								<th class="c-ref">{tr('Referrer')}</th>
								<th class="c-ref">{tr('Referee')}</th>
								<th class="c-tier">{tr('Peran')}</th>
								<th class="c-ip">IP</th>
								<th class="c-note">{tr('Catatan')}</th>
								<th class="c-join">{tr('Tanggal')}</th>
							</tr>
						</thead>
						<tbody>
							{#each attributions as a (a.id)}
								<tr>
									<td class="c-ref">
										<span class="block truncate text-sm font-medium">{a.referrer_name}</span>
										<span class="block text-xs text-muted-foreground">{a.code}</span>
									</td>
									<td class="c-ref">
										<span class="block truncate text-sm">{a.referee_name ?? a.referee_email ?? '—'}</span>
										{#if a.referee_email}<span class="block truncate text-xs text-muted-foreground">{a.referee_email}</span>{/if}
									</td>
									<td class="c-tier">
										{#if a.referee_role}
											<span class="role-chip" data-role={a.referee_role}>{ROLE_LABEL[a.referee_role] ?? a.referee_role}</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="c-ip tabular-nums text-xs text-muted-foreground">{a.ip ?? '—'}</td>
									<td class="c-note">
										{#if a.fraud_note}
											<span class="status-pill" data-tone="danger">{a.fraud_note}</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="c-join text-xs text-muted-foreground">{a.created_at ? fmtDateTime(a.created_at) : '—'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{/if}
	{/if}
</section>

{#if rejecting}
	<Modal title="Tolak Domain" label="Tolak domain" maxWidth="md" onClose={() => { rejecting = null; rejectionError = ''; }}>
		<p class="text-sm text-muted-foreground">{rejecting.domain_name}</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if rejectionError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{rejectionError}</div>
			{/if}
			<input
				class="app-input w-full"
				placeholder={tr("Contoh: Domain tidak aktif / umur domain belum memenuhi syarat")}
				value={rejectionReason}
				oninput={(e) => { rejectionReason = (e.currentTarget as HTMLInputElement).value; rejectionError = ''; }}
			/>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => { rejecting = null; rejectionError = ''; }}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void reject()}>
				{busy ? 'Menolak…' : tr('Tolak domain')}
			</button>
		{/snippet}
	</Modal>
{/if}

<style>
	/* ── Funnel: satu deret angka, dipisah garis tipis ── */
	.funnel {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background-color: var(--card);
		overflow: hidden;
	}
	@media (min-width: 768px) {
		.funnel { grid-template-columns: repeat(4, minmax(0, 1fr)); }
	}
	@media (min-width: 1100px) {
		.funnel { grid-template-columns: repeat(7, minmax(0, 1fr)); }
	}
	.funnel-step {
		padding: 14px 16px;
		border-right: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent);
		border-bottom: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent);
		min-width: 0;
	}
	@media (min-width: 1100px) {
		.funnel-step { border-bottom: 0; }
		.funnel-step:last-child { border-right: 0; }
	}
	/* Mobile: 7 item dalam 2 kolom menyisakan satu sel — sel terakhir melebar */
	.funnel-step:last-child { grid-column: span 2; border-right: 0; }
	@media (min-width: 768px) {
		.funnel-step:last-child { grid-column: auto; }
	}
	.funnel-label {
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 16px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.funnel-value {
		margin-top: 4px;
		color: var(--foreground);
		font-size: 20px;
		font-weight: 500;
		line-height: 28px;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.funnel-value[data-zero='true'] { color: var(--muted-foreground); }

	/* ── Tab ── */
	.tab {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-height: 32px;
		border-radius: 8px;
		padding: 0 12px;
		color: var(--muted-foreground);
		font-size: 13px;
		font-weight: 500;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1), color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.tab:hover { background-color: var(--secondary); color: var(--foreground); }
	.tab[data-active='true'] { background-color: var(--secondary); color: var(--foreground); }
	.tab-count {
		border-radius: 4px;
		padding: 0 5px;
		background-color: color-mix(in srgb, var(--foreground) 7%, transparent);
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 16px;
	}
	.tab[data-active='true'] .tab-count { color: var(--foreground); }

	/* ── Tabel data: kolom tetap, angka rata kanan ── */
	.data {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
		table-layout: fixed;
	}
	.data thead th {
		padding: 10px 12px;
		border-bottom: 1px solid var(--border);
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 500;
		text-align: left;
		white-space: nowrap;
	}
	.data tbody td {
		padding: 10px 12px;
		vertical-align: middle;
		min-width: 0;
		overflow: hidden;
	}
	.data tbody tr + tr td { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.data tbody tr:hover td { background-color: color-mix(in srgb, var(--secondary) 55%, transparent); }

	/* Kolom angka: header ikut rata kanan supaya sejajar dengan isinya */
	.data thead th.c-n,
	.data thead th.c-order { text-align: right; }

	/* Kolom: referrer fleksibel, sisanya tetap supaya sejajar antar baris */
	.c-ref { width: 26%; }
	.c-code { width: 84px; }
	.c-n { width: 72px; text-align: right !important; }
	.c-order { width: 148px; text-align: right !important; }
	.c-komisi { width: 120px; }
	.c-dom { width: 20%; }
	.c-ref2 { width: 18%; }
	.c-tier { width: 96px; }
	.c-owner { width: 18%; }
	.c-fee { width: 96px; }
	.c-status { width: 112px; }
	.c-join { width: 104px; white-space: nowrap; }
	.c-act { width: 140px; }
	.c-ip { width: 120px; }
	.c-note { width: 20%; }
	/* Nol dibiarkan redup supaya angka yang berarti menonjol */
	.data td[data-dim='true'] { color: var(--muted-foreground); }

	.code-chip {
		display: inline-block;
		max-width: 100%;
		border: 1px solid var(--border);
		border-radius: 5px;
		padding: 0 6px;
		font-family: var(--font-mono);
		font-size: 11px;
		line-height: 18px;
		color: var(--foreground);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.ord-line { display: flex; align-items: baseline; justify-content: flex-end; gap: 4px; }
	.ord-strong { font-size: 13px; font-weight: 500; font-variant-numeric: tabular-nums; }
	.ord-strong[data-on='true'] { color: var(--success); }

	/* ── Filter: tiga grup terpisah, bukan satu deret tombol ── */
	.filter-group { display: inline-flex; align-items: center; gap: 4px; }
	.filter-label {
		margin-right: 4px;
		color: var(--muted-foreground);
		font-size: 11px;
	}
	.filter-chip {
		min-height: 28px;
		border: 1px solid transparent;
		border-radius: 7px;
		padding: 0 10px;
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 500;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1), color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.filter-chip:hover { background-color: var(--secondary); color: var(--foreground); }
	.filter-chip[data-active='true'] {
		border-color: var(--border);
		background-color: var(--card);
		color: var(--foreground);
	}

	/* ── Chip status & peran ── */
	.status-pill,
	.tier-chip,
	.role-chip {
		display: inline-block;
		border-radius: 5px;
		padding: 1px 7px;
		font-size: 11px;
		font-weight: 500;
		line-height: 18px;
		white-space: nowrap;
	}
	.status-pill[data-tone='success'] { background-color: var(--success-soft); color: var(--success); }
	.status-pill[data-tone='warning'] { background-color: var(--warning-soft); color: var(--warning); }
	.status-pill[data-tone='danger'] { background-color: var(--destructive-soft, #fdecee); color: var(--destructive); }
	.tier-chip[data-vip='true'] { background-color: var(--vvip-soft); color: var(--vvip); }
	.role-chip[data-role='client'] { background-color: var(--accent); color: var(--accent-foreground); }
	.role-chip[data-role='partner'] { background-color: var(--vvip-soft); color: var(--vvip); }

	/* ── Tombol approve/reject ── */
	.btn-approve,
	.btn-reject {
		min-height: 28px;
		border: 1px solid var(--border);
		border-radius: 7px;
		padding: 0 10px;
		background-color: var(--card);
		font-size: 12px;
		font-weight: 500;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.btn-approve { color: var(--success); }
	.btn-approve:hover:not(:disabled) { background-color: var(--success-soft); }
	.btn-reject { color: var(--destructive); }
	.btn-reject:hover:not(:disabled) { background-color: var(--destructive-soft, #fdecee); }
	.btn-approve:disabled,
	.btn-reject:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
