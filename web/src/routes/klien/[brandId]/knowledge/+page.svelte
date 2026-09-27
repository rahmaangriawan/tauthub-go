<script lang="ts">
	// Fase 3: knowledge collections + sources (port section knowledge).
	// Chat UI menyusul fase 4 bersama route chat (dependensi runContentJob).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Collection = { id: string; name: string; description: string | null; created_at: string };
	type Source = {
		id: string;
		title: string;
		source_type: string;
		original_filename: string | null;
		status: string | null;
		error_message: string | null;
		created_at: string;
	};

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let collections = $state<Collection[]>([]);
	let activeCollection = $state<string | null>(null);
	let sources = $state<Source[]>([]);
	let loading = $state(true);

	let newCollectionName = $state('');
	let newSourceTitle = $state('');
	let newSourceContent = $state('');
	let sitemapUrl = $state('');
	let busy = $state(false);
	let msg = $state('');
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadCollections();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadCollections() {
		const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections`);
		if (res.ok) collections = await res.json();
	}

	async function loadSources(collectionId: string) {
		activeCollection = collectionId;
		sources = [];
		const res = await apiRequest(
			`/v1/brands/${brandId}/knowledge/collections/${collectionId}/sources`
		);
		if (res.ok) {
			const data = await res.json();
			sources = data.sources ?? [];
		}
	}

	async function createCollection(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/knowledge/collections`, {
				method: 'POST',
				body: JSON.stringify({ name: newCollectionName.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal membuat koleksi.');
				return;
			}
			newCollectionName = '';
			await loadCollections();
		} finally {
			busy = false;
		}
	}

	async function addSource(e: SubmitEvent) {
		e.preventDefault();
		if (!activeCollection) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(
				`/v1/brands/${brandId}/knowledge/collections/${activeCollection}/sources`,
				{ method: 'POST', body: JSON.stringify({ title: newSourceTitle.trim(), content: newSourceContent }) }
			);
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menambah sumber.');
				return;
			}
			newSourceTitle = '';
			newSourceContent = '';
			msg = 'Sumber ditambahkan & di-chunk + embedding.';
			await loadSources(activeCollection);
		} finally {
			busy = false;
		}
	}

	async function startCrawl(e: SubmitEvent) {
		e.preventDefault();
		if (!activeCollection) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/knowledge/sitemap`, {
				method: 'POST',
				body: JSON.stringify({
					collection_id: activeCollection,
					sitemap_url: sitemapUrl.trim(),
					max_pages: 50
				})
			});
			const data = await res.json();
			if (!res.ok) {
				error = typeof data.detail === 'string' ? data.detail : 'Gagal memulai crawl.';
				return;
			}
			msg = `Crawl diantrekan (run ${data.run_id}). Import berjalan latar belakang.`;
		} finally {
			busy = false;
		}
	}

	async function deleteSource(sourceId: string) {
		busy = true;
		try {
			await apiRequest(`/v1/brands/${brandId}/knowledge/sources/${sourceId}`, { method: 'DELETE' });
			if (activeCollection) await loadSources(activeCollection);
		} finally {
			busy = false;
		}
	}
</script>

<section class="mx-auto max-w-4xl px-4 py-8">
	<a class="text-sm text-muted-foreground underline" href={`/klien/${brandUrl}`}>← Dashboard</a>
	<h1 class="mt-2 text-2xl font-semibold tracking-tight">Knowledge Base</h1>

	<form class="mt-4 flex gap-2" onsubmit={createCollection}>
		<input
			class="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm"
			placeholder="Nama koleksi baru (min. 2 karakter)"
			bind:value={newCollectionName}
			required
			minlength={2}
		/>
		<button class="app-btn app-btn-primary" disabled={busy} type="submit">
			Buat koleksi
		</button>
	</form>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat…</p>
	{:else}
		<div class="mt-6 grid gap-4 md:grid-cols-2">
			<section class="app-card p-4">
				<h2 class="font-medium">Koleksi</h2>
				<ul class="mt-2 text-sm">
					{#each collections as c (c.id)}
						<li>
							<button
								class="w-full rounded px-2 py-1.5 text-left hover:bg-accent {activeCollection === c.id ? 'bg-accent' : ''}"
								onclick={() => loadSources(c.id)}
							>
								{c.name}
								{#if c.description}<span class="block text-xs text-muted-foreground">{c.description}</span>{/if}
							</button>
						</li>
					{:else}
						<li class="text-muted-foreground">Belum ada koleksi.</li>
					{/each}
				</ul>
			</section>

			{#if activeCollection}
				<section class="app-card p-4">
					<h2 class="font-medium">Sumber</h2>
					<ul class="mt-2 text-sm">
						{#each sources as s (s.id)}
							<li class="flex items-start justify-between gap-2 border-b border-border/50 py-1.5 last:border-0">
								<div>
									<span class="font-medium">{s.title}</span>
									<span class="block text-xs text-muted-foreground">
										{s.source_type} · {s.status}
									</span>
								</div>
								<button
									class="text-xs text-destructive underline"
									disabled={busy}
									onclick={() => deleteSource(s.id)}
								>
									Hapus
								</button>
							</li>
						{:else}
							<li class="text-muted-foreground">Belum ada sumber.</li>
						{/each}
					</ul>
				</section>
			{/if}
		</div>

		{#if activeCollection}
			<form class="app-card mt-4 p-4" onsubmit={addSource}>
				<h2 class="font-medium">Tambah sumber manual</h2>
				<input
					class="mt-2 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
					placeholder="Judul" bind:value={newSourceTitle} required minlength={2}
				/>
				<textarea
					class="mt-2 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
					placeholder="Isi konten (min. 20 karakter — akan di-chunk 1200 + embedding 128-dim)"
					bind:value={newSourceContent} required minlength={20} rows={4}
				></textarea>
				<button class="mt-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground" disabled={busy} type="submit">
					Tambah sumber
				</button>
			</form>

			<form class="app-card mt-4 p-4" onsubmit={startCrawl}>
				<h2 class="font-medium">Import via sitemap</h2>
				<input
					class="mt-2 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
					placeholder="https://contoh.com/sitemap.xml (maks 50 halaman)"
					bind:value={sitemapUrl} required
				/>
				<button class="mt-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground" disabled={busy} type="submit">
					Mulai crawl
				</button>
			</form>
		{/if}

		{#if msg}<p class="mt-3 text-sm text-emerald-600">{msg}</p>{/if}
		{#if error}<p class="mt-3 text-sm text-destructive">{error}</p>{/if}
	{/if}
</section>
