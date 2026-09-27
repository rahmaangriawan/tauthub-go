<script lang="ts">
	// Media Intelligence telah digabung ke /admin/media (tab Intelligence) pada app lama.
	// Port apps/web/src/app/admin/media-intelligence/page.tsx — redirect agar link lama tetap berfungsi.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import PageTitle from '$lib/components/page-title.svelte';

	onMount(() => {
		void goto('/admin/media', { replaceState: true });
	});
</script>

<PageTitle title="Media Intelligence" />

<section class="w-full">
	<p class="mt-6 text-sm text-muted-foreground">
		Halaman lama — Media Intelligence telah digabung ke <a href="/admin/media" style="color: var(--primary);">Admin · Media</a>
		(tab Intelligence). Anda dialihkan…
	</p>
</section>
