<script lang="ts">
	import { rp } from '$lib/credits/rupiah';

	// Sparkline ringkas untuk kartu KPI: tanpa sumbu, grid, legenda, atau
	// marker. Nilai berasal dari seri `sparkline` API (bucket harian/bulanan
	// mengikuti rentang filter), bukan data contoh.
	type SparkPoint = { label: string; value: number };

	let {
		points = [],
		color = 'var(--primary)'
	}: { points?: SparkPoint[]; color?: string } = $props();

	const W = 100;
	const H = 36;
	const PAD = 3;

	const n = $derived(points.length);
	const values = $derived(points.map((p) => Number(p.value) || 0));
	const lo = $derived(Math.min(0, ...values));
	const hi = $derived(Math.max(0, ...values));
	const span = $derived(hi - lo || 1);
	// Semua nilai nol (mis. belum ada pencairan): garis datar tipis di tengah,
	// tanpa area fill — jujur bahwa tidak ada pergerakan.
	const flat = $derived(hi === lo);

	const xs = $derived(values.map((_, i) => (n <= 1 ? W / 2 : PAD + (i / (n - 1)) * (W - PAD * 2))));
	const ys = $derived(
		values.map((v) => (flat ? H / 2 : H - PAD - ((v - lo) / span) * (H - PAD * 2)))
	);

	// Catmull-Rom → cubic Bézier: garis mengalir halus, bukan patah-patah di
	// tiap titik data. Kontrol Y dikunci di dalam area supaya curve tidak
	// melewati tepi kartu saat lonjakannya besar.
	const clampY = (y: number) => Math.max(PAD - 1, Math.min(H - PAD + 1, y));
	const line = $derived.by(() => {
		if (n === 0) return '';
		if (n === 1) return `M${PAD} ${ys[0].toFixed(1)} L${W - PAD} ${ys[0].toFixed(1)}`;
		const pts = xs.map((x, i) => ({ x, y: ys[i] }));
		const d = [`M${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`];
		for (let i = 0; i < pts.length - 1; i++) {
			const p0 = pts[i - 1] ?? pts[i];
			const p1 = pts[i];
			const p2 = pts[i + 1];
			const p3 = pts[i + 2] ?? p2;
			const c1x = p1.x + (p2.x - p0.x) / 6;
			const c1y = clampY(p1.y + (p2.y - p0.y) / 6);
			const c2x = p2.x - (p3.x - p1.x) / 6;
			const c2y = clampY(p2.y - (p3.y - p1.y) / 6);
			d.push(
				`C${c1x.toFixed(1)} ${c1y.toFixed(1)} ${c2x.toFixed(1)} ${c2y.toFixed(1)} ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
			);
		}
		return d.join(' ');
	});
	const area = $derived(
		n === 0 || flat
			? ''
			: `${line} L${xs[n - 1].toFixed(1)} ${H} L${xs[0].toFixed(1)} ${H} Z`
	);
	const seriesKey = $derived(values.join(','));

	const gid = $props.id();
	let hover = $state<number | null>(null);

	function onMove(e: PointerEvent) {
		if (n === 0) return;
		const rect = (e.currentTarget as SVGSVGElement).getBoundingClientRect();
		if (rect.width <= 0) return;
		const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
		hover = Math.round(ratio * (n - 1));
	}

	const tipPoint = $derived(hover !== null && hover >= 0 && hover < n ? points[hover] : null);
	const tipLeft = $derived(
		hover !== null && hover < xs.length ? Math.min(86, Math.max(14, (xs[hover] / W) * 100)) : 50
	);
</script>

<div class="sparkWrap">
	{#key seriesKey}
		<svg
			class="sparkSvg"
			viewBox="0 0 {W} {H}"
			preserveAspectRatio="none"
			aria-hidden="true"
			onpointermove={onMove}
			onpointerleave={() => (hover = null)}
		>
			<defs>
				<linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
					<stop offset="0%" stop-color={color} stop-opacity="0.24" />
					<stop offset="100%" stop-color={color} stop-opacity="0" />
				</linearGradient>
			</defs>
			{#if n > 0}
				<path d={area} fill="url(#{gid})" />
				<path
					class="sparkLine"
					class:sparkLineFlat={flat}
					d={line}
					fill="none"
					pathLength="1"
					stroke={color}
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			{/if}
		</svg>
	{/key}

	{#if tipPoint}
		<div class="sparkTip" style="left: {tipLeft}%">
			<span>{tipPoint.label}</span>
			<strong>{rp(tipPoint.value)}</strong>
		</div>
	{/if}
</div>

<style>
	.sparkWrap {
		position: relative;
		flex: 0 0 auto;
		width: 100px;
		height: 36px;
	}
	.sparkSvg {
		display: block;
		width: 100%;
		height: 100%;
		overflow: visible;
		animation: sparkFade 0.25s ease both;
	}
	.sparkLine {
		stroke-dasharray: 1;
		stroke-dashoffset: 1;
		animation: sparkDraw 0.32s ease-out forwards;
	}
	.sparkLineFlat {
		opacity: 0.4;
	}
	.sparkTip {
		position: absolute;
		bottom: calc(100% + 6px);
		transform: translateX(-50%);
		display: inline-flex;
		align-items: baseline;
		gap: 6px;
		padding: 3px 7px;
		border-radius: 6px;
		background: var(--foreground);
		color: var(--card);
		font-size: 10.5px;
		line-height: 1.2;
		white-space: nowrap;
		pointer-events: none;
		z-index: 2;
	}
	.sparkTip span {
		opacity: 0.72;
	}
	.sparkTip strong {
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	@keyframes sparkFade {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	@keyframes sparkDraw {
		to {
			stroke-dashoffset: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.sparkSvg {
			animation: none;
		}
		.sparkLine {
			animation: none;
			stroke-dashoffset: 0;
		}
	}
</style>
