<script lang="ts">
	// Port app/blogger-pick-blog/page.tsx — pilih blog Blogger setelah OAuth
	// (`?media={mediaId}`). Menyimpan pilihan lalu kembali ke portal mitra.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { CheckCircle2, Globe } from '@lucide/svelte';
	import { apiRequest, authErrorMessage, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';

	type Blog = { id: string; name: string; description?: string; url: string; post_count?: string };

	const mediaId = $derived(page.url.searchParams.get('media') ?? '');

	let blogs = $state<Blog[]>([]);
	let selected = $state<string | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				window.location.href = '/login';
				return;
			}
			await fetchBlogs();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function fetchBlogs() {
		if (!mediaId) {
			error = 'Media ID tidak ditemukan.';
			loading = false;
			return;
		}
		const res = await apiRequest(`/v1/partner/media/${mediaId}/blogger/blogs`);
		if (!res.ok) {
			error = await authErrorMessage(new Error(`Gagal memuat blog (${res.status})`));
			loading = false;
			return;
		}
		const data = (await res.json()) as { blogs: Blog[] };
		blogs = data.blogs ?? [];
		if (blogs.length === 1) selected = blogs[0].id;
		loading = false;
	}

	async function save() {
		if (!mediaId || !selected) return;
		saving = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${mediaId}/blogger`, {
				method: 'POST',
				body: JSON.stringify({ blog_id: selected })
			});
			if (!res.ok) throw new Error('Gagal menyimpan blog.');
			const meRes = await apiRequest('/v1/partner/me');
			const meData = (await meRes.json().catch(() => ({}))) as { partner?: { public_id?: string; id?: string } };
			const pid = getPartnerUrlId() || meData.partner?.public_id || meData.partner?.id || '';
			window.location.href = `/mitra/${pid}/blogger`;
		} catch (reason) {
			error = authErrorMessage(reason);
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Pilih Blog — Tauthub</title>
</svelte:head>

<main class="flex min-h-screen items-center justify-center bg-background p-6">
	<div class="app-card w-full max-w-[520px] p-6">
		<h1 class="text-xl font-semibold tracking-tight">Pilih Blog</h1>
		<p class="mt-1 text-sm text-muted-foreground">Pilih blog Blogger yang ingin diintegrasikan dengan Tauthub.</p>

		{#if error}
			<p class="mt-3 rounded-md px-3 py-2 text-sm" style="background: var(--destructive-soft, #fdecee); color: var(--destructive);">{error}</p>
		{/if}

		{#if loading}
			<p class="mt-4 text-sm text-muted-foreground">Memuat daftar blog…</p>
		{:else if blogs.length === 0 && !error}
			<div class="mt-6 flex flex-col items-center gap-2 py-6 text-center">
				<Globe size={32} class="text-muted-foreground" />
				<p class="text-sm text-muted-foreground">Tidak ada blog ditemukan di akun Google ini.</p>
			</div>
		{:else}
			<div class="mt-4 flex flex-col gap-2">
				{#each blogs as b (b.id)}
					<button
						type="button"
						class="flex items-start gap-2.5 rounded-[10px] px-3.5 py-3 text-left transition-colors"
						style="border: 1px solid {selected === b.id ? 'var(--primary)' : 'var(--border)'}; background: {selected === b.id ? 'var(--accent)' : 'var(--card)'};"
						onclick={() => (selected = b.id)}
					>
						<span class="mt-0.5 shrink-0" style="color: {selected === b.id ? 'var(--primary)' : 'var(--muted-foreground)'};">
							{#if selected === b.id}<CheckCircle2 size={18} />{:else}<Globe size={18} />{/if}
						</span>
						<span class="min-w-0 flex-1">
							<span class="flex items-center gap-2">
								<span class="text-sm font-medium">{b.name}</span>
								{#if b.post_count}
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">{b.post_count} postingan</span>
								{/if}
							</span>
							{#if b.description}<span class="block truncate text-xs text-muted-foreground">{b.description}</span>{/if}
							<span class="block truncate text-xs text-muted-foreground">{b.url}</span>
						</span>
					</button>
				{/each}
			</div>

			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (window.location.href = '/mitra')}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" disabled={!selected || saving} onclick={save}>
					{saving ? 'Menyimpan…' : 'Hubungkan Blog'}
				</button>
			</div>
		{/if}
	</div>
</main>
