<script lang="ts">
	// Parity app lama (app/mitra/[partnerId]/[[...section]]/page.tsx): section
	// `publishing-api` dialihkan ke
	// /integrations#web-custom. Isi aslinya hidup sebagai tab di halaman Integrasi.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { tr } from '$lib/i18n/phrases';
	import PageTitle from '$lib/components/page-title.svelte';

	const routeId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || routeId}`);

	onMount(() => {
		void goto(`${base}/integrations#web-custom${page.url.search}`, { replaceState: true });
	});
</script>

<PageTitle title="API Publishing" />

<section>
	<div class="app-card p-6 text-center">
		<p class="text-sm text-muted-foreground">{tr('Mengarahkan ke Integrasi…')}</p>
	</div>
</section>
