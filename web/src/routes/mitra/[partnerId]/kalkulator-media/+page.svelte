<script lang="ts">
	// Parity app lama: `section === "seo-tool" || section === "kalkulator-media"`
	// merender SeoToolSection yang sama (lihat app/mitra/[partnerId]/[[...section]]/page.tsx).
	// Jadi halaman ini BUKAN kalkulator harga — ia menampilkan Tool SEO yang sama.
	import SeoTool from '../seo-tool/+page.svelte';
</script>

<SeoTool />
