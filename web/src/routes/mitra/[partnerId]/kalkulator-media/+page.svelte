<script lang="ts">
	// Parity app lama: `section === "seo-tool" || section === "kalkulator-media"`
	// merender SeoToolSection yang sama (lihat app/mitra/[partnerId]/[[...section]]/page.tsx).
	// Jadi halaman ini BUKAN kalkulator harga — ia menampilkan Tool SEO yang sama.
	import SeoTool from '../seo-tool/+page.svelte';
	import { tr } from '$lib/i18n/phrases';
	import PageTitle from '$lib/components/page-title.svelte';
</script>

<PageTitle title={tr('SEO & Performa')} />

<SeoTool />
