<script lang="ts">
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

	const ROLE_LABEL: Record<string, string> = { partner: 'Mitra', client: 'Klien', admin: 'Admin' };
	const STATUS_LABEL: Record<string, string> = { pending: 'Menunggu', approved: 'Disetujui', rejected: 'Ditolak' };
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

<section class="mx-auto max-w-[1400px]">
	<PageHeader
		title="Monitoring Non VIP"
		description="Monitoring referral mitra non-VIP: siapa share kode, berapa klik & pendaftaran, domain yang join, domain yang dapat orderan, dan siapa pemilik tiap domain."
	/>

	<Feedback loading={!loaded} {error} loadingText="Memuat monitoring non VIP…" />

	{#if loaded}
		<div class="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">Referrer Non VIP</p><p class="mt-1 text-2xl font-bold">{fmtNum(totalReferrers)}</p></div>
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">Total Klik</p><p class="mt-1 text-2xl font-bold">{fmtNum(totalClicks)}</p></div>
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">Total Pendaftaran</p><p class="mt-1 text-2xl font-bold">{fmtNum(totalSignups)}</p></div>
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">Domain Join</p><p class="mt-1 text-2xl font-bold">{fmtNum(totalDomainsJoined)}</p></div>
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">Domain Dapat Orderan</p><p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtNum(totalDomainsWithOrders)}</p></div>
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">GMV Order Terbit</p><p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtRp(totalGmv)}</p></div>
			<div class="app-card p-4"><p class="text-xs text-muted-foreground">Total Komisi</p><p class="mt-1 text-2xl font-bold" style="color: var(--success);">{fmtRp(totalKomisi)}</p></div>
		</div>

		<div class="mt-4 flex flex-wrap gap-1 border-b">
			{#each [
				['referrers', `Per Referrer (${referrers.length})`],
				['domains', `Domain Referral (${domainsTotal})`],
				['attributions', `Pendaftaran (${attributions.length})`]
			] as [key, label] (key)}
				<button
					class="border-b-2 px-3 py-2 text-sm font-medium transition-colors"
					style="border-color: {tab === key ? 'var(--primary)' : 'transparent'}; color: {tab === key ? 'var(--primary)' : 'var(--muted-foreground)'};"
					type="button"
					onclick={() => (tab = key as typeof tab)}
				>{label}</button>
			{/each}
		</div>

		{#if tab === 'referrers'}
			<div class="app-card mt-3 overflow-x-auto">
				{#if referrers.length === 0}
					<p class="px-4 py-4 text-sm text-muted-foreground">Belum ada aktivitas referral dari mitra non-VIP.</p>
				{:else}
					<table class="w-full text-sm" style="min-width: 1080px;">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">Referrer</th>
								<th class="px-4 py-2.5 font-medium">Kode</th>
								<th class="px-4 py-2.5 font-medium">Klik</th>
								<th class="px-4 py-2.5 font-medium">Pendaftaran</th>
								<th class="px-4 py-2.5 font-medium">Mitra</th>
								<th class="px-4 py-2.5 font-medium">Klien</th>
								<th class="px-4 py-2.5 font-medium">Domain Join</th>
								<th class="px-4 py-2.5 font-medium">Domain Order</th>
								<th class="px-4 py-2.5 font-medium">Komisi</th>
							</tr>
						</thead>
						<tbody>
							{#each referrers as r (r.partner_id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">
										<p class="font-medium">{r.name}</p>
										<p class="text-xs text-muted-foreground">{r.email}</p>
									</td>
									<td class="px-4 py-2.5"><Badge label={r.referral_code ?? '—'} tone="neutral" /></td>
									<td class="px-4 py-2.5 tabular-nums">{fmtNum(r.clicks)}</td>
									<td class="px-4 py-2.5 font-medium tabular-nums">{fmtNum(r.signups)}</td>
									<td class="px-4 py-2.5"><Badge label={String(Number(r.mitra_count || 0))} tone="accent" /></td>
									<td class="px-4 py-2.5"><Badge label={String(Number(r.klien_count || 0))} tone="success" /></td>
									<td class="px-4 py-2.5">
										<p class="font-medium">{fmtNum(r.domains_joined)}</p>
										<p class="text-xs text-muted-foreground">domain masuk</p>
									</td>
									<td class="px-4 py-2.5">
										<p class="font-medium" style={Number(r.domains_with_orders || 0) > 0 ? 'color: var(--success);' : undefined}>{fmtNum(r.domains_with_orders)} dapat orderan</p>
										<p class="text-xs text-muted-foreground">{fmtNum(r.orders_published_total)} order terbit</p>
										<p class="text-xs text-muted-foreground">GMV {fmtRp(r.gmv_published_total)}</p>
									</td>
									<td class="px-4 py-2.5 font-medium" style="color: var(--success);">{fmtRp(r.commission_total)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{:else if tab === 'domains'}
			<div class="mt-3 flex flex-wrap items-center gap-2">
				{#each [['', 'Semua Domain'], ['with', 'Sudah Dapat Orderan'], ['without', 'Belum Ada Orderan']] as [key, label] (key || 'all')}
					<button
						class="app-btn !min-h-8 !px-3 !text-xs"
						class:app-btn-primary={orderFilter === key}
						class:app-btn-soft={orderFilter !== key}
						type="button"
						onclick={() => setOrderFilter(key as '' | 'with' | 'without')}
					>{label}</button>
				{/each}
				<span class="text-xs text-muted-foreground">·</span>
				{#each [['', 'Semua Tier'], ['vip', 'VIP'], ['nonvip', 'Non VIP']] as [key, label] (key || 'alltier')}
					<button
						class="app-btn !min-h-8 !px-3 !text-xs"
						class:app-btn-primary={tierFilter === key}
						class:app-btn-soft={tierFilter !== key}
						type="button"
						onclick={() => (tierFilter = key as '' | 'vip' | 'nonvip')}
					>{label}</button>
				{/each}
				<span class="text-xs text-muted-foreground">·</span>
				{#each [['', 'Semua Status'], ['pending', 'Menunggu'], ['approved', 'Disetujui'], ['rejected', 'Ditolak']] as [key, label] (key || 'allstatus')}
					<button
						class="app-btn !min-h-8 !px-3 !text-xs"
						class:app-btn-primary={domainStatus === key}
						class:app-btn-soft={domainStatus !== key}
						type="button"
						onclick={() => setDomainStatus(key as '' | 'pending' | 'approved' | 'rejected')}
					>{label}</button>
				{/each}
			</div>

			<div class="app-card mt-3 overflow-x-auto">
				{#if visibleDomains.length === 0}
					<div class="flex flex-col items-center px-4 py-12 text-center">
						<Globe size={26} class="text-muted-foreground" />
						<p class="mt-2 text-sm text-muted-foreground">Belum ada domain referral yang cocok dengan filter.</p>
					</div>
				{:else}
					<table class="w-full text-sm" style="min-width: 1320px;">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">Domain</th>
								<th class="px-4 py-2.5 font-medium">Referrer (Akun)</th>
								<th class="px-4 py-2.5 font-medium">Tier</th>
								<th class="px-4 py-2.5 font-medium">Pemilik Domain</th>
								<th class="px-4 py-2.5 font-medium">Media</th>
								<th class="px-4 py-2.5 font-medium">Order</th>
								<th class="px-4 py-2.5 font-medium">Fee</th>
								<th class="px-4 py-2.5 font-medium">Status</th>
								<th class="px-4 py-2.5 font-medium">Join</th>
								<th class="px-4 py-2.5"></th>
							</tr>
						</thead>
						<tbody>
							{#each visibleDomains as rd (rd.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">
										<div class="flex items-center gap-2">
											<Globe size={16} class="shrink-0 text-muted-foreground" />
											<div>
												<p class="font-medium">{rd.domain_name}</p>
												<p class="text-xs text-muted-foreground">{rd.is_tld ? 'TLD' : 'Subdomain'}</p>
											</div>
										</div>
									</td>
									<td class="px-4 py-2.5">
										<p class="text-sm">{rd.referrer_name ?? '—'}</p>
										<p class="text-xs text-muted-foreground">{rd.referrer_partner_id ? `ID ${rd.referrer_partner_id.slice(0, 8)}` : '—'}</p>
									</td>
									<td class="px-4 py-2.5">
										<Badge label={referrerTierLabel(rd.referrer_vip_tier, rd.referrer_is_vip)} tone={rd.referrer_is_vip ? 'accent' : 'neutral'} />
									</td>
									<td class="px-4 py-2.5">
										<p class="text-sm">{rd.owner_name ?? '—'}</p>
										<p class="text-xs text-muted-foreground">{rd.owner_email ?? '—'}</p>
									</td>
									<td class="px-4 py-2.5">
										<p class="text-sm">{rd.media_name ?? '—'}</p>
										{#if rd.media_website_url}<p class="max-w-[160px] break-all text-xs text-muted-foreground">{rd.media_website_url}</p>{/if}
									</td>
									<td class="px-4 py-2.5">
										{#if rd.orders_published > 0}
											<p class="text-sm font-medium" style="color: var(--success);">{fmtNum(rd.orders_published)} terbit</p>
											<p class="text-xs text-muted-foreground">dari {fmtNum(rd.orders_total)} order</p>
											<p class="text-xs text-muted-foreground">GMV {fmtRp(rd.gmv_published)}</p>
											{#if rd.last_order_at}<p class="text-xs text-muted-foreground">terakhir {fmtDate(rd.last_order_at)}</p>{/if}
										{:else}
											<p class="text-sm text-muted-foreground">Belum ada</p>
											{#if rd.orders_total > 0}<p class="text-xs text-muted-foreground">{fmtNum(rd.orders_total)} order belum terbit</p>{/if}
										{/if}
									</td>
									<td class="px-4 py-2.5">
										{#if rd.cpa_paid}
											<Badge label="Dibayar" tone="success" />
											{#if rd.cpa_amount != null}<p class="mt-1 text-xs text-muted-foreground">{fmtRp(rd.cpa_amount)}</p>{/if}
										{:else}
											<Badge label="Belum" tone="neutral" />
										{/if}
									</td>
									<td class="px-4 py-2.5">
										<Badge label={STATUS_LABEL[rd.status]} tone={STATUS_TONE[rd.status]} />
										{#if rd.status === 'rejected' && rd.rejection_reason}
											<p class="mt-1 max-w-[160px] text-xs" style="color: var(--destructive);">{rd.rejection_reason}</p>
										{/if}
									</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{fmtDate(rd.created_at)}</td>
									<td class="px-4 py-2.5">
										{#if rd.status === 'pending'}
											<div class="flex gap-2">
												<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--success-soft); color: var(--success);" type="button" disabled={busy} onclick={() => void approve(rd)}>Approve</button>
												<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);" type="button" disabled={busy} onclick={() => { rejecting = rd; rejectionReason = ''; rejectionError = ''; }}>Reject</button>
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
					<p class="px-4 py-4 text-sm text-muted-foreground">Belum ada pendaftaran via referral.</p>
				{:else}
					<table class="w-full text-sm" style="min-width: 900px;">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">Referrer</th>
								<th class="px-4 py-2.5 font-medium">Referee</th>
								<th class="px-4 py-2.5 font-medium">Peran</th>
								<th class="px-4 py-2.5 font-medium">IP</th>
								<th class="px-4 py-2.5 font-medium">Catatan</th>
								<th class="px-4 py-2.5 font-medium">Tanggal</th>
							</tr>
						</thead>
						<tbody>
							{#each attributions as a (a.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">
										<p class="font-medium">{a.referrer_name}</p>
										<p class="text-xs text-muted-foreground">{a.code}</p>
									</td>
									<td class="px-4 py-2.5">
										<p>{a.referee_name ?? a.referee_email ?? '—'}</p>
										{#if a.referee_email}<p class="text-xs text-muted-foreground">{a.referee_email}</p>{/if}
									</td>
									<td class="px-4 py-2.5">
										{#if a.referee_role}
											<Badge label={ROLE_LABEL[a.referee_role] ?? a.referee_role} tone={a.referee_role === 'partner' ? 'accent' : a.referee_role === 'client' ? 'success' : 'neutral'} />
										{:else}
											—
										{/if}
									</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{a.ip ?? '—'}</td>
									<td class="px-4 py-2.5">
										{#if a.fraud_note}
											<Badge label={a.fraud_note} tone="danger" />
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{a.created_at ? fmtDateTime(a.created_at) : '—'}</td>
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
				placeholder="Contoh: Domain tidak aktif / umur domain belum memenuhi syarat"
				value={rejectionReason}
				oninput={(e) => { rejectionReason = (e.currentTarget as HTMLInputElement).value; rejectionError = ''; }}
			/>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => { rejecting = null; rejectionError = ''; }}>Batal</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void reject()}>
				{busy ? 'Menolak…' : 'Tolak Domain'}
			</button>
		{/snippet}
	</Modal>
{/if}
