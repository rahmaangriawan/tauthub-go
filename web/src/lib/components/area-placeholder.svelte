<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Placeholder shell fase 1 — porting section asli per fase di REWRITE/05.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		getStoredSession,
		clearSession,
		ensureValidSession,
		AUTH_EXPIRED_EVENT,
		type AuthUser
	} from '$lib/auth';

	let { area }: { area: 'klien' | 'mitra' | 'admin' } = $props();

	let user = $state<AuthUser | null>(null);
	let checking = $state(true);

	onMount(() => {
		void (async () => {
			const session = getStoredSession();
			if (!session) {
				await goto('/login');
				return;
			}
			const fresh = await ensureValidSession();
			if (!fresh) {
				await goto('/login');
				return;
			}
			user = fresh.user;
			checking = false;
		})();

		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function logout() {
		clearSession();
		goto('/login');
	}
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-3 bg-background text-foreground">
	{#if checking}
		<p class="text-sm text-muted-foreground">Memeriksa sesi…</p>
	{:else if user}
		<h1 class="text-2xl font-semibold tracking-tight capitalize">Area {area}</h1>
		<p class="text-sm text-muted-foreground">
			{user.email} · <span class="uppercase">{user.role}</span>
		</p>
		<p class="text-xs text-muted-foreground">{tr("Section asli di-port pada fase masing-masing (REWRITE/05_UI_PAGES.md)")}</p>
		<button
			class="mt-2 rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent"
			onclick={logout}
		>{tr("Keluar")}</button>
	{/if}
</main>
