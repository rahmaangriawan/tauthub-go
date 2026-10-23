<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import AgentPanel from '../_agent-panel.svelte';
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }]} crumb={tr('Agent Pemilih Media')} path="/admin/taut-brief/agent-matchmaker-select">
		<PageHeader
			title={tr('Agent AI Pemilih Media')}
			description={tr('Memilih media dari kandidat katalog nyata berdasarkan ringkasan kebutuhan. Server tetap menentukan kandidat, harga, dan total — agent hanya memilih dari daftar.')}
		/>
		<AgentPanel
			agent="media_matchmaker_select"
			title={tr('Koneksi & Prompt Agent')}
			description={tr('Atur transport, model, dan prompt yang dipakai agent pemilih media pada langkah "Pilih media" di katalog.')}
		/>
	</ExplorerFrame>
</section>
