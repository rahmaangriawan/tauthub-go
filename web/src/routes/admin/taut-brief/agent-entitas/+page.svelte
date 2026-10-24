<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import AgentPanel from '../_agent-panel.svelte';
	import AgentLog from '../_agent-log.svelte';

	// Dua tab: konfigurasi agent dan log run-nya sendiri. Tanpa tab Log,
	// kegagalan agent ringkasan hanya terlihat di journal server.
	let tab = $state<'agent' | 'log'>('agent');
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }, { label: tr('Taut Brief'), href: '/admin/taut-brief' }]} crumb={tr('Agent Entitas')} path="/admin/taut-brief/agent-entitas">
	<PageHeader
		title={tr('Agent Ringkasan Entitas')}
		description={tr('Meringkas identitas & fakta kanonik brief. Hasilnya dipakai AI Writer mode Yang Proper, menggantikan dump seluruh profil.')}
	/>

	<div class="mt-4 flex flex-wrap gap-1">
		{#each [['agent', 'Konfigurasi'], ['log', 'Log pemakaian']] as [value, label] (value)}
			<button
				type="button"
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={tab === value}
				class:app-btn-soft={tab !== value}
				onclick={() => (tab = value as 'agent' | 'log')}
			>{tr(label)}</button>
		{/each}
	</div>

	{#if tab === 'agent'}
		<AgentPanel
			agent="brief_entity_summary"
			title={tr('Koneksi & Prompt Agent')}
			description={tr('Atur transport, model, dan prompt yang dipakai agent ringkasan entitas.')}
		/>
	{:else}
		<AgentLog
			agents={['brief_entity_summary']}
			emptyText={tr('Belum ada run. Buka sebuah brief lalu klik "Buat ringkasan" di tab Ringkasan.')}
		/>
	{/if}
	</ExplorerFrame>
</section>
