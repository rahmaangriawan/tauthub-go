<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import AgentPanel from '../_agent-panel.svelte';
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }, { label: tr('Taut Brief'), href: '/admin/taut-brief' }]} crumb={tr('Agent Entitas')} path="/admin/taut-brief/agent-entitas">
	<PageHeader
		title={tr('Agent Ringkasan Entitas')}
		description={tr('Meringkas identitas & fakta kanonik brief. Hasilnya dipakai AI Writer mode Yang Proper, menggantikan dump seluruh profil.')}
	/>
	<AgentPanel
		agent="brief_entity_summary"
		title={tr('Koneksi & Prompt Agent')}
		description={tr('Atur transport, model, dan prompt yang dipakai agent ringkasan entitas.')}
	/>
	</ExplorerFrame>
</section>
