<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Media Intelligence telah digabung ke /admin/media (tab Intelligence) pada app lama.
	// Port apps/web/src/app/admin/media-intelligence/page.tsx — redirect agar link lama tetap berfungsi.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import PageTitle from '$lib/components/page-title.svelte';

	onMount(() => {
		void goto('/admin/media', { replaceState: true });
	});
</script>

<PageTitle title={tr("Media Intelligence")} />

<section class="w-full">
	<p class="mt-6 text-sm text-muted-foreground">{tr("Halaman lama — Media Intelligence telah digabung ke")}<a href="/admin/media" style="color: var(--primary);">{tr("Admin · Media")}</a>{tr("(tab Intelligence). Anda dialihkan…")}</p>
</section>
