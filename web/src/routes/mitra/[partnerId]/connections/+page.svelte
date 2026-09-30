<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Parity app lama (app/mitra/[partnerId]/[[...section]]/page.tsx): section
	// `connections` dialihkan ke
	// /integrations#wordpress. Isi aslinya hidup sebagai tab di halaman Integrasi.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const routeId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || routeId}`);

	onMount(() => {
		void goto(`${base}/integrations#wordpress${page.url.search}`, { replaceState: true });
	});
</script>

<PageTitle title={tr("Koneksi Media")} />

<section>
	<div class="app-card p-6 text-center">
		<p class="text-sm text-muted-foreground">{tr('Mengarahkan ke Integrasi…')}</p>
	</div>
</section>
