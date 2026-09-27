<script lang="ts">
	// Port /login/google-callback — tukar ticket session Google → token pair,
	// lalu redirect per role (admin → /admin, partner → /mitra, klien → brand pertama).
	import { goto } from '$app/navigation';
	import { apiRequest, clearSession, getAccessToken, storeSession } from '$lib/auth';

	let error = $state('');

	function safeReturnTo(value: string | null, origin: string): string | null {
		if (!value || value.length > 4096) return null;
		try {
			const target = new URL(value, origin);
			if (target.origin !== origin || !value.startsWith('/')) return null;
			return `${target.pathname}${target.search}${target.hash}`;
		} catch {
			return null;
		}
	}

	$effect(() => {
		let cancelled = false;
		void (async () => {
			try {
				const response = await apiRequest('/v1/auth/google/exchange-session', { method: 'POST' });
				if (response.ok) {
					const data = await response.json();
					storeSession(data);
					if (getAccessToken()) {
						const returnTo = safeReturnTo(data.returnTo, window.location.origin);
						if (returnTo) {
							await goto(returnTo, { replaceState: true });
							return;
						}
						const role = data.user?.role;
						if (role === 'admin') {
							await goto('/admin', { replaceState: true });
						} else if (role === 'partner') {
							await goto('/mitra', { replaceState: true });
						} else {
							const brands = await apiRequest('/v1/brands');
							if (brands.ok) {
								const list = (await brands.json()) as Array<{ id: string; public_id?: string | null }>;
								if (list[0]) {
									await goto(`/klien/${list[0].public_id ?? list[0].id}`, { replaceState: true });
									return;
								}
							}
							await goto('/klien', { replaceState: true });
						}
						return;
					}
				}
				if (!cancelled) error = 'Sesi Google tidak valid atau kedaluwarsa. Silakan coba lagi.';
				clearSession();
			} catch {
				if (!cancelled) error = 'Gagal membuat sesi. Silakan coba lagi.';
			}
		})();
		return () => {
			cancelled = true;
		};
	});
</script>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="app-card w-full max-w-[420px] p-6" style="margin-top:10vh">
		{#if error}
			<p class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
			<a class="mt-3 inline-block rounded-md bg-muted px-3 py-2 text-sm" href="/login">Kembali ke Login</a>
		{:else}
			<p class="text-sm text-muted-foreground">Masuk dengan Google…</p>
		{/if}
	</div>
</main>
