<script lang="ts">
	// Integrasi platform — port app/admin/integrations/page.tsx.
	// Tab 1 "Listing Media Mitra": seluruh media + owner mitra + tipe integrasi, dengan filter & modal edit.
	// Tab 2 "Kredensial & Endpoint CMS": daftar koneksi CMS per brand/mitra + tambah/test/cabut.
	import { onMount } from 'svelte';
	import { ExternalLink, Globe, Pencil, PlugZap, RefreshCw, Settings, Users, Zap } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type MediaRow = {
		id: string;
		name: string;
		slug: string | null;
		category: string | null;
		description: string | null;
		website_url: string | null;
		adapter_url: string | null;
		price: number;
		credit_cost: number;
		status: string;
		integration_type: string;
		integration_connected: boolean;
		partner_id: string | null;
		partner_name: string | null;
		publish_defaults: Record<string, unknown> | null;
		media_tier: string;
		created_at: string;
	};
	type Brand = { id: string; name: string; status: string };
	type Partner = { id: string; name: string | null; email?: string };
	type Connection = {
		id: string;
		provider: string;
		name: string;
		base_url: string;
		username: string | null;
		status: string;
		last_test_status: string | null;
		last_test_message: string | null;
		created_at: string;
	};

	const INTEGRATION_META: Record<
		string,
		{ label: string; desc: string; tone: 'accent' | 'neutral' | 'warning' | 'success'; isAuto: boolean }
	> = {
		tulisio_api: { label: 'Tauthub Publishing API', desc: 'Push otomatis (HMAC + Idempotency)', tone: 'accent', isAuto: true },
		wordpress: { label: 'WordPress REST API', desc: 'Push langsung via WP REST API CMS', tone: 'accent', isAuto: true },
		rss: { label: 'RSS Feed Importer', desc: 'Sindikasi otomatis via RSS Feed', tone: 'warning', isAuto: true },
		webhook: { label: 'Web Custom (Webhook)', desc: 'Webhook payload ke endpoint custom', tone: 'success', isAuto: true },
		manual: { label: 'Manual', desc: 'Publikasi manual tanpa integrasi API', tone: 'neutral', isAuto: false }
	};

	const INTEGRATION_OPTIONS = [
		{ value: 'all', label: 'Semua Tipe Integrasi' },
		{ value: 'tulisio_api', label: 'Tauthub Publishing API' },
		{ value: 'wordpress', label: 'WordPress REST API' },
		{ value: 'rss', label: 'RSS Feed' },
		{ value: 'webhook', label: 'Web Custom (Webhook)' },
		{ value: 'manual', label: 'Manual' }
	];

	const PAGE_SIZE = 25;

	let tab = $state<'media' | 'connections'>('media');
	let loading = $state(true);
	let error = $state('');

	let mediaList = $state<MediaRow[]>([]);
	let partners = $state<Partner[]>([]);
	let search = $state('');
	let filterType = $state('all');
	let filterPartner = $state('all');
	let filterStatus = $state('all');
	let mediaPage = $state(1);

	let editingMedia = $state<MediaRow | null>(null);
	let editBusy = $state(false);
	let editForm = $state({ integration_type: 'manual', website_url: '', adapter_url: '', partner_id: '' });

	let connMode = $state<'brand' | 'partner'>('brand');
	let brands = $state<Brand[]>([]);
	let selectedBrandId = $state('');
	let selectedPartnerId = $state('');
	let connections = $state<Connection[]>([]);
	let createOpen = $state(false);
	let connBusy = $state(false);
	let connLoading = $state(false);
	let connForm = $state({
		provider: 'wordpress',
		name: '',
		base_url: '',
		username: '',
		application_password: '',
		secret: ''
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadMediaAndPartners();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadMediaAndPartners() {
		loading = true;
		error = '';
		try {
			const [mediaRes, partnerRes] = await Promise.all([
				apiRequest('/v1/platform/media?limit=500'),
				apiRequest('/v1/platform/partners?limit=500')
			]);
			if (!mediaRes.ok) {
				const msg = await readApiError(mediaRes, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			const mediaData = await mediaRes.json();
			mediaList = mediaData.media ?? [];
			if (partnerRes.ok) partners = (await partnerRes.json()).partners ?? [];
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		} finally {
			loading = false;
		}
	}

	const connBase = $derived(
		connMode === 'brand'
			? `/v1/platform/operations/brands/${selectedBrandId}/connections`
			: `/v1/platform/partners/${selectedPartnerId}/connections`
	);
	const selectedConnOwner = $derived(connMode === 'brand' ? selectedBrandId : selectedPartnerId);

	async function loadConnOwners() {
		try {
			if (connMode === 'brand') {
				const res = await apiRequest('/v1/platform/operations/brands');
				if (!res.ok) return;
				const data = (await res.json()) as Brand[];
				brands = data ?? [];
				if (data.length > 0 && !selectedBrandId) selectedBrandId = data[0].id;
			} else {
				const res = await apiRequest('/v1/platform/partners');
				if (!res.ok) return;
				const data = await res.json();
				partners = data.partners ?? [];
				if ((data.partners ?? []).length > 0 && !selectedPartnerId) selectedPartnerId = data.partners[0].id;
			}
		} catch {
			/* opsional */
		}
	}

	async function loadConnections() {
		if (!selectedConnOwner) return;
		connLoading = true;
		try {
			const res = await apiRequest(connBase);
			if (res.ok) connections = (await res.json()).connections ?? [];
		} catch {
			/* opsional */
		} finally {
			connLoading = false;
		}
	}

	function openConnections() {
		tab = 'connections';
		void (async () => {
			await loadConnOwners();
			await loadConnections();
		})();
	}

	async function switchConnMode(mode: 'brand' | 'partner') {
		connMode = mode;
		connections = [];
		await loadConnOwners();
		await loadConnections();
	}

	function openEditMedia(row: MediaRow) {
		editingMedia = row;
		editForm = {
			integration_type: row.integration_type || 'manual',
			website_url: row.website_url ?? '',
			adapter_url: row.adapter_url ?? '',
			partner_id: row.partner_id ?? ''
		};
	}

	async function saveMediaEdit(e: SubmitEvent) {
		e.preventDefault();
		if (!editingMedia) return;
		const target = editingMedia;
		editBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/media/${target.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: target.name,
					category: target.category,
					website_url: editForm.website_url.trim() || null,
					adapter_url: editForm.adapter_url.trim() || null,
					integration_type: editForm.integration_type,
					partner_id: editForm.partner_id || null,
					price: target.price,
					credit_cost: target.credit_cost,
					status: target.status,
					media_tier: target.media_tier
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			editingMedia = null;
			await loadMediaAndPartners();
		} finally {
			editBusy = false;
		}
	}

	async function createConnection(e: SubmitEvent) {
		e.preventDefault();
		connBusy = true;
		error = '';
		try {
			const body: Record<string, string | null> = {
				provider: connForm.provider,
				name: connForm.name.trim(),
				base_url: connForm.base_url.trim(),
				username: connForm.provider === 'wordpress' ? connForm.username.trim() || null : null
			};
			if (connForm.provider === 'wordpress') body.application_password = connForm.application_password;
			else body.secret = connForm.secret;
			const res = await apiRequest(connBase, { method: 'POST', body: JSON.stringify(body) });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			createOpen = false;
			connForm = { provider: 'wordpress', name: '', base_url: '', username: '', application_password: '', secret: '' };
			await loadConnections();
		} finally {
			connBusy = false;
		}
	}

	async function testConnection(row: Connection) {
		error = '';
		const res = await apiRequest(`${connBase}/${row.id}/test`, { method: 'POST' });
		if (!res.ok) {
			error = await readApiError(res, 'Permintaan platform gagal.');
			return;
		}
		await loadConnections();
	}

	async function revokeConnection(row: Connection) {
		if (!confirm(`Cabut koneksi ${row.name}?`)) return;
		error = '';
		const res = await apiRequest(`${connBase}/${row.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, 'Permintaan platform gagal.');
			return;
		}
		await loadConnections();
	}

	const filteredMedia = $derived(
		mediaList.filter((item) => {
			if (filterType !== 'all' && item.integration_type !== filterType) return false;
			if (filterPartner !== 'all' && (item.partner_id || 'none') !== filterPartner) return false;
			if (filterStatus !== 'all' && item.status !== filterStatus) return false;
			if (search.trim()) {
				const q = search.toLowerCase().trim();
				const matchName = item.name.toLowerCase().includes(q);
				const matchWeb = (item.website_url ?? '').toLowerCase().includes(q);
				const matchPartner = (item.partner_name ?? '').toLowerCase().includes(q);
				const matchType = (item.integration_type ?? '').toLowerCase().includes(q);
				if (!matchName && !matchWeb && !matchPartner && !matchType) return false;
			}
			return true;
		})
	);

	const visibleMedia = $derived(filteredMedia.slice((mediaPage - 1) * PAGE_SIZE, mediaPage * PAGE_SIZE));

	const totalCount = $derived(mediaList.length);
	const autoCount = $derived(mediaList.filter((m) => m.integration_type !== 'manual').length);
	const manualCount = $derived(mediaList.filter((m) => m.integration_type === 'manual').length);
	const withPartnerCount = $derived(mediaList.filter((m) => !!m.partner_id).length);

	function resetFilters() {
		search = '';
		filterType = 'all';
		filterPartner = 'all';
		filterStatus = 'all';
		mediaPage = 1;
	}

	function displayUrl(url: string) {
		if (url.startsWith('http')) return url;
		return `https://${url}`;
	}
	function shortUrl(url: string) {
		return url.replace(/^https?:\/\//i, '').replace(/\/$/, '');
	}
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Listing Media Mitra"
		description="Daftar seluruh media mitra yang terdaftar di Tauthub lengkap dengan informasi website, nama mitra pemilik, dan tipe integrasi publikasi."
	>
		<Badge tone="accent" label="{mediaList.length} media mitra" />
		<button class="app-btn app-btn-soft" type="button" onclick={() => void loadMediaAndPartners()}><RefreshCw size={15} /> Refresh</button>
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat Listing Media Mitra…" />

	{#if error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke direktori integrasi platform.</p>
		</div>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-4">
			{#each [
				{ label: 'TOTAL MEDIA MITRA', value: totalCount, sub: 'Katalog media platform' },
				{ label: 'INTEGRASI OTOMATIS', value: autoCount, sub: 'WordPress · API · RSS · Webhook' },
				{ label: 'INTEGRASI MANUAL', value: manualCount, sub: 'Diproses manual mitra' },
				{ label: 'MITRA TERHUBUNG', value: withPartnerCount, sub: 'Media dengan pemilik mitra' }
			] as card (card.label)}
				<div class="app-card p-3">
					<p class="text-xs font-bold text-muted-foreground">{card.label}</p>
					<p class="mt-1 text-2xl font-bold">{card.value}</p>
					<p class="text-xs text-muted-foreground">{card.sub}</p>
				</div>
			{/each}
		</div>

		<div class="mt-4 flex flex-wrap gap-2">
			<button class="app-btn {tab === 'media' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs" type="button" onclick={() => (tab = 'media')}>
				<PlugZap size={13} /> Listing Media Mitra ({filteredMedia.length})
			</button>
			<button class="app-btn {tab === 'connections' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs" type="button" onclick={openConnections}>
				<PlugZap size={13} /> Kredensial &amp; Endpoint CMS
			</button>
		</div>

		{#if tab === 'media'}
			<div class="mt-3 space-y-3">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex flex-1 flex-wrap items-center gap-2">
						<input class="app-input min-w-[240px] max-w-[360px]" placeholder="Cari media, website, atau mitra…" bind:value={search} oninput={() => (mediaPage = 1)} />
						<select class="app-input w-auto min-w-[190px]" aria-label="Filter tipe integrasi" bind:value={filterType} onchange={() => (mediaPage = 1)}>
							{#each INTEGRATION_OPTIONS as opt (opt.value)}
								<option value={opt.value}>{opt.label}</option>
							{/each}
						</select>
						<select class="app-input w-auto min-w-[180px]" aria-label="Filter mitra" bind:value={filterPartner} onchange={() => (mediaPage = 1)}>
							<option value="all">Semua Mitra</option>
							<option value="none">Tanpa Mitra (Platform)</option>
							{#each partners as p (p.id)}
								<option value={p.id}>{p.name ?? p.email ?? p.id.slice(0, 8)}</option>
							{/each}
						</select>
						<select class="app-input w-auto min-w-[160px]" aria-label="Filter status" bind:value={filterStatus} onchange={() => (mediaPage = 1)}>
							<option value="all">Semua Status</option>
							<option value="active">Aktif</option>
							<option value="inactive">Inaktif</option>
							<option value="draft">Draft</option>
						</select>
					</div>
					{#if search || filterType !== 'all' || filterPartner !== 'all' || filterStatus !== 'all'}
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={resetFilters}>Reset Filter</button>
					{/if}
				</div>

				<div class="app-card overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 1080px;">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-4 py-2.5 text-left font-medium" style="width:28%;">Media &amp; Website</th>
								<th class="px-4 py-2.5 text-left font-medium" style="width:22%;">Nama Mitra</th>
								<th class="px-4 py-2.5 text-left font-medium" style="width:26%;">Integrasi Ke Apa</th>
								<th class="px-4 py-2.5 text-left font-medium" style="width:12%;">Status</th>
								<th class="px-4 py-2.5 text-right font-medium" style="width:12%;">Aksi</th>
							</tr>
						</thead>
						<tbody>
							{#each visibleMedia as row (row.id)}
								{@const meta = INTEGRATION_META[row.integration_type] ?? INTEGRATION_META.manual}
								{@const partnerDisplay = row.partner_name || partners.find((p) => p.id === row.partner_id)?.name}
								<tr class="border-t">
									<td class="px-4 py-2.5">
										<p class="font-bold">{row.name}</p>
										{#if row.website_url}
											<a
												class="mt-1 inline-flex items-center gap-1 text-xs"
												style="color: var(--accent-foreground);"
												href={displayUrl(row.website_url)}
												target="_blank"
												rel="noopener noreferrer"
											>
												<Globe size={12} /> {shortUrl(row.website_url)} <ExternalLink size={10} />
											</a>
										{:else}
											<p class="text-xs text-muted-foreground">Website belum diatur</p>
										{/if}
										{#if row.category}<p class="text-xs text-muted-foreground">Kategori: {row.category}</p>{/if}
									</td>
									<td class="px-4 py-2.5">
										{#if row.partner_id}
											<div class="flex items-center gap-1">
												<Users size={14} class="text-muted-foreground" />
												<span class="font-medium">{partnerDisplay ?? 'Mitra'}</span>
											</div>
											<a class="text-[11px] text-muted-foreground" href="/admin/partners">ID: {row.partner_id.slice(0, 8)}…</a>
										{:else}
											<Badge tone="neutral" label="Milik Platform (Tanpa Mitra)" />
										{/if}
									</td>
									<td class="px-4 py-2.5">
										<Badge tone={meta.tone} label={meta.label} />
										<p class="mt-1 text-xs text-muted-foreground">{meta.desc}</p>
										{#if row.adapter_url}
											<p class="mt-1 rounded bg-secondary px-1.5 py-0.5 text-xs">Endpoint: {row.adapter_url}</p>
										{/if}
									</td>
									<td class="px-4 py-2.5">
										<span title={row.integration_connected ? 'Integrasi publikasi sudah terhubung' : 'Integrasi publikasi belum terhubung'}>
											<Badge tone={row.integration_connected ? 'success' : 'warning'} label={row.integration_connected ? 'Aktif' : 'Tidak Aktif'} />
										</span>
									</td>
									<td class="px-4 py-2.5 text-right">
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" title="Ubah konfigurasi integrasi & mitra" onclick={() => openEditMedia(row)}>
											<Pencil size={12} /> Edit
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if !loading && filteredMedia.length === 0}
						<p class="px-4 py-12 text-center text-sm text-muted-foreground">Tidak ada media mitra yang sesuai dengan filter pencarian.</p>
					{/if}
				</div>
				<Pagination total={filteredMedia.length} limit={PAGE_SIZE} offset={(mediaPage - 1) * PAGE_SIZE} onChange={(next) => (mediaPage = next / PAGE_SIZE + 1)} />
			</div>
		{:else}
			<div class="mt-3 space-y-3">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex flex-wrap items-center gap-2">
						<select class="app-input w-auto" aria-label="Mode pemilik" value={connMode} onchange={(e) => void switchConnMode((e.currentTarget as HTMLSelectElement).value as 'brand' | 'partner')}>
							<option value="brand">Brand (Klien)</option>
							<option value="partner">Partner (Mitra)</option>
						</select>
						{#if connMode === 'brand'}
							<span class="text-sm text-muted-foreground">Brand:</span>
							<select
								class="app-input w-auto min-w-[200px]"
								aria-label="Pilih brand"
								bind:value={selectedBrandId}
								onchange={() => void loadConnections()}
							>
								{#each brands as b (b.id)}
									<option value={b.id}>{b.name}</option>
								{/each}
							</select>
						{:else}
							<span class="text-sm text-muted-foreground">Mitra:</span>
							<select
								class="app-input w-auto min-w-[200px]"
								aria-label="Pilih mitra"
								bind:value={selectedPartnerId}
								onchange={() => void loadConnections()}
							>
								{#each partners as p (p.id)}
									<option value={p.id}>{p.name ?? p.id.slice(0, 8)}</option>
								{/each}
							</select>
						{/if}
					</div>
					<button class="app-btn app-btn-primary" type="button" disabled={!selectedConnOwner} onclick={() => (createOpen = true)}>Tambah Endpoint</button>
				</div>

				<div class="app-card overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 1080px;">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-4 py-2.5 text-left font-medium">Nama Koneksi</th>
								<th class="px-4 py-2.5 text-left font-medium">Provider</th>
								<th class="px-4 py-2.5 text-left font-medium">Endpoint Base URL</th>
								<th class="px-4 py-2.5 text-left font-medium">Status</th>
								<th class="px-4 py-2.5 text-left font-medium">Hasil Uji Terakhir</th>
								<th class="px-4 py-2.5 text-right font-medium">Aksi</th>
							</tr>
						</thead>
						<tbody>
							{#each connections as row (row.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">
										<div class="flex items-center gap-2">
											<Settings size={15} />
											<div>
												<p class="font-medium">{row.name}</p>
												<p class="text-xs text-muted-foreground">{row.username ?? row.id.slice(0, 8)}</p>
											</div>
										</div>
									</td>
									<td class="px-4 py-2.5"><Badge tone="neutral" label={row.provider} /></td>
									<td class="px-4 py-2.5"><code class="text-xs">{row.base_url}</code></td>
									<td class="px-4 py-2.5"><Badge tone={row.status === 'active' ? 'success' : 'neutral'} label={row.status} /></td>
									<td class="px-4 py-2.5 text-xs" style="color: {row.last_test_status === 'ok' ? 'var(--success)' : row.last_test_status === 'error' ? 'var(--destructive)' : 'var(--muted-foreground)'};">
										{row.last_test_message ?? 'Belum diuji'}
									</td>
									<td class="px-4 py-2.5 text-right">
										{#if row.status === 'active'}
											<div class="flex justify-end gap-2">
												<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void testConnection(row)}><Zap size={12} /> Test</button>
												<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void revokeConnection(row)}>Cabut</button>
											</div>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if !connLoading && connections.length === 0}
						<p class="px-4 py-12 text-center text-sm text-muted-foreground">
							{connMode === 'brand' ? 'Belum ada koneksi CMS/Webhook untuk brand ini.' : 'Belum ada koneksi CMS/Webhook untuk mitra ini.'}
						</p>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</section>

{#if editingMedia}
	<Modal
		title="Edit Integrasi Media · {editingMedia.name}"
		label="Edit integrasi media"
		maxWidth="lg"
		onClose={() => (editingMedia = null)}
	>
		<form id="edit-media-form" class="space-y-3" onsubmit={saveMediaEdit}>
			<p class="text-sm text-muted-foreground">Atur website, kepemilikan mitra, serta tipe dan target endpoint integrasi publikasi.</p>
			<label class="block">
				<span class="text-sm font-medium">Website / Domain Media</span>
				<input class="app-input mt-1 w-full" type="url" placeholder="https://suarapublika.com" bind:value={editForm.website_url} />
			</label>
			<label class="block">
				<span class="text-sm font-medium">Tipe Integrasi Publikasi</span>
				<select class="app-input mt-1 w-full" bind:value={editForm.integration_type}>
					<option value="manual">Manual (Dikelola oleh Mitra / Tanpa API)</option>
					<option value="tulisio_api">Tauthub Publishing API (Push Webhook + HMAC)</option>
					<option value="wordpress">WordPress REST API (CMS Direct Post)</option>
					<option value="rss">RSS Feed Importer (Sindikasi XML/RSS)</option>
					<option value="webhook">Web Custom (Webhook Notification)</option>
				</select>
			</label>
			{#if editForm.integration_type !== 'manual'}
				<label class="block">
					<span class="text-sm font-medium">Endpoint / Adapter URL (Opsional)</span>
					<span class="block text-xs text-muted-foreground">URL endpoint mitra yang dituju saat mengirim konten publikasi secara otomatis.</span>
					<input class="app-input mt-1 w-full" type="url" placeholder="https://suarapublika.com/api/tauthub/publish" bind:value={editForm.adapter_url} />
				</label>
			{/if}
			<label class="block">
				<span class="text-sm font-medium">Mitra Pemilik (Partner)</span>
				<select class="app-input mt-1 w-full" bind:value={editForm.partner_id}>
					<option value="">Tanpa Mitra (Milik Platform / Direct)</option>
					{#each partners as p (p.id)}
						<option value={p.id}>{p.name ?? p.email ?? p.id}</option>
					{/each}
				</select>
			</label>
		</form>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (editingMedia = null)}>Batal</button>
			<button class="app-btn app-btn-primary" type="submit" form="edit-media-form" disabled={editBusy}>{editBusy ? 'Menyimpan…' : 'Simpan Integrasi'}</button>
		{/snippet}
	</Modal>
{/if}

{#if createOpen}
	<Modal title="Tambah Endpoint Koneksi CMS" label="Tambah koneksi CMS" maxWidth="lg" onClose={() => (createOpen = false)}>
		<form id="create-connection-form" class="space-y-3" onsubmit={createConnection}>
			<label class="block">
				<span class="text-sm font-medium">Provider</span>
				<select class="app-input mt-1 w-full" bind:value={connForm.provider}>
					<option value="wordpress">WordPress REST API</option>
					<option value="webhook">Web Custom (API + HMAC)</option>
				</select>
			</label>
			<label class="block">
				<span class="text-sm font-medium">Nama Koneksi</span>
				<input class="app-input mt-1 w-full" required placeholder="Website Utama Mitra" bind:value={connForm.name} />
			</label>
			<label class="block">
				<span class="text-sm font-medium">{connForm.provider === 'wordpress' ? 'URL WordPress' : 'Endpoint URL'}</span>
				<input
					class="app-input mt-1 w-full"
					required
					type="url"
					placeholder={connForm.provider === 'wordpress' ? 'https://situs.example' : 'https://situs.example/api/tauthub/content'}
					bind:value={connForm.base_url}
				/>
			</label>
			{#if connForm.provider === 'wordpress'}
				<label class="block">
					<span class="text-sm font-medium">Username / Email WordPress</span>
					<input class="app-input mt-1 w-full" required bind:value={connForm.username} />
				</label>
				<label class="block">
					<span class="text-sm font-medium">Application Password</span>
					<input class="app-input mt-1 w-full" required type="password" bind:value={connForm.application_password} />
				</label>
			{:else}
				<label class="block">
					<span class="text-sm font-medium">Shared Secret (min 16 karakter)</span>
					<input class="app-input mt-1 w-full" required type="password" minlength={16} bind:value={connForm.secret} />
				</label>
			{/if}
		</form>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
			<button class="app-btn app-btn-primary" type="submit" form="create-connection-form" disabled={connBusy}>{connBusy ? 'Menyimpan…' : 'Simpan Endpoint'}</button>
		{/snippet}
	</Modal>
{/if}
