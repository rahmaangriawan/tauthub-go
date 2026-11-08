<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Shell authentication split-screen — panel kiri visual (garis mengalir
	// ala blok "Login 3" Hirael, versi terang), panel kanan form. Panel kiri
	// murni dekoratif dan disembunyikan di mobile (logo pindah ke atas form).
	import type { Snippet } from 'svelte';
	import FloatingPaths from '$lib/components/floating-paths.svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="auth">
	<aside class="auth-marketing">
		<div class="auth-media">
			<FloatingPaths position={1} />
			<FloatingPaths position={-1} />
			<div class="auth-media-fade" aria-hidden="true"></div>

			<a class="auth-brand auth-enter" href="/login" aria-label="Tauthub">
				<img src="/tauthub-logo.webp" alt="Tauthub" width="130" height="34" class="auth-brand-logo" />
			</a>

			<div class="auth-media-copy auth-enter" style="--i: 4">
				<h2>{tr("Semua Publikasi, Satu Kendali.")}</h2>
				<p>{tr("Pantau proses distribusi dan status artikel tanpa berpindah platform.")}</p>
			</div>
		</div>
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
