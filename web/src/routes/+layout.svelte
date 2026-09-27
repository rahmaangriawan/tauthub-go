<script lang="ts">
	// Port app/layout.tsx — font, metadata, tema (dipaksa light, parity
	// ThemeProvider: dark dimatikan), ToastHost, init i18n locale.
	import '@fontsource/geist/latin-400.css';
	import '@fontsource/geist-mono/latin-400.css';
	import '../app.css';
	import ToastHost from '$lib/components/toast-host.svelte';
	import { initLocaleFromStorage } from '$lib/i18n/i18n.svelte';

	let { children } = $props();

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
