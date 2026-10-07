<script lang="ts">
	// Port app/layout.tsx — font, metadata, tema (dipaksa light, parity
	// ThemeProvider: dark dimatikan), ToastHost, init i18n locale.
	import '@fontsource/geist/latin-400.css';
	import '@fontsource/geist-mono/latin-400.css';
	import '../app.css';
	import { onMount } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import { updated } from '$app/state';
	import ToastHost from '$lib/components/toast-host.svelte';
	import { initLocaleFromStorage } from '$lib/i18n/i18n.svelte';

	let { children } = $props();

	// Deploy baru: aset lama bisa sudah tidak ada. Muat ulang penuh saat user
	// berpindah halaman berikutnya — pekerjaan yang sedang berjalan tidak dipotong.
	beforeNavigate(({ willUnload, to }) => {
		if (updated.current && !willUnload && to?.url) {
			window.location.href = to.url.href;
		}
	});

	onMount(() => {
		// Request yang dibatalkan (timeout 12 dtk / pembatalan sengaja) bukan bug,
		// tapi browser mencatatnya sebagai "Uncaught (in promise) AbortError".
		// Telan hanya kelas error itu; error lain tetap dilaporkan apa adanya.
		const onRejection = (event: PromiseRejectionEvent) => {
			const name = (event.reason as { name?: string } | null | undefined)?.name;
			if (name === 'AbortError' || name === 'TimeoutError') {
				event.preventDefault();
				console.debug('[tauthub] request dibatalkan/di-timeout:', name);
			}
		};
		window.addEventListener('unhandledrejection', onRejection);
		// Chunk route lama (hash berubah setelah deploy) gagal di-import: Vite
		// menembakkan vite:preloadError. Muat ulang sekali supaya ambil bundle baru.
		const RELOAD_KEY = 'tauthub:preload-reload-at';
		const onPreloadError = (event: Event) => {
			event.preventDefault();
			try {
				const last = Number(sessionStorage.getItem(RELOAD_KEY) ?? 0);
				if (Date.now() - last < 15_000) return; // cegah loop reload
				sessionStorage.setItem(RELOAD_KEY, String(Date.now()));
			} catch {
				/* sessionStorage diblokir — tetap reload */
			}
			window.location.reload();
		};
		window.addEventListener('vite:preloadError', onPreloadError);
		return () => {
			window.removeEventListener('unhandledrejection', onRejection);
			window.removeEventListener('vite:preloadError', onPreloadError);
		};
	});

	$effect(() => {
		// Parity ThemeProvider: selalu light + bersihkan preferensi dark lama.
		document.documentElement.setAttribute('data-theme', 'light');
		localStorage.removeItem('aiw-theme');
		initLocaleFromStorage();
	});
</script>

<svelte:head>
	<!-- Ikon parity app/layout.tsx: favicon.ico (sizes any) + tauthub-icon.png,
	     apple/shortcut memakai ikon PNG yang sama. -->
	<link rel="icon" href="/favicon.ico" sizes="any" />
	<link rel="icon" href="/tauthub-icon.png" type="image/png" />
	<link rel="apple-touch-icon" href="/tauthub-icon.png" />
	<link rel="shortcut icon" href="/favicon.ico" />
	<link rel="manifest" href="/manifest.webmanifest" />
	<meta name="theme-color" content="#212121" />
	<title>Tauthub</title>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<meta
		name="description"
		content="Platform distribusi konten yang menghubungkan Klien dengan Media Partner untuk pembuatan, pengelolaan, dan distribusi artikel."
	/>
</svelte:head>

{@render children()}

<ToastHost />
