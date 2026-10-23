<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import AgentPanel from '../_agent-panel.svelte';
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }, { label: tr('Taut Brief'), href: '/admin/taut-brief' }]} crumb={tr('Agent File')} path="/admin/taut-brief/agent-file">
	<PageHeader
		title={tr('Agent Ringkasan File')}
		description={tr('Meringkas dokumen, tautan, dan catatan gambar yang diunggah ke brief. Hasilnya jadi referensi pendukung AI Writer.')}
	/>
	<AgentPanel
		agent="brief_file_summary"
		title={tr('Koneksi & Prompt Agent')}
		description={tr('Atur transport, model, dan prompt yang dipakai agent ringkasan file.')}
	/>
	</ExplorerFrame>
</section>
