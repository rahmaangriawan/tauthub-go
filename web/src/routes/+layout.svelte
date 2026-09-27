<script lang="ts">
	// Port app/layout.tsx — font, metadata, tema (dipaksa light, parity
	// ThemeProvider: dark dimatikan), ToastHost, init i18n locale.
	import '@fontsource/geist/latin-400.css';
	import '@fontsource/geist-mono/latin-400.css';
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
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
	<link rel="icon" href={favicon} />
	<title>Tauthub</title>
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<meta
		name="description"
		content="Platform distribusi konten yang menghubungkan Klien dengan Media Partner untuk pembuatan, pengelolaan, dan distribusi artikel."
	/>
</svelte:head>

{@render children()}

<ToastHost />
