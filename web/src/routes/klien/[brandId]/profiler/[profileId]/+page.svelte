<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Editor satu Profiler — workspace knowledge terstruktur milik klien.
	// Tab: Ringkasan, Identitas Brand, Gaya Penulisan, Pedoman Konten & SEO,
	// Knowledge & Referensi (Dokumen/Gambar/URL/Blok), Instruksi Khusus.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, FileText, Image as ImageIcon, Link2, Plus, RefreshCw, Save, Trash2, Upload } from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';

	type Profile = {
		id: string;
		name: string;
		description: string | null;
		identity: Record<string, any>;
		style: Record<string, any>;
		seo: Record<string, any>;
		instructions: string | null;
		updated_at: string | null;
		blocks: Array<{ id: string; title: string; kind: string | null; content: string | null; note: string | null }>;
		documents: Array<{ id: string; original_filename: string; mime_type: string | null; size_bytes: number | null; parse_status: string; parse_error: string | null; created_at: string; url?: string }>;
		images: Array<{ id: string; url: string; file_name: string | null; note: string | null }>;
		urls: Array<{ id: string; url: string; title: string | null; note: string | null; fetch_status: string; fetch_error: string | null; last_synced_at: string | null }>;
	};

	const brandId = $derived(page.params.brandId as string);
	const profileId = $derived(page.params.profileId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);

	let profile = $state<Profile | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);

	let tab = $state<'ringkasan' | 'identitas' | 'gaya' | 'seo' | 'knowledge' | 'instruksi'>('ringkasan');
	let knowTab = $state<'dokumen' | 'gambar' | 'url' | 'blok'>('dokumen');

	let newBlock = $state({ title: '', kind: '', content: '' });
	let editingBlockId = $state<string | null>(null);
	let newUrl = $state({ url: '', title: '', note: '' });
	let urlBusy = $state<string | null>(null);
	let uploadBusy = $state(false);
	let docInput = $state<HTMLInputElement | undefined>(undefined);
	let imgInput = $state<HTMLInputElement | undefined>(undefined);

	const IDENTITY_FIELDS: Array<[string, string, 'input' | 'area']> = [
		['brand_name', 'Nama brand / perusahaan', 'input'],
		['industry', 'Industri / bidang bisnis', 'input'],
		['business_description', 'Deskripsi bisnis', 'area'],
		['products', 'Produk & layanan utama', 'area'],
		['audience', 'Target audiens', 'input'],
		['problems_solved', 'Masalah yang diselesaikan', 'area'],
		['usp', 'Keunggulan / USP', 'area'],
		['values', 'Nilai brand', 'input'],
		['vision', 'Visi', 'input'],
		['mission', 'Misi', 'input'],
		['region', 'Wilayah operasional', 'input'],
		['website', 'Website resmi', 'input'],
		['contact', 'Kontak bisnis', 'input'],
		['key_facts', 'Fakta penting yang wajib dipertahankan', 'area']
	];
	const STYLE_FIELDS: Array<[string, string, 'input' | 'area']> = [
		['tone', 'Gaya bahasa (formal/santai/teknis/persuasif/jurnalistik/kustom)', 'input'],
		['brand_character', 'Karakter brand', 'input'],
		['formality', 'Tingkat formalitas', 'input'],
		['greeting', 'Sapaan kepada pembaca', 'input'],
		['point_of_view', 'Sudut pandang penulisan', 'input'],
		['depth', 'Kedalaman pembahasan', 'input'],
		['length_preference', 'Preferensi panjang artikel', 'input'],
		['language', 'Bahasa utama', 'input'],
		['preferred_terms', 'Istilah yang harus digunakan', 'area'],
		['forbidden_terms', 'Kata/frasa yang dilarang', 'area'],
		['eyd_guideline', 'Pedoman EYD/PUEBI', 'area'],
		['opening_preference', 'Preferensi pembukaan', 'input'],
		['closing_preference', 'Preferensi penutup', 'input']
	];
	const SEO_FIELDS: Array<[string, string, 'input' | 'area']> = [
		['article_goal', 'Tujuan artikel', 'input'],
		['search_intent', 'Search intent', 'input'],
		['primary_keyword', 'Keyword utama', 'input'],
		['secondary_keywords', 'Keyword sekunder', 'area'],
		['required_topics', 'Topik wajib dibahas', 'area'],
		['forbidden_topics', 'Topik yang dihindari', 'area'],
		['heading_structure', 'Struktur heading', 'input'],
		['use_tables', 'Preferensi tabel', 'input'],
		['use_lists', 'Preferensi daftar', 'input'],
		['use_faq', 'Preferensi FAQ', 'input'],
		['use_examples', 'Preferensi contoh', 'input'],
		['internal_link_rules', 'Aturan internal link', 'area'],
		['brand_mention_rules', 'Aturan penyebutan brand/produk', 'area'],
		['meta_title_preference', 'Preferensi meta title', 'input'],
		['meta_description_preference', 'Preferensi meta description', 'input'],
		['allowed_cta', 'CTA yang diizinkan', 'input'],
		['ideal_references', 'Referensi artikel ideal', 'area']
	];

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
		const res = await apiRequest(`${apiBase}/profiles/${profileId}`);
		if (!res.ok) {
			error = await readApiError(res, tr('Profiler tidak ditemukan.'));
			return;
		}
		profile = await res.json();
	}

	async function saveSection(keys: string[], section: 'identity' | 'style' | 'seo') {
		if (!profile) return;
		const obj: Record<string, any> = {};
		for (const k of keys) obj[k] = profile[section][k] ?? '';
		busy = true;
		notice = '';
		try {
			const res = await apiRequest(`${apiBase}/profiles/${profileId}`, { method: 'PUT', body: JSON.stringify({ [section]: obj }) });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan.'));
				return;
			}
			notice = tr('Tersimpan.');
		} finally {
			busy = false;
		}
	}

	async function saveInstructions() {
		if (!profile) return;
		busy = true;
		notice = '';
		try {
			const res = await apiRequest(`${apiBase}/profiles/${profileId}`, {
				method: 'PUT',
				body: JSON.stringify({ instructions: profile.instructions ?? '' })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan.'));
				return;
			}
			notice = tr('Tersimpan.');
		} finally {
			busy = false;
		}
	}

	async function uploadDoc(file: File) {
		uploadBusy = true;
		error = '';
		try {
			const form = new FormData();
			form.append('file', file);
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/documents`, { method: 'POST', body: form }, 120_000);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengunggah dokumen.'));
				return;
			}
			await load();
		} finally {
			uploadBusy = false;
			if (docInput) docInput.value = '';
		}
	}

	async function uploadImage(file: File, note = '') {
		uploadBusy = true;
		error = '';
		try {
			const form = new FormData();
			form.append('file', file);
			form.append('note', note);
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/images`, { method: 'POST', body: form }, 120_000);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengunggah gambar.'));
				return;
			}
			await load();
		} finally {
			uploadBusy = false;
			if (imgInput) imgInput.value = '';
		}
	}

	/** Simpan catatan/deskripsi opsional untuk satu gambar referensi. */
	async function saveImageNote(im: Profile['images'][number]) {
		const res = await apiRequest(`${apiBase}/images/${im.id}`, {
			method: 'PUT',
			body: JSON.stringify({ note: im.note ?? '' })
		});
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menyimpan catatan gambar.'));
			return;
		}
		notice = tr('Tersimpan.');
	}

	async function deleteChild(kind: 'documents' | 'images' | 'urls', id: string) {
		const res = await apiRequest(`${apiBase}/${kind}/${id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menghapus.'));
			return;
		}
		await load();
	}

	async function addBlock(e: SubmitEvent) {
		e.preventDefault();
		if (!newBlock.title.trim()) return;
		const res = await apiRequest(`${apiBase}/profiles/${profileId}/blocks`, {
			method: 'POST',
			body: JSON.stringify(newBlock)
		});
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menambah blok.'));
			return;
		}
		newBlock = { title: '', kind: '', content: '' };
		await load();
	}

	async function saveBlock(b: Profile['blocks'][number]) {
		const res = await apiRequest(`${apiBase}/blocks/${b.id}`, {
			method: 'PUT',
			body: JSON.stringify({ title: b.title, kind: b.kind ?? '', content: b.content ?? '', note: b.note ?? '' })
		});
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menyimpan blok.'));
			return;
		}
		editingBlockId = null;
	}

	async function deleteBlock(id: string) {
		const res = await apiRequest(`${apiBase}/blocks/${id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menghapus blok.'));
			return;
		}
		await load();
	}

	async function addUrl(e: SubmitEvent) {
		e.preventDefault();
		if (!newUrl.url.trim()) return;
		urlBusy = 'add';
		try {
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/urls`, { method: 'POST', body: JSON.stringify(newUrl) }, 60_000);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menambah referensi URL.'));
				return;
			}
			newUrl = { url: '', title: '', note: '' };
			await load();
		} finally {
			urlBusy = null;
		}
	}

	async function syncUrl(id: string) {
		urlBusy = id;
		try {
			const res = await apiRequest(`${apiBase}/urls/${id}/sync`, { method: 'POST' }, 60_000);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal sinkron ulang.'));
				return;
			}
			await load();
		} finally {
			urlBusy = null;
		}
	}

	const fmtSize = (n: number | null) => (n ? `${(n / 1024).toFixed(0)} KB` : '—');
	const STATUS_LABEL: Record<string, string> = { pending: 'Menunggu', parsing: 'Diproses', ready: 'Siap', failed: 'Gagal', fetching: 'Mengambil' };
</script>

<PageTitle title={profile ? `Profiler — ${profile.name}` : tr('Profiler')} />

<section>
	<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => void goto(`/klien/${brandUrl}/profiler`)}>
		<ArrowLeft size={14} /> {tr('Semua Profiler')}
	</button>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat…')}</p>
	{:else if !profile}
		<div class="app-card mt-4 p-6 text-sm" style="color: var(--destructive);">{error || tr('Profiler tidak ditemukan.')}</div>
	{:else}
		<header class="mt-3">
			<h1 class="text-xl font-bold">{profile.name}</h1>
			{#if profile.description}<p class="mt-1 text-sm text-muted-foreground">{profile.description}</p>{/if}
		</header>

		{#if error}
			<p class="mt-3 rounded-lg px-3 py-2 text-sm" style="background: var(--destructive-soft, #fdecee); color: var(--destructive);">{error}</p>
		{/if}
		{#if notice}
			<p class="mt-3 text-sm" style="color: var(--success);">{notice}</p>
		{/if}

		<nav class="mt-4 flex flex-wrap gap-1.5 border-b pb-2" style="border-color: var(--border);" aria-label={tr('Bagian Profiler')}>
			{#each [['ringkasan','Ringkasan'],['identitas','Identitas Brand'],['gaya','Gaya Penulisan'],['seo','Pedoman Konten & SEO'],['knowledge','Knowledge & Referensi'],['instruksi','Instruksi Khusus']] as [key, label] (key)}
				<button class="app-btn !min-h-8 !px-3 !text-xs" type="button" class:app-btn-primary={tab === key} class:app-btn-soft={tab !== key} onclick={() => (tab = key as typeof tab)}>{tr(label)}</button>
			{/each}
		</nav>

		{#if tab === 'ringkasan'}
			<div class="app-card mt-4 p-4">
				<h2 class="text-sm font-semibold">{tr('Ringkasan')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">{tr('Ringkasan isi Profiler ini. Isi tiap bagian lewat tab di atas.')}</p>
				<dl class="mt-3 grid gap-2 text-sm sm:grid-cols-2">
					<div><dt class="text-xs text-muted-foreground">{tr('Dokumen')}</dt><dd class="font-semibold">{profile.documents.length}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{tr('Gambar')}</dt><dd class="font-semibold">{profile.images.length}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{tr('Referensi URL')}</dt><dd class="font-semibold">{profile.urls.length}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{tr('Blok knowledge')}</dt><dd class="font-semibold">{profile.blocks.length}</dd></div>
				</dl>
			</div>
		{/if}

		{#if tab === 'identitas' || tab === 'gaya' || tab === 'seo'}
			{@const fields = tab === 'identitas' ? IDENTITY_FIELDS : tab === 'gaya' ? STYLE_FIELDS : SEO_FIELDS}
			{@const section: 'identity' | 'style' | 'seo' = tab === 'identitas' ? 'identity' : tab === 'gaya' ? 'style' : 'seo'}
			<div class="app-card mt-4 p-4">
				<div class="grid gap-3 sm:grid-cols-2">
					{#each fields as [key, label, kind] (key)}
						<label class="block" class:sm:col-span-2={kind === 'area'}>
							<span class="text-xs font-medium text-muted-foreground">{tr(label)}</span>
							{#if kind === 'area'}
								<textarea class="app-input mt-1 w-full" rows="2" bind:value={profile[section][key]}></textarea>
							{:else}
								<input class="app-input mt-1 w-full" bind:value={profile[section][key]} />
							{/if}
						</label>
					{/each}
				</div>
				<div class="mt-3 flex justify-end">
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveSection(fields.map((f) => f[0]), section)}>
						<Save size={15} /> {busy ? tr('Menyimpan…') : tr('Simpan')}
					</button>
				</div>
			</div>
		{/if}

		{#if tab === 'instruksi'}
			<div class="app-card mt-4 p-4">
				<h2 class="text-sm font-semibold">{tr('Instruksi Khusus')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">{tr('Ditulis bebas, disimpan terpisah dari field terstruktur. Contoh: "Jangan pakai pembukaan umum", "Utamakan contoh nyata industri kami".')}</p>
				<textarea class="app-input mt-3 w-full" rows="10" placeholder={tr('Tulis instruksi khusus untuk The Writer…')} bind:value={profile.instructions}></textarea>
				<div class="mt-3 flex justify-end">
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={saveInstructions}>
						<Save size={15} /> {busy ? tr('Menyimpan…') : tr('Simpan')}
					</button>
				</div>
			</div>
		{/if}

		{#if tab === 'knowledge'}
			<div class="mt-4 flex flex-wrap gap-1.5">
				{#each [['dokumen','Dokumen'],['gambar','Gambar'],['url','Referensi URL'],['blok','Knowledge Tambahan']] as [key, label] (key)}
					<button class="app-btn !min-h-8 !px-3 !text-xs" type="button" class:app-btn-primary={knowTab === key} class:app-btn-soft={knowTab !== key} onclick={() => (knowTab = key as typeof knowTab)}>
						{tr(label)} <span class="ml-1 rounded-full px-1.5 text-[10px]" style="background: var(--secondary);">{key === 'dokumen' ? profile.documents.length : key === 'gambar' ? profile.images.length : key === 'url' ? profile.urls.length : profile.blocks.length}</span>
					</button>
				{/each}
			</div>

			{#if knowTab === 'dokumen'}
				<div class="app-card mt-3 p-4">
					<div class="flex items-center justify-between gap-2">
						<h3 class="flex items-center gap-1.5 text-sm font-semibold"><FileText size={15} /> {tr('Dokumen Referensi')}</h3>
						<label class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs">
							<Upload size={13} /> {uploadBusy ? tr('Mengunggah…') : tr('Unggah Dokumen')}
							<input bind:this={docInput} type="file" class="hidden" accept=".pdf,.docx,.txt,.md,.csv" onchange={(e) => { const f = e.currentTarget.files?.[0]; if (f) void uploadDoc(f); }} />
						</label>
					</div>
					<p class="mt-1 text-[11px] text-muted-foreground">{tr('PDF, DOCX, TXT, MD, CSV (maks 10MB). Teks diekstrak di server; PDF hasil pemindaian tanpa lapisan teks akan ditandai gagal.')}</p>
					<ul class="mt-3 flex flex-col gap-1.5">
						{#each profile.documents as d (d.id)}
							<li class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2" style="border-color: var(--border);">
								<div class="min-w-0">
									<p class="truncate text-sm">{d.original_filename}</p>
									<p class="text-[11px] text-muted-foreground">{fmtSize(d.size_bytes)} · {tr(STATUS_LABEL[d.parse_status] ?? d.parse_status)}{d.parse_error ? ` — ${d.parse_error}` : ''}</p>
								</div>
								<div class="flex shrink-0 items-center gap-1">
									{#if d.url}<a class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" href={d.url} target="_blank" rel="noreferrer">{tr('Buka')}</a>{/if}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" style="color: var(--destructive);" onclick={() => void deleteChild('documents', d.id)}><Trash2 size={12} /></button>
								</div>
							</li>
						{/each}
						{#if profile.documents.length === 0}<li class="py-4 text-center text-xs text-muted-foreground">{tr('Belum ada dokumen.')}</li>{/if}
					</ul>
				</div>
			{/if}

			{#if knowTab === 'gambar'}
				<div class="app-card mt-3 p-4">
					<div class="flex items-center justify-between gap-2">
						<h3 class="flex items-center gap-1.5 text-sm font-semibold"><ImageIcon size={15} /> {tr('Gambar Referensi')}</h3>
						<label class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs">
							<Upload size={13} /> {uploadBusy ? tr('Mengunggah…') : tr('Unggah Gambar')}
							<input bind:this={imgInput} type="file" class="hidden" accept="image/*" onchange={(e) => { const f = e.currentTarget.files?.[0]; if (f) void uploadImage(f); }} />
						</label>
					</div>
					<p class="mt-1 text-[11px] text-muted-foreground">{tr('Logo, foto produk, infografik, screenshot (maks 10MB per file).')}</p>
					<div class="mt-3 grid gap-2 sm:grid-cols-3 lg:grid-cols-4">
						{#each profile.images as im (im.id)}
							<div class="rounded-lg border p-2" style="border-color: var(--border);">
								<img src={im.url} alt={im.file_name ?? ''} class="aspect-video w-full rounded object-cover" />
								<p class="mt-1 truncate text-[11px] text-muted-foreground">{im.file_name}</p>
								<input class="app-input mt-1 w-full !text-[11px]" placeholder={tr('Catatan (opsional)')} bind:value={im.note} onchange={() => void saveImageNote(im)} />
								<button class="app-btn app-btn-soft mt-1 w-full !min-h-7 !px-2 !text-[11px]" type="button" style="color: var(--destructive);" onclick={() => void deleteChild('images', im.id)}>{tr('Hapus')}</button>
							</div>
						{/each}
						{#if profile.images.length === 0}<p class="col-span-full py-4 text-center text-xs text-muted-foreground">{tr('Belum ada gambar.')}</p>{/if}
					</div>
				</div>
			{/if}

			{#if knowTab === 'url'}
				<div class="app-card mt-3 p-4">
					<h3 class="flex items-center gap-1.5 text-sm font-semibold"><Link2 size={15} /> {tr('Referensi URL')}</h3>
					<form class="mt-2 flex flex-wrap gap-2" onsubmit={addUrl}>
						<input class="app-input min-w-[240px] flex-1" placeholder="https://situs-resmi.com/halaman" bind:value={newUrl.url} />
						<input class="app-input min-w-[160px] flex-1" placeholder={tr('Judul (opsional)')} bind:value={newUrl.title} />
						<button class="app-btn app-btn-primary" type="submit" disabled={urlBusy === 'add'}><Plus size={14} /> {tr('Tambahkan')}</button>
					</form>
					<p class="mt-1 text-[11px] text-muted-foreground">{tr('Server mengambil isi halaman dengan aman (anti-SSRF). URL baru langsung dicoba diambil.')}</p>
					<ul class="mt-3 flex flex-col gap-1.5">
						{#each profile.urls as u (u.id)}
							<li class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2" style="border-color: var(--border);">
								<div class="min-w-0">
									<p class="truncate text-sm">{u.title || u.url}</p>
									<p class="truncate text-[11px] text-muted-foreground">{u.url} · {tr(STATUS_LABEL[u.fetch_status] ?? u.fetch_status)}{u.fetch_error ? ` — ${u.fetch_error}` : ''}</p>
								</div>
								<div class="flex shrink-0 items-center gap-1">
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={urlBusy === u.id} onclick={() => void syncUrl(u.id)}><RefreshCw size={12} /> {urlBusy === u.id ? '…' : tr('Sinkron')}</button>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" style="color: var(--destructive);" onclick={() => void deleteChild('urls', u.id)}><Trash2 size={12} /></button>
								</div>
							</li>
						{/each}
						{#if profile.urls.length === 0}<li class="py-4 text-center text-xs text-muted-foreground">{tr('Belum ada referensi URL.')}</li>{/if}
					</ul>
				</div>
			{/if}

			{#if knowTab === 'blok'}
				<div class="app-card mt-3 p-4">
					<h3 class="text-sm font-semibold">{tr('Knowledge Tambahan')}</h3>
					<form class="mt-3 grid gap-2 sm:grid-cols-3" onsubmit={addBlock}>
						<input class="app-input" placeholder={tr('Judul blok (mis. Daftar Harga)')} bind:value={newBlock.title} />
						<input class="app-input" placeholder={tr('Jenis (mis. FAQ, Harga)')} bind:value={newBlock.kind} />
						<button class="app-btn app-btn-primary" type="submit"><Plus size={14} /> {tr('Tambah')}</button>
						<textarea class="app-input sm:col-span-3" rows="2" placeholder={tr('Isi teks blok…')} bind:value={newBlock.content}></textarea>
					</form>
					<ul class="mt-3 flex flex-col gap-2">
						{#each profile.blocks as b (b.id)}
							<li class="rounded-lg border p-3" style="border-color: var(--border);">
								{#if editingBlockId === b.id}
									<div class="grid gap-2 sm:grid-cols-3">
										<input class="app-input" bind:value={b.title} />
										<input class="app-input" bind:value={b.kind} />
										<div class="flex gap-1"><button class="app-btn app-btn-primary !min-h-9 !px-3 !text-xs" type="button" onclick={() => void saveBlock(b)}>{tr('Simpan')}</button><button class="app-btn app-btn-soft !min-h-9 !px-3 !text-xs" type="button" onclick={() => (editingBlockId = null)}>{tr('Batal')}</button></div>
										<textarea class="app-input sm:col-span-3" rows="2" bind:value={b.content}></textarea>
									</div>
								{:else}
									<div class="flex items-start justify-between gap-2">
										<div class="min-w-0">
											<p class="text-sm font-semibold">{b.title}{b.kind ? ` · ${b.kind}` : ''}</p>
											<p class="mt-0.5 whitespace-pre-wrap text-xs text-muted-foreground">{b.content}</p>
										</div>
										<div class="flex shrink-0 gap-1">
											<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => (editingBlockId = b.id)}>{tr('Edit')}</button>
											<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" style="color: var(--destructive);" onclick={() => void deleteBlock(b.id)}><Trash2 size={12} /></button>
										</div>
									</div>
								{/if}
							</li>
						{/each}
						{#if profile.blocks.length === 0}<li class="py-4 text-center text-xs text-muted-foreground">{tr('Belum ada blok knowledge.')}</li>{/if}
					</ul>
				</div>
			{/if}
		{/if}
	{/if}
</section>
