<script lang="ts">
	// Portal mitra — pintu masuk: resolusi partner user lalu arahkan ke
	// /mitra/{public_id} (parity app lama: public_id 6 karakter di URL).
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let error = $state('');
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				await goto('/login?returnTo=%2Fmitra');
				return;
			}
			const res = await apiRequest('/v1/partner/me');
			if (!res.ok) {
				error = 'Akun ini tidak terhubung ke data mitra. Hubungi admin Tauthub.';
				loading = false;
				return;
			}
			const data = await res.json();
			const partner = data?.partner;
			if (!partner) {
				error = 'Data mitra belum tersedia untuk akun ini.';
				loading = false;
				return;
			}
			await goto(`/mitra/${partner.public_id ?? partner.id}`);
		})();
		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});
</script>

<section>
	<div class="app-card p-6 text-center">
		<h1 class="text-lg font-semibold">Portal Mitra Tauthub</h1>
		{#if loading}
			<p class="mt-2 text-sm text-muted-foreground">Memuat data mitra…</p>
		{:else if error}
			<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>
			<a class="app-btn app-btn-soft mt-4" href="/klien">Buka area klien</a>
		{/if}
	</div>
</section>
