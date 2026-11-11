<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { rp } from '$lib/credits/rupiah';

	// Grafik batang 12 bulan tanpa library: tiga seri per bulan (top up masuk,
	// GMV publikasi terbit, pencairan mitra), sumbu dengan garis bantu, dan
	// tooltip saat hover atau fokus keyboard. Warna seri datang dari token
	// --s1..--s3 yang didefinisikan halaman induk.
	type BarRow = {
		month: string; label: string;
		money_in: number; money_out: number; gmv?: number;
	};

	let { rows, onSelect }: { rows: BarRow[]; onSelect: (month: string) => void } = $props();

	const hasGMV = $derived(rows.some((r) => (r.gmv ?? 0) > 0));
	const rawMax = $derived(Math.max(1, ...rows.map((r) => Math.max(r.money_in, r.money_out, r.gmv ?? 0))));

	// Puncak sumbu dibulatkan ke 1, 2, 4, 8, atau 10 kali pangkat sepuluh supaya
	// empat selang garis bantunya angka yang rapi.
	function niceCeil(v: number) {
		const p = Math.pow(10, Math.floor(Math.log10(v)));
		const n = v / p;
		const step = n <= 1 ? 1 : n <= 2 ? 2 : n <= 4 ? 4 : n <= 8 ? 8 : 10;
		return step * p;
	}
	const niceMax = $derived(niceCeil(rawMax));
	const ticks = $derived([1, 0.75, 0.5, 0.25, 0].map((f) => niceMax * f));

	function compact(n: number) {
		const trim = (x: number) => x.toFixed(1).replace(/\.0$/, '').replace('.', ',');
		if (n >= 1e9) return `${trim(n / 1e9)} M`;
		if (n >= 1e6) return `${trim(n / 1e6)} jt`;
		if (n >= 1e3) return `${trim(n / 1e3)} rb`;
		return String(Math.round(n));
	}
	const height = (v: number) => (v <= 0 ? 0 : Math.max(1.5, (v / niceMax) * 100));
	const sum = (pick: (r: BarRow) => number) => rows.reduce((a, r) => a + pick(r), 0);

	const series = $derived([
		{ key: 'in', label: 'Top up masuk', color: 'var(--s1)', total: sum((r) => r.money_in), pick: (r: BarRow) => r.money_in },
		...(hasGMV
			? [{ key: 'gmv', label: 'GMV publikasi terbit', color: 'var(--s3)', total: sum((r) => r.gmv ?? 0), pick: (r: BarRow) => r.gmv ?? 0 }]
			: []),
		{ key: 'out', label: 'Pencairan mitra', color: 'var(--s2)', total: sum((r) => r.money_out), pick: (r: BarRow) => r.money_out }
	]);
</script>

<div class="cb">
	<div class="cbLegend">
		{#each series as s (s.key)}
			<span class="cbKey">
				<span class="cbSw" style="background-color: {s.color};"></span>
				{tr(s.label)}
				<strong>{rp(s.total)}</strong>
			</span>
		{/each}
	</div>

	<div class="cbScroll">
		<div class="cbPlot" style="--n: {rows.length};">
			<div class="cbAxis" aria-hidden="true">
				{#each ticks as t, i (i)}
					<span style="top: {i * 25}%;">{compact(t)}</span>
				{/each}
			</div>

			<div class="cbArea">
				{#each ticks as _, i (i)}
					<span class="cbLine" class:cbBase={i === ticks.length - 1} style="top: {i * 25}%;"></span>
				{/each}

				<div class="cbCols">
					{#each rows as row, i (row.month)}
						<button
							type="button"
							class="cbCol"
							aria-label={`${row.label}: ${series.map((s) => `${tr(s.label)} ${rp(s.pick(row))}`).join(', ')}`}
							onclick={() => onSelect(row.month)}
						>
							<span class="cbBars">
								{#each series as s (s.key)}
									{#if s.pick(row) > 0}
										<span class="cbBar" style="height: {height(s.pick(row))}%; background-color: {s.color};"></span>
									{/if}
								{/each}
							</span>
							<span class="cbTip" class:cbTipL={i < 2} class:cbTipR={i >= rows.length - 2}>
								<span class="cbTipTitle">{row.label}</span>
								{#each series as s (s.key)}
									<span class="cbTipRow">
										<span class="cbSw" style="background-color: {s.color};"></span>
										<span class="cbTipName">{tr(s.label)}</span>
										<span class="cbTipVal">{rp(s.pick(row))}</span>
									</span>
								{/each}
							</span>
						</button>
					{/each}
				</div>
			</div>

			<span></span>
			<div class="cbLabels">
				{#each rows as row (row.month)}
					<span>{row.label}</span>
				{/each}
			</div>
		</div>
	</div>
</div>

<style>
	.cb {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.cbLegend {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 22px;
	}
	.cbKey {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.cbKey strong {
		font-weight: 600;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.cbSw {
		flex: 0 0 auto;
		width: 10px;
		height: 10px;
		border-radius: 3px;
	}
	.cbScroll {
		overflow-x: auto;
	}
	.cbPlot {
		display: grid;
		grid-template-columns: 52px minmax(0, 1fr);
		min-width: 640px;
	}
	.cbAxis {
		position: relative;
		height: 188px;
		margin-top: 8px;
	}
	.cbAxis span {
		position: absolute;
		right: 10px;
		transform: translateY(-50%);
		font-size: 11px;
		color: var(--muted-foreground);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.cbArea {
		position: relative;
		height: 188px;
		margin-top: 8px;
	}
	.cbLine {
		position: absolute;
		left: 0;
		right: 0;
		border-top: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
	}
	.cbBase {
		border-top-color: var(--border);
	}
	.cbCols {
		position: absolute;
		inset: 0;
		display: grid;
		grid-template-columns: repeat(var(--n), minmax(0, 1fr));
		gap: 6px;
		padding: 0 4px;
	}
	.cbCol {
		appearance: none;
		position: relative;
		display: flex;
		align-items: flex-end;
		justify-content: center;
		height: 100%;
		padding: 0;
		border: 0;
		border-radius: 6px;
		background: transparent;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}
	.cbCol:hover,
	.cbCol:focus-visible {
		background: color-mix(in srgb, var(--primary) 7%, transparent);
	}
	.cbCol:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: -2px;
	}
	.cbBars {
		display: flex;
		align-items: flex-end;
		justify-content: center;
		gap: 3px;
		width: 100%;
		height: 100%;
	}
	.cbBar {
		width: clamp(4px, 26%, 14px);
		border-radius: 3px 3px 0 0;
	}
	.cbLabels {
		display: grid;
		grid-template-columns: repeat(var(--n), minmax(0, 1fr));
		gap: 6px;
		padding: 8px 4px 0;
	}
	.cbLabels span {
		font-size: 11px;
		text-align: center;
		color: var(--muted-foreground);
		white-space: nowrap;
	}

	.cbTip {
		position: absolute;
		top: 8px;
		left: 50%;
		z-index: 10;
		display: none;
		flex-direction: column;
		gap: 6px;
		min-width: 188px;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card);
		box-shadow: 0 10px 28px rgba(16, 24, 40, 0.14);
		text-align: left;
		transform: translateX(-50%);
		pointer-events: none;
	}
	.cbTipL {
		left: 0;
		transform: none;
	}
	.cbTipR {
		left: auto;
		right: 0;
		transform: none;
	}
	.cbCol:hover .cbTip,
	.cbCol:focus-visible .cbTip {
		display: flex;
	}
	.cbTipTitle {
		font-size: 12px;
		font-weight: 600;
		color: var(--foreground);
	}
	.cbTipRow {
		display: grid;
		grid-template-columns: 10px minmax(0, 1fr) auto;
		align-items: center;
		gap: 8px;
		font-size: 12px;
	}
	.cbTipName {
		color: var(--muted-foreground);
	}
	.cbTipVal {
		font-weight: 600;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	@media (prefers-reduced-motion: reduce) {
		.cbCol {
			transition: none;
		}
	}
</style>
