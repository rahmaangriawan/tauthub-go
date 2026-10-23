<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import FolderGrid from '$lib/components/admin/folder-grid.svelte';

	// Folder Taut Brief di dalam explorer Taut Pintar — isi: Konfigurasi,
	// Agent Entitas, Agent File. Menambah sub-halaman = tambah entri FOLDERS.
	const FOLDERS = [
		{
			label: tr('Konfigurasi'),
			href: '/admin/taut-brief/konfigurasi',
			description: tr('Asisten Profiler brand: percakapan yang menyusun brief lewat chat. Terpisah dari agent Taut Pintar.'),
			tag: 'taut-brief.cfg'
		},
		{
			label: tr('Agent Entitas'),
			href: '/admin/taut-brief/agent-entitas',
			description: tr('Meringkas identitas & fakta kanonik brief. Hasilnya dipakai AI Writer mode Yang Proper, menggantikan dump seluruh profil.'),
			tag: 'entity.config'
		},
		{
			label: tr('Agent File'),
			href: '/admin/taut-brief/agent-file',
			description: tr('Meringkas dokumen, tautan, dan catatan gambar yang diunggah ke brief. Hasilnya jadi referensi pendukung AI Writer.'),
			tag: 'file.config'
		}
	];

	const countLabel = `${FOLDERS.length} ${tr('folder')}`;
</script>

<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }]} crumb={tr('Taut Brief')} path="/admin/taut-brief" meta={countLabel}>
	<PageHeader
		title={tr('Taut Brief')}
		description={tr('Semua pengaturan Taut Brief dalam satu folder. Buka folder untuk membuka halamannya.')}
	/>

	<FolderGrid folders={FOLDERS} />
</ExplorerFrame>
