<script lang="ts">
	// Audit Publikasi — fitur belum dirilis, seluruh isi digantikan panel
	// Coming Soon (parity brand-workspace.tsx section audit-publikasi).
	// Fetch/state tidak dijalankan supaya tidak ada request sia-sia; logika asli
	// disimpan di riwayat git saat fitur ini benar-benar dikerjakan.
	import { tr } from '$lib/i18n/phrases';
	import PageTitle from '$lib/components/page-title.svelte';
	import ComingSoon from '$lib/components/coming-soon.svelte';
</script>

<PageTitle title={tr("Audit Publikasi")} />

<section>
	<header>
		<h1 class="page-title">{tr("Audit Publikasi")}</h1>
		<p class="page-desc">{tr("Rekap kepatuhan artikel terbit: link hidup, hilang, atau tidak terjangkau.")}</p>
	</header>

	<!-- Fitur belum dirilis — panel Coming Soon menggantikan isi (parity
	     brand-workspace.tsx section audit-publikasi di project lama). -->
	<div class="mt-4">
		<ComingSoon
			size="lg"
			message="Laporan QC dan rekam jejak Gatekeeper sedang kami siapkan. Fitur ini akan segera tersedia di Tauthub."
		/>
	</div>
</section>
