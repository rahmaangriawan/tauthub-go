<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { rp } from '$lib/credits/rupiah';

	// Grafik arus kas 12 bulan: dua batang per bulan (masuk/keluar) tanpa
	// library chart — mengikuti pola sparkline.svelte yang sudah ada.
	type BarRow = { month: string; label: string; money_in: number; money_out: number };

	let { rows, onSelect }: { rows: BarRow[]; onSelect: (month: string) => void } = $props();

	const max = $derived(Math.max(1, ...rows.map((row) => Math.max(row.money_in, row.money_out))));
	const pct = (value: number) => (value <= 0 ? 0 : Math.max(2, (value / max) * 100));
</script>

<div class="overflow-x-auto">
	<div class="min-w-[560px]">
		<div class="mb-3 flex items-center justify-end gap-4 text-xs text-muted-foreground">
			<span class="flex items-center gap-1.5">
				<span class="h-2.5 w-2.5 rounded-sm" style="background-color: var(--success);"></span>
				{tr("Uang masuk")}
			</span>
			<span class="flex items-center gap-1.5">
				<span class="h-2.5 w-2.5 rounded-sm" style="background-color: var(--destructive);"></span>
				{tr("Uang keluar")}
			</span>
		</div>

		<div class="flex items-end gap-1.5" style="height: 150px;">
			{#each rows as row (row.month)}
				<button
					type="button"
					class="flex h-full flex-1 flex-col justify-end rounded-md transition-colors hover:bg-secondary"
					title={`${row.label} · ${tr("masuk")} ${rp(row.money_in)} · ${tr("keluar")} ${rp(row.money_out)}`}
					aria-label={`${row.label}: ${tr("masuk")} ${rp(row.money_in)}, ${tr("keluar")} ${rp(row.money_out)}`}
					onclick={() => onSelect(row.month)}
				>
					<div class="flex flex-1 items-end justify-center gap-1 px-1">
						<span
							class="w-1/3 max-w-4 rounded-t-sm"
							style="height: {pct(row.money_in)}%; background-color: var(--success); opacity: {row.money_in > 0 ? 1 : 0.15};"
						></span>
						<span
							class="w-1/3 max-w-4 rounded-t-sm"
							style="height: {pct(row.money_out)}%; background-color: var(--destructive); opacity: {row.money_out > 0 ? 1 : 0.15};"
						></span>
					</div>
				</button>
			{/each}
		</div>

		<div class="mt-2 flex gap-1.5">
			{#each rows as row (row.month)}
				<span class="flex-1 text-center text-[10px] text-muted-foreground">{row.label}</span>
			{/each}
		</div>
	</div>
</div>
