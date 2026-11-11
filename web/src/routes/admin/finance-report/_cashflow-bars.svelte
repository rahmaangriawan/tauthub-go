<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { rp } from '$lib/credits/rupiah';

	// Grafik 12 bulan: tiga batang per bulan (Top Up masuk, GMV publikasi
	// terbit, pencairan mitra) tanpa library chart — mengikuti pola pure-SVG/CSS
	// sparkline.svelte. gmv ditandai optional supaya kompat dengan respons API
	// lama yang belum mengirim field itu.
	type BarRow = {
		month: string; label: string;
		money_in: number; money_out: number; gmv?: number;
	};

	let { rows, onSelect }: { rows: BarRow[]; onSelect: (month: string) => void } = $props();

	const hasGMV = $derived(rows.some((r) => (r.gmv ?? 0) > 0));
	const max = $derived(
		Math.max(
			1,
			...rows.map((row) => Math.max(row.money_in, row.money_out, row.gmv ?? 0))
		)
	);
	const pct = (value: number) => (value <= 0 ? 0 : Math.max(2, (value / max) * 100));
</script>

<div class="overflow-x-auto">
	<div class="min-w-[640px]">
		<div class="mb-3 flex flex-wrap items-center justify-end gap-x-4 gap-y-1 text-xs text-muted-foreground">
			<span class="flex items-center gap-1.5">
				<span class="h-2.5 w-2.5 rounded-sm" style="background-color: #3b82f6;"></span>
				{tr("Top Up masuk")}
			</span>
			{#if hasGMV}
				<span class="flex items-center gap-1.5">
					<span class="h-2.5 w-2.5 rounded-sm" style="background-color: #10b981;"></span>
					{tr("GMV publikasi terbit")}
				</span>
			{/if}
			<span class="flex items-center gap-1.5">
				<span class="h-2.5 w-2.5 rounded-sm" style="background-color: #f97316;"></span>
				{tr("Pencairan mitra")}
			</span>
		</div>

		<div class="flex items-end gap-1.5" style="height: 160px;">
			{#each rows as row (row.month)}
				<button
					type="button"
					class="flex h-full flex-1 flex-col justify-end rounded-md transition-colors hover:bg-secondary"
					title={`${row.label} · ${tr("Top Up")} ${rp(row.money_in)}${hasGMV ? ` · ${tr("GMV")} ${rp(row.gmv ?? 0)}` : ''} · ${tr("Pencairan")} ${rp(row.money_out)}`}
					aria-label={`${row.label}: Top Up ${rp(row.money_in)}, GMV ${rp(row.gmv ?? 0)}, Pencairan ${rp(row.money_out)}`}
					onclick={() => onSelect(row.month)}
				>
					<div class="flex flex-1 items-end justify-center gap-0.5 px-1">
						<span
							class="w-1/4 max-w-3 rounded-t-sm"
							style="height: {pct(row.money_in)}%; background-color: #3b82f6; opacity: {row.money_in > 0 ? 1 : 0.15};"
						></span>
						{#if hasGMV}
							<span
								class="w-1/4 max-w-3 rounded-t-sm"
								style="height: {pct(row.gmv ?? 0)}%; background-color: #10b981; opacity: {(row.gmv ?? 0) > 0 ? 1 : 0.15};"
							></span>
						{/if}
						<span
							class="w-1/4 max-w-3 rounded-t-sm"
							style="height: {pct(row.money_out)}%; background-color: #f97316; opacity: {row.money_out > 0 ? 1 : 0.15};"
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
