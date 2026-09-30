<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Route flow lama (intake → content → media → review → distribusi) sudah
	// dipensiunkan. Semua tautan lama diarahkan ke alur Taut Pintar: router
	// /taut-pintar/{id} menentukan langkah dari taut_status kampanye.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.campaignId as string);

	onMount(() => {
		const base = `/klien/${getBrandUrlId() || brandId}/taut-pintar/${campaignId}`;
		void goto(base, { replaceState: true });
	});
</script>

<PageTitle title={tr("Alur Kampanye")} />

<main class="flex min-h-[50vh] items-center justify-center">
	<p class="flex items-center gap-2 text-sm text-muted-foreground">
		<LoaderCircle size={16} class="animate-spin" /> Mengalihkan ke alur Taut Pintar…
	</p>
</main>
