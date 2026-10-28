<script lang="ts">
	// Port app/layout.tsx — font, metadata, tema (dipaksa light, parity
	// ThemeProvider: dark dimatikan), ToastHost, init i18n locale.
	import '@fontsource/geist/latin-400.css';
	import '@fontsource/geist-mono/latin-400.css';
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import ToastHost from '$lib/components/toast-host.svelte';
	import { initLocaleFromStorage } from '$lib/i18n/i18n.svelte';

	let { children } = $props();

	// Metadata SEO: halaman publik (data.tauthub.com) mengisi page.data.seo;
	// halaman dashboard tidak, jadi memakai nilai bawaan.
	const seo = $derived(page.data?.seo);
	const defaultDescription =
		'Platform distribusi konten yang menghubungkan Klien dengan Media Partner untuk pembuatan, pengelolaan, dan distribusi artikel.';

	// Escape `<` supaya JSON-LD tidak bisa menutup tag script lebih awal.
	const jsonLd = $derived(seo?.jsonLd ? JSON.stringify(seo.jsonLd).replace(/</g, '\\u003c') : '');

	// Catatan: dulu ada beforeNavigate yang memaksa window.location.href saat
	// versi aplikasi berubah (deploy baru). Itu balapan dengan router SvelteKit:
	// router sudah memindahkan URL, lalu navigasi dokumen yang dipaksa dibatalkan,
	// hasilnya URL berubah tapi halaman lama tetap tampil. Aset basi setelah
	// deploy sudah ditangani listener vite:preloadError di bawah.

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
	<!-- Favicon media (Google s2) dipasang setelah hydration; tarik TLS sejak
	     awal supaya bitmap avatar pertama tidak menunggu handshake. -->
	<link rel="preconnect" href="https://www.google.com" />
	<link rel="dns-prefetch" href="https://www.google.com" />
	<meta name="theme-color" content="#212121" />
	<title>{seo?.title ?? 'Tauthub'}</title>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<meta name="description" content={seo?.description ?? defaultDescription} />
	{#if seo?.robots}<meta name="robots" content={seo.robots} />{/if}
	{#if seo?.canonical}
		<link rel="canonical" href={seo.canonical} />
		<meta property="og:url" content={seo.canonical} />
	{/if}
	{#if seo}
		<meta property="og:site_name" content="Tauthub" />
		<meta property="og:type" content={seo.ogType ?? 'website'} />
		<meta property="og:title" content={seo.title} />
		<meta property="og:description" content={seo.description ?? defaultDescription} />
		<meta property="og:locale" content="id_ID" />
		<meta name="twitter:card" content="summary" />
		<meta name="twitter:title" content={seo.title} />
		<meta name="twitter:description" content={seo.description ?? defaultDescription} />
	{/if}
	{#if jsonLd}
		{@html `<script type="application/ld+json">${jsonLd}</script>`}
	{/if}
</svelte:head>

{@render children()}

<ToastHost />
