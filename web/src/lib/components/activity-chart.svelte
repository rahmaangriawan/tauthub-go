<script lang="ts">
	// Grafik garis harian untuk dua deret angka nyata (mis. jangkauan dan klik).
	// SVG tanpa dependensi. Satu warna aksen untuk deret utama, abu untuk
	// pembanding. Sorotan per hari lewat pointer: garis vertikal, titik, dan
	// kotak kecil berisi tanggal dan kedua angka.
	type Point = { date: string; reach: number; clicks: number };

	let {
		points,
		reachLabel,
		clicksLabel,
		locale = 'id-ID'
	}: {
		points: Point[];
		reachLabel: string;
		clicksLabel: string;
		locale?: string;
	} = $props();

	const W = 640;
	const H = 200;
	const PL = 38;
	const PR = 12;
	const PT = 12;
	const PB = 26;

	// Batas atas sumbu dibulatkan ke 1, 2, atau 5 kali pangkat sepuluh supaya
	// angka di sumbu mudah dibaca.
	function niceMax(v: number): number {
		if (v <= 1) return 1;
		const exp = Math.pow(10, Math.floor(Math.log10(v)));
		const f = v / exp;
		const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10;
		return nice * exp;
	}

	const n = $derived(points.length);
	const max = $derived(niceMax(Math.max(1, ...points.flatMap((p) => [p.reach, p.clicks]))));
	const stepX = $derived(n > 1 ? (W - PL - PR) / (n - 1) : 0);
	const xAt = (i: number) => PL + i * stepX;
	const yAt = (v: number) => PT + (H - PT - PB) * (1 - v / max);
	const baseY = H - PB;

	const ticks = $derived(max <= 2 ? [0, max] : [0, max / 2, max]);

	function linePath(pick: (p: Point) => number): string {
		return points.map((p, i) => `${i === 0 ? 'M' : 'L'}${xAt(i).toFixed(1)} ${yAt(pick(p)).toFixed(1)}`).join(' ');
	}
	const reachPath = $derived(linePath((p) => p.reach));
	const clicksPath = $derived(linePath((p) => p.clicks));
	const reachArea = $derived(
		n > 1 ? `${reachPath} L${xAt(n - 1).toFixed(1)} ${baseY} L${xAt(0).toFixed(1)} ${baseY} Z` : ''
	);

	function fmtDay(date: string, withYear = false): string {
		const d = new Date(`${date}T12:00:00`);
		return d.toLocaleDateString(locale, { day: 'numeric', month: 'short', ...(withYear ? { year: 'numeric' } : {}) });
	}
	// Label tanggal tiap tujuh hari, ditambah hari terakhir bila tidak berimpit.
	const xLabels = $derived.by(() => {
		const out: number[] = [];
		for (let i = 0; i < n; i += 7) out.push(i);
		if (n > 0 && n - 1 - out[out.length - 1] >= 4) out.push(n - 1);
		return out;
	});

	let hover = $state<number | null>(null);
	let svgEl = $state<SVGSVGElement | null>(null);
	function onMove(e: PointerEvent) {
		if (!svgEl || n === 0) return;
		const rect = svgEl.getBoundingClientRect();
		const x = ((e.clientX - rect.left) / rect.width) * W;
		const idx = stepX > 0 ? Math.round((x - PL) / stepX) : 0;
		hover = Math.min(n - 1, Math.max(0, idx));
	}

	const totalReach = $derived(points.reduce((s, p) => s + p.reach, 0));
	const totalClicks = $derived(points.reduce((s, p) => s + p.clicks, 0));
	const fmt = (v: number) => v.toLocaleString(locale);
	const summary = $derived(
		`${reachLabel}: ${fmt(totalReach)}. ${clicksLabel}: ${fmt(totalClicks)}. ${n > 0 ? `${fmtDay(points[0].date)} - ${fmtDay(points[n - 1].date)}` : ''}`
	);
</script>

<div class="actChart">
	<div class="actLegend">
		<span class="actLegendItem"><i class="actDot actDotMain"></i>{reachLabel} <b>{fmt(totalReach)}</b></span>
		<span class="actLegendItem"><i class="actDot actDotMuted"></i>{clicksLabel} <b>{fmt(totalClicks)}</b></span>
	</div>

	<div class="actPlot">
		<svg
			bind:this={svgEl}
			viewBox="0 0 {W} {H}"
			role="img"
			aria-label={summary}
			onpointermove={onMove}
			onpointerleave={() => (hover = null)}
			preserveAspectRatio="none"
		>
			{#each ticks as t (t)}
				<line x1={PL} x2={W - PR} y1={yAt(t)} y2={yAt(t)} class="actGrid" />
				<text x={PL - 6} y={yAt(t) + 3.5} class="actAxis" text-anchor="end">{fmt(Math.round(t))}</text>
			{/each}
			{#each xLabels as i (i)}
				<text x={xAt(i)} y={H - 8} class="actAxis" text-anchor={i === 0 ? 'start' : i === n - 1 ? 'end' : 'middle'}>{fmtDay(points[i].date)}</text>
			{/each}

			{#if reachArea}<path d={reachArea} class="actArea" />{/if}
			<path d={clicksPath} class="actLine actLineMuted" />
			<path d={reachPath} class="actLine actLineMain" />

			{#if hover !== null}
				<line x1={xAt(hover)} x2={xAt(hover)} y1={PT} y2={baseY} class="actCross" />
				<circle cx={xAt(hover)} cy={yAt(points[hover].reach)} r="3.5" class="actPt actPtMain" />
				<circle cx={xAt(hover)} cy={yAt(points[hover].clicks)} r="3.5" class="actPt actPtMuted" />
			{/if}
		</svg>

		{#if hover !== null}
			<div class="actTip" style="left: {(xAt(hover) / W) * 100}%;" class:actTipRight={hover > n * 0.6}>
				<strong>{fmtDay(points[hover].date, true)}</strong>
				<span><i class="actDot actDotMain"></i>{reachLabel}: {fmt(points[hover].reach)}</span>
				<span><i class="actDot actDotMuted"></i>{clicksLabel}: {fmt(points[hover].clicks)}</span>
			</div>
		{/if}
	</div>
</div>

<style>
	.actChart {
		width: 100%;
	}
	.actLegend {
		display: flex;
		flex-wrap: wrap;
		gap: 6px 20px;
		margin-bottom: 10px;
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.actLegendItem b {
		margin-left: 4px;
		font-weight: 600;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.actDot {
		display: inline-block;
		width: 8px;
		height: 8px;
		margin-right: 6px;
		border-radius: 9999px;
	}
	.actDotMain {
		background: var(--primary);
	}
	.actDotMuted {
		background: var(--muted-foreground);
	}
	.actPlot {
		position: relative;
		width: 100%;
	}
	svg {
		display: block;
		width: 100%;
		height: 200px;
		touch-action: pan-y;
		cursor: crosshair;
	}
	.actGrid {
		stroke: var(--border);
		stroke-width: 1;
		vector-effect: non-scaling-stroke;
	}
	.actAxis {
		font-size: 10.5px;
		fill: var(--muted-foreground);
		font-variant-numeric: tabular-nums;
	}
	.actArea {
		fill: color-mix(in srgb, var(--primary) 10%, transparent);
	}
	.actLine {
		fill: none;
		stroke-linejoin: round;
		stroke-linecap: round;
		vector-effect: non-scaling-stroke;
	}
	.actLineMain {
		stroke: var(--primary);
		stroke-width: 2;
	}
	.actLineMuted {
		stroke: var(--muted-foreground);
		stroke-width: 1.5;
		opacity: 0.7;
	}
	.actCross {
		stroke: var(--muted-foreground);
		stroke-width: 1;
		stroke-dasharray: 3 3;
		vector-effect: non-scaling-stroke;
	}
	.actPt {
		stroke: var(--card);
		stroke-width: 1.5;
	}
	.actPtMain {
		fill: var(--primary);
	}
	.actPtMuted {
		fill: var(--muted-foreground);
	}
	.actTip {
		position: absolute;
		top: 4px;
		z-index: 2;
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 150px;
		padding: 8px 10px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.1);
		font-size: 11.5px;
		color: var(--foreground);
		pointer-events: none;
		transform: translateX(8px);
	}
	.actTipRight {
		transform: translateX(calc(-100% - 8px));
	}
	.actTip strong {
		font-weight: 600;
	}
</style>
