<script lang="ts">
	import { page } from '$app/state';
	import { env } from '$env/dynamic/public';
	import '$lib/public/public.css';

	// Shell situs publik data.tauthub.com: header + footer ringan, tanpa shell
	// dashboard. Semua halaman katalog memakai komponen ini.
	let { children } = $props();

	const APP_ORIGIN = (env.PUBLIC_APP_ORIGIN ?? 'https://app.tauthub.com').replace(/\/+$/, '');
	const host = $derived(page.url.hostname.toLowerCase());
	// Di host lokal/staging, /login milik aplikasi sendiri.
	const loginHref = $derived(
		host.startsWith('data.') || host === 'data.tauthub.com' ? `${APP_ORIGIN}/login` : '/login'
	);

	// Tahun berjalan dihitung di server saat render; tidak perlu reaktif.
	const year = new Date().getFullYear();
</script>

<div class="pub-page">
	<a class="pub-skip" href="#main">Lewati ke konten utama</a>

	<header class="pub-header">
		<div class="pub-container pub-header-inner">
			<a class="pub-brand" href="/" aria-label="Tauthub Data, beranda">
				<img class="pub-brand-logo" src="/tauthub-logo.webp" alt="Tauthub" width="134" height="22" />
				<span class="pub-brand-tag">Data</span>
			</a>
			<nav class="pub-nav" aria-label="Navigasi katalog">
				<a class="pub-nav-link" href="/simulasi">Simulasi paket</a>
				<a class="pub-btn pub-btn-primary" href={loginHref}>Masuk / Daftar</a>
			</nav>
		</div>
	</header>

	{@render children()}

	<footer class="pub-footer">
		<div class="pub-container pub-footer-inner">
			<p class="pub-footer-copy">© {year} Tauthub</p>
			<nav class="pub-footer-legal" aria-label="Legal">
				<a href="https://tauthub.com/kebijakan-privasi/">Kebijakan Privasi</a>
				<span aria-hidden="true" class="pub-footer-sep">|</span>
				<a href="https://tauthub.com/ketentuan-layanan/">Syarat &amp; Ketentuan</a>
			</nav>
		</div>
	</footer>
</div>
