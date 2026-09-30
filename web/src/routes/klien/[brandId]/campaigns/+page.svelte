<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Menu "Kampanye" klien = daftar kampanye Taut Pintar (parity app lama:
	// section "campaigns" merender TautPintarClient, bukan wizard intake lama).
	import CampaignList from '../_components/campaign-list.svelte';
</script>

<svelte:head><title>{tr("Kampanye · Tauthub")}</title></svelte:head>
<CampaignList />
