<script lang="ts">
	// Analisis Kinerja — fitur belum dirilis, seluruh isi digantikan panel
	// Coming Soon (parity distribution.tsx AnalyticsSection).
	import { tr } from '$lib/i18n/phrases';
	import PageTitle from '$lib/components/page-title.svelte';
	import ComingSoon from '$lib/components/coming-soon.svelte';
</script>

<PageTitle title="Analisis Kinerja" />

<section>
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="page-title">Analisis Kinerja</h1>
			<p class="page-desc">{tr("Event keterbacaan artikel per hari dari pelacakan publikasi.")}</p>
		</div>
	</header>

	<!-- Fitur belum dirilis — seluruh isi section digantikan panel Coming Soon
	     (parity distribution.tsx AnalyticsSection di project lama). -->
	<div class="mt-4">
		<ComingSoon
			size="lg"
			message="Laporan performa artikel per media & per kampanye sedang kami siapkan. Fitur ini akan segera tersedia di Tauthub."
		/>
	</div>
</section>
