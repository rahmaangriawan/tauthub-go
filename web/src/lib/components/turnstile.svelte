<script lang="ts">
	// Cloudflare Turnstile (explicit render) — port components/turnstile.tsx.
	// Token single-use: parent me-render ulang widget dengan mengganti `key`
	// (Svelte: blok {#key}) setiap kali token dipakai atau kedaluwarsa.
	import { onMount } from 'svelte';

	let {
		siteKey,
		onToken,
		onExpire
	}: { siteKey: string; onToken: (token: string) => void; onExpire: () => void } = $props();

	const SCRIPT_SRC = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
	let scriptPromise: Promise<void> | null = null;

	function loadScript(): Promise<void> {
		if (window.turnstile) return Promise.resolve();
		if (scriptPromise) return scriptPromise;
		scriptPromise = new Promise<void>((resolve, reject) => {
			const el = document.createElement('script');
			el.src = SCRIPT_SRC;
			el.async = true;
			el.defer = true;
			el.onload = () => resolve();
			el.onerror = () => reject(new Error('Gagal memuat Turnstile'));
			document.head.appendChild(el);
		});
		return scriptPromise;
	}

	let container: HTMLDivElement | undefined = $state();
	let status = $state<'loading' | 'ready' | 'error'>('loading');

	onMount(() => {
		if (!container || !siteKey) {
			status = 'error';
			return;
		}
		let disposed = false;
		loadScript()
			.then(() => {
				if (disposed || !window.turnstile || !container) return;
				try {
					window.turnstile.render(container, {
						sitekey: siteKey,
						callback: (token: string) => onToken(token),
						'expired-callback': () => onExpire(),
						'error-callback': () => onExpire()
					});
					status = 'ready';
				} catch {
					status = 'error';
				}
			})
			.catch(() => {
				if (!disposed) status = 'error';
			});
		return () => {
			disposed = true;
		};
	});
</script>

{#if status === 'error' || !siteKey}
	<p class="auth-verify-error">Verifikasi keamanan tidak tersedia. Muat ulang halaman atau hubungi admin.</p>
{:else}
	<div bind:this={container} class="auth-turnstile" aria-label="Cloudflare Turnstile"></div>
{/if}
