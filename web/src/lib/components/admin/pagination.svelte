<script lang="ts">
	// Paginasi admin (parity kontrol "Sebelumnya/Berikutnya" app lama).
	let {
		total,
		limit,
		offset,
		onChange
	}: { total: number; limit: number; offset: number; onChange: (nextOffset: number) => void } = $props();

	const fmt = (n: number) => n.toLocaleString('id-ID');
</script>

{#if total > limit}
	<div class="mt-4 flex items-center justify-between text-sm">
		<p class="text-muted-foreground">
			Menampilkan {offset + 1}–{Math.min(offset + limit, total)} dari {fmt(total)}
		</p>
		<div class="flex gap-2">
			<button
				class="app-btn app-btn-soft"
				type="button"
				disabled={offset === 0}
				onclick={() => onChange(Math.max(0, offset - limit))}
			>Sebelumnya</button>
			<button
				class="app-btn app-btn-soft"
				type="button"
				disabled={offset + limit >= total}
				onclick={() => onChange(offset + limit)}
			>Berikutnya</button>
		</div>
	</div>
{/if}
