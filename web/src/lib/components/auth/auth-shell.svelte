<script lang="ts">
	// Shell authentication split-screen (brief §1, §3) — panel kiri marketing,
	// panel kanan form. Panel kiri murni dekoratif/penjelas dan disembunyikan di
	// mobile (logo dipindah ke atas form).
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="auth">
	<aside class="auth-marketing">
		<div class="auth-arc" aria-hidden="true"></div>

		<a class="auth-brand" href="/login" aria-label="Tauthub">
			<img src="/tauthub-logo.webp" alt="Tauthub" width="130" height="34" class="auth-brand-logo" />
		</a>

		<div class="auth-marketing-body">
			<p class="auth-eyebrow">Distribusi konten &amp; PR multi-brand</p>
			<h1 class="auth-headline">
				Dari brief sampai artikel<br />terbit di media mitra.
			</h1>
			<p class="auth-lede">
				AI bantu susun strategi dan drafnya, Anda yang review dan memutuskan. Kelola semua brand
				Anda dari satu dasbor.
			</p>
		</div>

		<p class="auth-caption">Grounded writing. Human-reviewed.</p>
	</aside>

	<main class="auth-form-pane">
		<div class="auth-form-inner">
			<a class="auth-mobile-brand" href="/login" aria-label="Tauthub">
				<img src="/tauthub-logo.webp" alt="Tauthub" width="120" height="30" class="auth-brand-logo" />
			</a>
			{@render children()}
		</div>
	</main>
</div>
