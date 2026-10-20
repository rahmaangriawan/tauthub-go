<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Shell authentication split-screen — panel kiri foto penuh, panel kanan
	// form. Panel kiri murni visual dan disembunyikan di mobile (logo di atas form).
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="auth">
	<aside class="auth-marketing">
		<div class="auth-media">
			<img
				src="/img/auth-hero.webp"
				alt=""
				aria-hidden="true"
				class="auth-marketing-photo"
				decoding="async"
			/>

			<a class="auth-brand" href="/login" aria-label="Tauthub">
				<img src="/tauthub-logo.webp" alt="Tauthub" width="130" height="34" class="auth-brand-logo" />
			</a>

			<div class="auth-media-copy">
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
