<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Daftar & manajemen kampanye klien (alur Taut Pintar).
	// Di app lama, menu "Kampanye" merender TautPintarClient yang sama, jadi
	// /campaigns dan /taut-pintar memakai komponen daftar ini.
	import CampaignList from '../_components/campaign-list.svelte';
</script>

<svelte:head><title>{tr("Kampanye · Tauthub")}</title></svelte:head>
<CampaignList />
