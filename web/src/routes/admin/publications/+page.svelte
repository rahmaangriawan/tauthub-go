<script lang="ts">
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import FolderGrid from '$lib/components/admin/folder-grid.svelte';

	const i18n = $derived(getI18n());

	// Explorer folder Publikasi — seperti Taut Pintar: menu tanpa daftar panjang,
	// sub-halamannya ditampilkan sebagai folder ala file manager.
	// Menambah sub-halaman baru = menambah satu entri di FOLDERS ini.

	type FolderEntry = {
		label: string;
		href: string;
		description: string;
		// Nama "berkas" ala explorer yang menggambarkan isi folder.
		tag: string;
	};

	const FOLDERS: FolderEntry[] = $derived([
		{
			label: tr('Daftar Publikasi'),
			href: '/admin/publications/daftar',
			description: tr('Semua publikasi seluruh brand: status pengiriman, harga klien, dan publikasi bermasalah.'),
			tag: 'publikasi.log'
		},
		{
			label: i18n.t.nav_admin.orders,
			href: '/admin/orders',
			description: tr('Monitoring order per artikel maupun per kampanye (CAM-): media terorder, terbit, rencana vs terpakai rupiah.'),
			tag: 'monitoring-order.log'
		},
		{
			label: i18n.t.nav_admin.variants,
			href: '/admin/variants',
			description: tr('Batch varian artikel untuk media yang butuh tulisan berbeda, plus batch yang di-generate ulang.'),
			tag: 'variants/'
		},
		{
			label: i18n.t.nav_admin.auto_replace,
			href: '/admin/auto-replace',
			description: tr('Aturan penggantian otomatis (gambar, tautan, teks) saat artikel dikirim ke media.'),
			tag: 'auto-replace.cfg'
		},
		{
			label: i18n.t.nav_admin.rss_feeds,
			href: '/admin/rss-feeds',
			description: tr('Sumber artikel RSS feed yang secara berkala menyerap artikel baru ke katalog.'),
			tag: 'rss-feeds.cfg'
		}
	]);

	const countLabel = $derived(`${FOLDERS.length} ${tr('folder')}`);
</script>

<ExplorerFrame crumb={tr('Publikasi')} path="/admin/publications" meta={countLabel}>
	<PageHeader
		title={tr('Publikasi')}
		description={tr('Semua alat publikasi dan distribusi artikel dalam satu folder. Buka folder untuk membuka halamannya.')}
	/>

	<FolderGrid folders={FOLDERS} />
</ExplorerFrame>
