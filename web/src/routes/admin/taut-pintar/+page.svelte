<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import FolderGrid from '$lib/components/admin/folder-grid.svelte';

	// Explorer folder Taut Pintar — menu Taut Pintar sengaja tanpa submenu,
	// semua sub-halaman ditampilkan di sini sebagai folder ala file manager.
	// Menambah sub-halaman baru = menambah satu entri di FOLDERS ini.

	type FolderEntry = {
		label: string;
		href: string;
		description: string;
		// Nama "berkas" ala explorer yang menggambarkan isi folder.
		tag: string;
	};

	const FOLDERS: FolderEntry[] = [
		{
			label: tr('Monitoring'),
			href: '/admin/taut-pintar/monitoring',
			description: tr('Pipeline estafet 4 agent per kampanye + retry per-step.'),
			tag: 'monitoring.log'
		},
		{
			label: 'AI Gambar',
			href: '/admin/taut-pintar/ai-image',
			description: tr('Konfigurasi provider generate gambar (cover & gambar per H2) Taut Pintar Writer, plus log hasil.'),
			tag: 'image-provider.cfg'
		},
		{
			label: 'Agent Config',
			href: '/admin/taut-agents',
			description: tr('Konfigurasi per-agent LLM Taut Pintar: base URL, API key, model, dan gatekeeper.'),
			tag: 'agents.config'
		},
		{
			label: tr('Racik Agent'),
			href: '/admin/taut-brief/racik-agent',
			description: tr('Agent penyusun instruksi topik per potongan naskah dari Anchor.'),
			tag: 'racik.config'
		},
		{
			// Folder bertingkat: dua agent AI Media Matchmaker di dalamnya.
			label: tr('Agent Matchmaker'),
			href: '/admin/taut-brief/agent-matchmaker',
			description: tr('Dua agent AI Media Matchmaker: Agent Ringkasan (brief menjadi kriteria) dan Agent Pemilih Media (memilih paket dari katalog).'),
			tag: 'matchmaker/'
		},
		{
			// Folder bertingkat: isinya Konfigurasi, Agent Entitas, Agent File.
			label: tr('Taut Brief'),
			href: '/admin/taut-brief',
			description: tr('Konfigurasi Taut Brief dan agent ringkasannya: Konfigurasi, Agent Entitas, Agent File.'),
			tag: 'taut-brief/'
		}
	];

	const countLabel = `${FOLDERS.length} ${tr('folder')}`;
</script>

<ExplorerFrame crumb={tr('Taut Pintar')} path="/admin/taut-pintar" meta={countLabel}>
	<PageHeader
		title={tr('Taut Pintar')}
		description={tr('Semua alat AI Taut Pintar dalam satu folder. Buka folder untuk membuka halamannya.')}
	/>

	<FolderGrid folders={FOLDERS} />
</ExplorerFrame>
