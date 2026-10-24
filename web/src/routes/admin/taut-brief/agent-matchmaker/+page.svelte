<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import FolderGrid from '$lib/components/admin/folder-grid.svelte';

	// Folder Agent Matchmaker di dalam explorer Taut Pintar — dua agent
	// AI Media Matchmaker: peringkas brief dan pemilih media paket.
	const FOLDERS = [
		{
			label: tr('Agent Ringkasan'),
			href: '/admin/taut-brief/agent-matchmaker/ringkasan',
			description: tr('Mengubah brief kampanye bebas menjadi kriteria media. Penyaringan dan skor tetap deterministik di server terhadap katalog nyata.'),
			tag: 'matchmaker.config'
		},
		{
			label: tr('Agent Pemilih Media'),
			href: '/admin/taut-brief/agent-matchmaker/pemilih-media',
			description: tr('Memilih media dari kandidat katalog nyata berdasarkan ringkasan kebutuhan. Server tetap menentukan kandidat, harga, dan total — agent hanya memilih dari daftar.'),
			tag: 'media-picker.config'
		},
		{
			label: tr('Log'),
			href: '/admin/taut-brief/agent-matchmaker/log',
			description: tr('Pemakaian token, durasi, dan error dua agent AI Media Matchmaker (ringkasan brief & pemilih media).'),
			tag: 'matchmaker.log'
		}
	];

	const countLabel = `${FOLDERS.length} ${tr('folder')}`;
</script>

<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }]} crumb={tr('Agent Matchmaker')} path="/admin/taut-brief/agent-matchmaker" meta={countLabel}>
	<PageHeader
		title={tr('Agent Matchmaker')}
		description={tr('Konfigurasi dua agent AI Media Matchmaker: peringkas brief dan pemilih media. Buka folder untuk membuka halamannya.')}
	/>

	<FolderGrid folders={FOLDERS} />
</ExplorerFrame>
