<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import AgentPanel from '../_agent-panel.svelte';
</script>

<section>
	<PageHeader
		title={tr('Agent Ringkasan File')}
		description={tr('Meringkas dokumen, tautan, dan catatan gambar yang diunggah ke brief. Hasilnya jadi referensi pendukung AI Writer.')}
	/>
	<AgentPanel
		agent="brief_file_summary"
		title={tr('Koneksi & Prompt Agent')}
		description={tr('Atur transport, model, dan prompt yang dipakai agent ringkasan file.')}
	/>
</section>
