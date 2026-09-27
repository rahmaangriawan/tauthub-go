<script lang="ts">
	// Daftar Media mitra — port section "Daftar Media" portal mitra:
	// daftar + filter status, tambah media, edit, arsip, dan impor massal XLSX
	// (pratinjau → konfirmasi, parity alur bulk import app lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ExternalLink, Pencil, Plus, Search, Trash2, Upload, X } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	type Media = {
		id: string; name: string; category: string | null; categories: string[];
		website_url: string | null; price: number; effective_price: number; client_price: number;
		markup_percent: number; status: string; integration_type: string;
		publication_mode: string; traffic_estimate: number; max_links: number | null;
		bahasa_artikel: string[]; label_sponsor: string; dewan_pers_verified: boolean;
		pending_publications: number; total_terbit: number; price_status: string;
		topik_terlarang: string[]; contoh_artikel_url: string | null;
	};

	let items = $state<Media[]>([]);
	let categories = $state<string[]>([]);
	let autoPublishEnabled = $state(false);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let q = $state('');
	let statusFilter = $state('');

	// Form tambah/edit
	let showForm = $state(false);
	let editingId = $state<string | null>(null);
	let form = $state({
		name: '', website_url: '', categories: [] as string[], media_type: 'mainstream',
		price: 0, traffic_estimate: 0, integration_type: 'manual', publication_mode: 'draft',
		pricing_type: 'nett', max_links: 1, bahasa_artikel: ['Indonesia'], label_sponsor: 'tidak_ada',
		word_count_mode: 'sesuai_permintaan', allow_embed: false, topik_terlarang: '', notes: ''
	});

	// Impor massal
	let importResult = $state<any>(null);
	let importError = $state('');

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
		const params = new URLSearchParams();
		if (statusFilter) params.set('status', statusFilter);
		params.set('limit', '200');
		const res = await apiRequest(`/v1/partner/media?${params}`);
		if (res.ok) {
			const data = await res.json();
			items = data.media ?? [];
			categories = data.categories ?? [];
			autoPublishEnabled = Boolean(data.auto_publish_enabled);
		}
	}

	const filtered = $derived(
		items.filter((m) => {
			if (!q.trim()) return true;
			const term = q.trim().toLowerCase();
			return m.name.toLowerCase().includes(term) || (m.website_url ?? '').toLowerCase().includes(term);
		})
	);

	function openCreate() {
		editingId = null;
		error = '';
		form = {
			name: '', website_url: '', categories: [], media_type: 'mainstream', price: 0,
			traffic_estimate: 0, integration_type: 'manual', publication_mode: 'draft',
			pricing_type: 'nett', max_links: 1, bahasa_artikel: ['Indonesia'], label_sponsor: 'tidak_ada',
			word_count_mode: 'sesuai_permintaan', allow_embed: false, topik_terlarang: '', notes: ''
		};
		showForm = true;
	}

	function openEdit(m: Media) {
		editingId = m.id;
		error = '';
		form = {
			name: m.name, website_url: m.website_url ?? '', categories: [...m.categories],
			media_type: (m as any).media_type ?? 'mainstream', price: m.price, traffic_estimate: m.traffic_estimate,
			integration_type: m.integration_type, publication_mode: m.publication_mode,
			pricing_type: (m as any).pricing_type ?? 'nett', max_links: m.max_links ?? 1,
			bahasa_artikel: m.bahasa_artikel.length ? [...m.bahasa_artikel] : ['Indonesia'],
			label_sponsor: m.label_sponsor, word_count_mode: (m as any).word_count_mode ?? 'sesuai_permintaan',
			allow_embed: Boolean((m as any).allow_embed), topik_terlarang: m.topik_terlarang.join(', '),
			notes: (m as any).notes ?? ''
		};
		showForm = true;
	}

	async function submitForm(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = {
				name: form.name.trim(),
				categories: form.categories,
				media_type: form.media_type,
				price: Number(form.price) || 0,
				traffic_estimate: Number(form.traffic_estimate) || 0,
				integration_type: form.integration_type,
				publication_mode: form.publication_mode,
				pricing_type: form.pricing_type,
				max_links: Number(form.max_links) || 1,
				bahasa_artikel: form.bahasa_artikel,
				label_sponsor: form.label_sponsor,
				word_count_mode: form.word_count_mode,
				allow_embed: form.allow_embed,
				topik_terlarang: form.topik_terlarang.split(',').map((s) => s.trim()).filter(Boolean),
				notes: form.notes.trim()
			};
			if (form.website_url.trim()) payload.website_url = form.website_url.trim();
			const res = editingId
				? await apiRequest(`/v1/partner/media/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) })
				: await apiRequest('/v1/partner/media', { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			showForm = false;
			notice = editingId ? 'Media diperbarui.' : 'Media diajukan untuk review admin.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function archive(m: Media) {
		if (!confirm(`Arsipkan media "${m.name}"? Media yang diarsipkan tidak tampil di katalog klien.`)) return;
		const res = await apiRequest(`/v1/partner/media/${m.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = 'Media diarsipkan.';
		await load();
	}

	async function resubmit(m: Media) {
		const res = await apiRequest(`/v1/partner/media/${m.id}/submit`, { method: 'POST' });
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = 'Media diajukan ulang untuk review admin.';
		await load();
	}

	async function uploadImport(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		importError = '';
		importResult = null;
		const body = new FormData();
		body.append('file', file);
		body.append('confirm', 'false');
		const res = await apiRequest('/v1/partner/media/import', { method: 'POST', body });
		if (!res.ok) {
			importError = await readApiError(res, "Permintaan gagal.");
			input.value = '';
			return;
		}
		importResult = await res.json();
		input.value = '';
	}

	async function confirmImport() {
		if (!importResult) return;
		busy = true;
		importError = '';
		try {
			const rows = importResult.rows ?? [];
			// Bangun ulang file tidak mungkin di klien — server butuh file asli,
			// jadi konfirmasi dilakukan dengan mengunggah ulang file yang sama.
			const input = document.getElementById('bulk-file') as HTMLInputElement | null;
			const file = input?.files?.[0];
			if (!file) {
				importError = 'Pilih ulang file XLSX untuk mengonfirmasi impor.';
				return;
			}
			const body = new FormData();
			body.append('file', file);
			body.append('confirm', 'true');
			const res = await apiRequest('/v1/partner/media/import', { method: 'POST', body });
			const data = await res.json();
			if (!res.ok) {
				importError = data?.detail ?? 'Impor gagal.';
				return;
			}
			importResult = data;
			notice = `${data.imported} media berhasil diimpor.`;
			await load();
		} finally {
			busy = false;
		}
	}

	function toggleCategory(name: string) {
		if (form.categories.includes(name)) {
			form.categories = form.categories.filter((c) => c !== name);
			return;
		}
		if (form.categories.length >= 20) {
			error = 'Maksimal 20 kategori per media.';
			return;
		}
		form.categories = [...form.categories, name];
	}

	const statusLabel: Record<string, string> = {
		active: 'Aktif', inactive: 'Nonaktif', draft: 'Draf', archived: 'Diarsipkan',
		pending_review: 'Menunggu review', rejected: 'Ditolak'
	};
	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID')}`;
</script>

<PageTitle title="Daftar Media" />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Daftar Media</h1>
			<p class="mt-1 text-sm text-muted-foreground">{items.length} media terdaftar pada akun mitra Anda.</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<label class="app-btn app-btn-soft cursor-pointer">
				<Upload size={15} /> Impor XLSX
				<input id="bulk-file" class="hidden" type="file" accept=".xlsx" onchange={uploadImport} />
			</label>
			<button class="app-btn app-btn-primary" onclick={openCreate}><Plus size={15} /> Tambah Media</button>
		</div>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if importError}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{importError}</div>
	{/if}

	{#if importResult}
		<div class="app-card mt-3 p-4">
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-sm font-semibold">
						{importResult.mode === 'preview' ? 'Pratinjau Impor' : 'Hasil Impor'}
					</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">
						{importResult.total} baris · {importResult.validCount ?? importResult.imported ?? 0} valid
						{#if importResult.invalidCount}· {importResult.invalidCount} bermasalah{/if}
						{#if importResult.failed}· {importResult.failed} gagal{/if}
					</p>
				</div>
				<div class="flex gap-2">
					{#if importResult.mode === 'preview' && importResult.invalidCount === 0}
						<button class="app-btn app-btn-primary" disabled={busy} onclick={confirmImport}>Impor {importResult.validCount} media</button>
					{/if}
					<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Tutup hasil impor" onclick={() => (importResult = null)}><X size={16} /></button>
				</div>
			</div>

			{#if Array.isArray(importResult.rows)}
				<div class="mt-3 max-h-72 overflow-y-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="py-1.5 pr-3 font-medium">Baris</th>
								<th class="py-1.5 pr-3 font-medium">Nama</th>
								<th class="py-1.5 pr-3 font-medium">Kategori</th>
								<th class="py-1.5 pr-3 text-right font-medium">Harga</th>
								<th class="py-1.5 font-medium">Catatan</th>
							</tr>
						</thead>
						<tbody>
							{#each importResult.rows as r (r.rowNumber)}
								<tr class="border-t">
									<td class="py-1.5 pr-3 tabular-nums">{r.rowNumber}</td>
									<td class="py-1.5 pr-3">{r.parsed?.name ?? '—'}</td>
									<td class="py-1.5 pr-3">{r.parsed?.category ?? '—'}</td>
									<td class="py-1.5 pr-3 text-right tabular-nums">{rp(r.parsed?.price ?? 0)}</td>
									<td class="py-1.5">
										{#if r.errors?.length}
											<span style="color: var(--destructive);">{r.errors.join('; ')}</span>
										{:else if r.warnings?.length}
											<span class="text-muted-foreground">{r.warnings.join('; ')}</span>
										{:else}
											<span style="color: var(--success);">OK</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
			{#if Array.isArray(importResult.errors) && importResult.errors.length > 0}
				<ul class="mt-3 space-y-1 text-xs" style="color: var(--destructive);">
					{#each importResult.errors as e (e.rowNumber)}
						<li>Baris {e.rowNumber}: {e.detail}</li>
					{/each}
				</ul>
			{/if}
			{#if importResult.mode === 'preview'}
				<p class="mt-3 text-xs text-muted-foreground">
					File XLSX harus dipilih ulang saat menekan Impor (server memvalidasi ulang berkas aslinya).
				</p>
			{/if}
		</div>
	{/if}

	<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => e.preventDefault()}>
		<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari nama media atau domain…" bind:value={q} />
		</label>
		<Select
			bind:value={statusFilter}
			ariaLabel="Status media"
			placeholder="Semua status (tanpa arsip)"
			align="end"
			onchange={load}
			options={[
				{ value: '', label: 'Semua status (tanpa arsip)' },
				{ value: 'active', label: 'Aktif' },
				{ value: 'pending_review', label: 'Menunggu review' },
				{ value: 'rejected', label: 'Ditolak' },
				{ value: 'inactive', label: 'Nonaktif' },
				{ value: 'archived', label: 'Diarsipkan' }
			]}
		/>
	</form>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat media…</p>
	{:else if filtered.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{q || statusFilter ? 'Tidak ada media yang cocok dengan filter ini.' : 'Belum ada media. Tambahkan media pertama Anda.'}
			</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Media</th>
						<th class="px-4 py-2.5 font-medium">Kategori</th>
						<th class="px-4 py-2.5 text-right font-medium">Harga Mitra</th>
						<th class="px-4 py-2.5 text-right font-medium">Harga Klien</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 text-right font-medium">Terbit</th>
						<th class="px-4 py-2.5 text-right font-medium">Aksi</th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as m (m.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p class="font-medium">{m.name}</p>
								{#if m.website_url}
									<a class="inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={m.website_url} target="_blank" rel="noreferrer">
										{m.website_url.replace(/^https?:\/\//, '').slice(0, 40)} <ExternalLink size={11} />
									</a>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{m.categories.join(', ') || '—'}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(m.effective_price)}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(m.client_price)}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
									{statusLabel[m.status] ?? m.status}
								</span>
								{#if m.pending_publications > 0}
									<span class="ml-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning);">
										{m.pending_publications} menunggu
									</span>
								{/if}
								{#if m.dewan_pers_verified}
									<span class="ml-1 text-[11px]" style="color: var(--success);">Dewan Pers</span>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmt(m.total_terbit)}</td>
							<td class="px-4 py-2.5">
								<div class="flex justify-end gap-1">
									{#if m.status === 'rejected'}
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => resubmit(m)}>Ajukan ulang</button>
									{/if}
									<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={`Edit ${m.name}`} onclick={() => openEdit(m)}><Pencil size={15} /></button>
									<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={`Arsipkan ${m.name}`} onclick={() => archive(m)} disabled={m.status === 'archived'}><Trash2 size={15} /></button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

{#if showForm}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-8 w-full max-w-2xl p-5" aria-label={editingId ? 'Edit media' : 'Tambah media'}>
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-lg font-semibold">{editingId ? 'Edit Media' : 'Tambah Media'}</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Tutup formulir" onclick={() => (showForm = false)}><X size={18} /></button>
			</div>

			{#if error}<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>{/if}

			<form class="mt-4 space-y-4" onsubmit={submitForm}>
				<div class="grid gap-3 sm:grid-cols-2">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Nama Media *</span>
						<input class="app-input mt-1 w-full" required minlength="2" maxlength="160" bind:value={form.name} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Website / Domain</span>
						<input class="app-input mt-1 w-full" placeholder="https://media.example" bind:value={form.website_url} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Harga Mitra (IDR) *</span>
						<input class="app-input mt-1 w-full" type="number" min="0" required bind:value={form.price} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Estimasi Trafik</span>
						<input class="app-input mt-1 w-full" type="number" min="0" bind:value={form.traffic_estimate} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Jenis Media</span>
						<Select
							bind:value={form.media_type}
							ariaLabel="Jenis Media"
							placeholder="Media Mainstream"
							minWidth={220}
							options={[
								{ value: 'mainstream', label: 'Media Mainstream' },
								{ value: 'blogger', label: 'Media Blogger' }
							]}
						/>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Platform / Integrasi</span>
						<Select
							bind:value={form.integration_type}
							ariaLabel="Platform / Integrasi"
							placeholder="Manual"
							minWidth={220}
							options={[
								{ value: 'manual', label: 'Manual' },
								{ value: 'wordpress', label: 'WordPress' },
								{ value: 'blogger', label: 'Blogspot' },
								{ value: 'custom_web', label: 'Custom Web' },
								{ value: 'tulisio_api', label: 'Tulisio API' },
								{ value: 'webhook', label: 'Webhook' },
								{ value: 'rss', label: 'RSS' }
							]}
						/>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Mode Publikasi</span>
						<Select
							bind:value={form.publication_mode}
							ariaLabel="Mode Publikasi"
							placeholder="Draf (manual, mitra terbitkan)"
							minWidth={240}
							options={[
								{ value: 'draft', label: 'Draf (manual, mitra terbitkan)' },
								{ value: 'publish', label: 'Terbit otomatis' },
								{ value: 'rss', label: 'RSS' }
							]}
						/>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Sifat Harga</span>
						<Select
							bind:value={form.pricing_type}
							ariaLabel="Sifat Harga"
							placeholder="NETT"
							minWidth={140}
							options={[
								{ value: 'nett', label: 'NETT' },
								{ value: 'nego', label: 'NEGO' }
							]}
						/>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Maks. Link per Artikel</span>
						<input class="app-input mt-1 w-full" type="number" min="1" max="2" bind:value={form.max_links} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Panjang Artikel</span>
						<Select
							bind:value={form.word_count_mode}
							ariaLabel="Panjang Artikel"
							placeholder="Sesuai permintaan klien (bebas)"
							minWidth={240}
							options={[
								{ value: 'sesuai_permintaan', label: 'Sesuai permintaan klien (bebas)' },
								{ value: 'kata_300_700', label: '300–700 kata' },
								{ value: 'kata_800_1500', label: '800–1500 kata' }
							]}
						/>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Label Sponsor</span>
						<Select
							bind:value={form.label_sponsor}
							ariaLabel="Label Sponsor"
							placeholder="Tidak ada"
							minWidth={180}
							options={[
								{ value: 'tidak_ada', label: 'Tidak ada' },
								{ value: 'ada_sponsor', label: 'Ada sponsor' }
							]}
						/>
					</label>
				</div>

				<div>
					<span class="text-xs font-medium text-muted-foreground">Bahasa Artikel *</span>
					<div class="mt-1 flex flex-wrap gap-2">
						{#each ['Indonesia', 'Inggris', 'China', 'Arab'] as lang (lang)}
							<button
								type="button"
								class="app-btn !min-h-8 !px-3 !text-xs"
								class:app-btn-primary={form.bahasa_artikel.includes(lang)}
								class:app-btn-soft={!form.bahasa_artikel.includes(lang)}
								onclick={() => {
									form.bahasa_artikel = form.bahasa_artikel.includes(lang)
										? form.bahasa_artikel.filter((l) => l !== lang)
										: [...form.bahasa_artikel, lang];
								}}
							>{lang}</button>
						{/each}
					</div>
				</div>

				<div>
					<span class="text-xs font-medium text-muted-foreground">Kategori Media</span>
					{#if categories.length > 0}
						<div class="mt-1 flex flex-wrap gap-2">
							{#each categories as c (c)}
								<button
									type="button"
									class="app-btn !min-h-8 !px-3 !text-xs"
									class:app-btn-primary={form.categories.includes(c)}
									class:app-btn-soft={!form.categories.includes(c)}
									onclick={() => toggleCategory(c)}
								>{c}</button>
							{/each}
						</div>
					{:else}
						<p class="mt-1 text-xs text-muted-foreground">Belum ada kategori tersimpan. Kategori dibuat dari media pertama Anda.</p>
					{/if}
					<input class="app-input mt-2 w-full" placeholder="Tambah kategori baru, pisahkan dengan koma" onchange={(e) => {
						const value = (e.currentTarget as HTMLInputElement).value;
						for (const name of value.split(',').map((s) => s.trim()).filter(Boolean)) toggleCategory(name);
						(e.currentTarget as HTMLInputElement).value = '';
					}} />
				</div>

				<div class="grid gap-3 sm:grid-cols-2">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Topik Terlarang</span>
						<input class="app-input mt-1 w-full" placeholder="judi, sara, rokok" bind:value={form.topik_terlarang} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Catatan Internal</span>
						<input class="app-input mt-1 w-full" bind:value={form.notes} />
					</label>
				</div>

				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={form.allow_embed} />
					Izinkan embed video/peta di artikel
				</label>

				<div class="flex justify-end gap-2 border-t pt-4">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (showForm = false)}>Batal</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy || form.bahasa_artikel.length === 0}>
						{busy ? 'Menyimpan…' : editingId ? 'Simpan Perubahan' : 'Ajukan Media'}
					</button>
				</div>
				{#if autoPublishEnabled}
					<p class="text-xs text-muted-foreground">Mode otomatis aktif untuk akun Anda.</p>
				{/if}
			</form>
		</dialog>
	</div>
{/if}
