<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// /profiler bukan lagi halaman daftar — daftar brief sekarang HIDUP DI SIDEBAR
	// workspace chat. Klik menu/button "Profiler" harus langsung mendarat di
	// chatbot: buka brief yang paling baru disentuh, atau buat yang pertama
	// kalau brand ini belum punya. Halaman ini cuma perutean, tidak ada UI.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { LoaderCircle } from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);

	let error = $state('');

	type ProfileRow = { id: string; short_code: string | null };

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await route();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function route() {
		const res = await apiRequest(`${apiBase}/profiles`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat daftar brief.'));
			return;
		}
		const data = (await res.json()) as { profiles?: Array<{ id: string; short_code: string | null }> };
		const rows = data.profiles ?? [];
		if (rows.length === 0) {
			const created = await apiRequest(`${apiBase}/profiles`, {
				method: 'POST',
				body: JSON.stringify({ name: tr('Brief pertama') })
			});
			if (!created.ok) {
				error = await readApiError(created, tr('Gagal membuat brief pertama.'));
				return;
			}
			const item = (await created.json()) as { id: string; short_code: string | null };
			await goto(target(item.id, item.short_code), { replaceState: true });
			return;
		}
		// Server memilah updated_at DESC, jadi baris pertama = brief terakhir disentuh.
		const last = rows[0];
		await goto(target(last.id, last.short_code), { replaceState: true });
	}

	function target(id: string, shortCode: string | null) {
		return `/klien/${brandUrl}/profiler/${shortCode || id}`;
	}
</script>

<PageTitle title="Profiler" />

<div class="flex items-center gap-2 py-10 text-sm text-muted-foreground">
	{#if error}
		<span style="color: var(--destructive);">{error}</span>
		<a class="app-btn app-btn-soft ml-2" href={`/klien/${brandUrl}/campaigns`}>{tr('Kembali ke kampanye')}</a>
	{:else}
		<LoaderCircle size={18} class="animate-spin" />
		<span>{tr('Membuka brief terakhir…')}</span>
	{/if}
</div>
