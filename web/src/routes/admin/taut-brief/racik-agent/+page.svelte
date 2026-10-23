<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import AgentPanel from '../_agent-panel.svelte';
</script>

<section>
	<PageHeader
		title={tr('Racik Agent')}
		description={tr('Menyusun instruksi topik dari anchor di Pengelola Naskah. Dipakai tombol Racik Instruksi dari Anchor.')}
	/>
	<AgentPanel
		agent="racik_instruksi"
		title={tr('Koneksi & Prompt Agent')}
		description={tr('Atur transport, model, dan master prompt yang dipakai agent racik instruksi.')}
		disabledHint={tr('Selama nonaktif, tombol Racik Instruksi dari Anchor akan menolak permintaan.')}
	/>
</section>
