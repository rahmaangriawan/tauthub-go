<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import AgentPanel from '../_agent-panel.svelte';
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }]} crumb={tr('Agent Matchmaker')} path="/admin/taut-brief/agent-matchmaker">
		<PageHeader
			title={tr('Agent AI Media Matchmaker')}
			description={tr('Mengubah brief kampanye bebas menjadi kriteria media. Penyaringan dan skor tetap deterministik di server terhadap katalog nyata.')}
		/>
		<AgentPanel
			agent="media_matchmaker"
			title={tr('Koneksi & Prompt Agent')}
			description={tr('Atur transport, model, dan prompt yang dipakai tombol AI Matchmaker di katalog media.')}
		/>
	</ExplorerFrame>
</section>
