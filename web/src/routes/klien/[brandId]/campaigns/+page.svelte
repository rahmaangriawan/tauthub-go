<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { tr } from '$lib/i18n/phrases';

	// Menu "Kampanye" klien = daftar kampanye Taut Pintar (parity app lama:
	// section "campaigns" merender TautPintarClient, bukan wizard intake lama).
	import CampaignList from '../_components/campaign-list.svelte';

	// Topik dari "Buat artikel untuk ini" di Taut Brief. Disimpan di browser
	// sampai Pengelola Naskah memakainya sekali (atau 24 jam berlalu).
	let pendingTopic = $state('');
	const pendingKey = () => `tauthub:pending-topic:${page.params.brandId}`;

	onMount(() => {
		try {
			const raw = localStorage.getItem(pendingKey());
			if (!raw) return;
			const parsed = JSON.parse(raw) as { topic?: string; at?: number };
			if (parsed?.topic && Date.now() - Number(parsed.at ?? 0) < 24 * 60 * 60 * 1000) {
				pendingTopic = String(parsed.topic);
			} else {
				localStorage.removeItem(pendingKey());
			}
		} catch {
			/* penyimpanan browser diblokir atau isinya rusak: tidak ada banner */
		}
	});

	function dropPendingTopic() {
		pendingTopic = '';
		try {
			localStorage.removeItem(pendingKey());
		} catch {
			/* abaikan */
		}
	}
</script>

<svelte:head><title>{tr("Kampanye · Tauthub")}</title></svelte:head>
{#if pendingTopic}
	<div class="pending-topic" role="status">
		<p>
			<strong>{tr('Topik artikel dari Taut Brief sudah siap:')}</strong>
			{pendingTopic}
		</p>
		<p class="pending-topic-steps">
			{tr('Buka satu kampanye, lalu Pengelola Naskah. Topik terisi sendiri dan brief aktif Anda sudah terpilih.')}
		</p>
		<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={dropPendingTopic}>{tr('Batalkan topik')}</button>
	</div>
{/if}
<CampaignList />

<style>
	.pending-topic {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px 16px;
		margin: 0 0 16px;
		padding: 12px 16px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--accent);
		color: var(--accent-foreground);
		font-size: 13px;
	}
	.pending-topic p {
		margin: 0;
		min-width: 0;
		flex: 1 1 320px;
	}
	.pending-topic-steps {
		color: var(--muted-foreground);
		font-size: 12px;
	}
</style>
