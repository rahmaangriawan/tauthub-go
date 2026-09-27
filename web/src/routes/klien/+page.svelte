<script lang="ts">
	// Fase 2: daftar brand + create (port section klien index).
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiRequest, readApiError, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';

	type BrandRow = {
		id: string;
		name: string;
		slug: string;
		public_id: string | null;
		status: string;
		role: string;
		ownership_type: string;
		industry: string | null;
		readiness_score: number;
		websites: Array<{ domain: string; is_primary: boolean }>;
	};

	const brandList = swr<BrandRow[]>(
		() => (getStoredSession() ? '/v1/brands' : null),
		async () => {
			const res = await apiRequest('/v1/brands');
			if (!res.ok) throw new Error('Gagal memuat brand');
			return res.json();
		}
	);
	let loading = $state(true);
	let showCreate = $state(false);
	let busy = $state(false);
	let error = $state('');
	let form = $state({ name: '', slug: '', ownership_type: 'internal', client_name: '', industry: '', domain: '' });

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				await goto('/login');
				return;
			}
			loading = false;
		})();
		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = {
				name: form.name.trim(),
				slug: form.slug.trim(),
				ownership_type: form.ownership_type,
				primary_language: 'id-ID'
			};
			if (form.client_name.trim()) payload.client_name = form.client_name.trim();
			if (form.industry.trim()) payload.industry = form.industry.trim();
			if (form.domain.trim()) payload.domain = form.domain.trim();
			const res = await apiRequest('/v1/brands', {
				method: 'POST',
				body: JSON.stringify(payload)
			});
			if (res.ok) {
				showCreate = false;
				form = { name: '', slug: '', ownership_type: 'internal', client_name: '', industry: '', domain: '' };
				await brandList.mutate(); // invalidasi cache + revalidasi (parity)
			} else {
				error = await readApiError(res, 'Gagal membuat brand.');
			}
		} finally {
			busy = false;
		}
	}
</script>

<main class="mx-auto max-w-5xl px-4 py-8">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-semibold tracking-tight">Brand Saya</h1>
		<button
			class="app-btn app-btn-primary"
			onclick={() => (showCreate = !showCreate)}
		>
			{showCreate ? 'Tutup' : '+ Brand Baru'}
		</button>
	</div>

	{#if showCreate}
		<form class="app-card mt-4 grid gap-3 p-4" onsubmit={create}>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="flex flex-col gap-1 text-sm">Nama
					<input class="app-input" bind:value={form.name} minlength={2} required />
				</label>
				<label class="flex flex-col gap-1 text-sm">Slug
					<input class="app-input" bind:value={form.slug} pattern="[a-z0-9]+(-[a-z0-9]+)*" required />
				</label>
				<label class="flex flex-col gap-1 text-sm">Tipe kepemilikan
					<select class="app-input" bind:value={form.ownership_type}>
						<option value="internal">Internal</option>
						<option value="client">Client</option>
					</select>
				</label>
				{#if form.ownership_type === 'client'}
					<label class="flex flex-col gap-1 text-sm">Nama klien
						<input class="app-input" bind:value={form.client_name} />
					</label>
				{/if}
				<label class="flex flex-col gap-1 text-sm">Industri
					<input class="app-input" bind:value={form.industry} />
				</label>
				<label class="flex flex-col gap-1 text-sm">Domain (opsional)
					<input class="app-input" bind:value={form.domain} />
				</label>
			</div>
			{#if error}<p class="text-sm text-destructive">{error}</p>{/if}
			<button class="app-btn app-btn-primary w-fit" disabled={busy} type="submit">
				{busy ? 'Menyimpan…' : 'Buat Brand'}
			</button>
		</form>
	{/if}

	{#if loading || (brandList.loading && brandList.data === null)}
		<p class="mt-6 text-sm text-muted-foreground">Memuat…</p>
	{:else if (brandList.data ?? []).length === 0}
		<p class="mt-6 text-sm text-muted-foreground">Belum ada brand. Buat brand pertama Anda.</p>
	{:else}
		<div class="mt-6 grid gap-3 sm:grid-cols-2">
			{#each brandList.data ?? [] as brand (brand.id)}
				<a
					class="app-card p-4 transition-colors hover:bg-accent"
					href={`/klien/${brand.id}`}
				>
					<div class="flex items-center justify-between">
						<span class="font-medium">{brand.name}</span>
						<span class="rounded-full px-2 py-0.5 text-xs {brand.status === 'active' ? 'bg-emerald-100 text-emerald-700' : 'bg-muted text-muted-foreground'}">
							{brand.status}
						</span>
					</div>
					<p class="mt-1 text-sm text-muted-foreground">
						/{brand.slug} · {brand.ownership_type}{#if brand.industry} · {brand.industry}{/if}
					</p>
					<div class="mt-3 flex items-center gap-2">
						<div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
							<div class="h-full rounded-full bg-primary" style="width:{brand.readiness_score}%"></div>
						</div>
						<span class="text-xs text-muted-foreground">readiness {brand.readiness_score}%</span>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</main>
